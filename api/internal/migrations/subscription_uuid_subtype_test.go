package migrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/libs/store"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MigrateSuite exercises the subscription uuid subtype migration against a
// real MongoDB running in a testcontainer, matching the pattern used by
// api/db/mongodb/account_test.go.
type MigrateSuite struct {
	suite.Suite
	client    *mongo.Client
	dbName    string
	container testcontainers.Container
	// storeCfg mirrors the live container so TestCodecWiredThroughNewMongoDB
	// can reconnect via libs/store.NewMongoDB and exercise the real connect()
	// path including clientOpts.SetRegistry.
	storeCfg *store.Config
}

func TestMigrateSuite(t *testing.T) {
	suite.Run(t, new(MigrateSuite))
}

func (s *MigrateSuite) SetupSuite() {
	ctx := context.Background()

	mongoImage := firstNonEmptyEnv("TEST_MONGO_IMAGE", "mongo:7.0.8")
	username := firstNonEmptyEnv("TEST_MONGO_USERNAME", "testuser")
	password := firstNonEmptyEnv("TEST_MONGO_PASSWORD", "testpass")
	authSource := firstNonEmptyEnv("DB_AUTH_SOURCE", "admin")

	req := testcontainers.ContainerRequest{
		Image: mongoImage,
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": username,
			"MONGO_INITDB_ROOT_PASSWORD": password,
		},
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		s.T().Fatalf("failed to start mongo container: %v", err)
	}
	s.container = container

	host, err := container.Host(ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(ctx, "27017/tcp")
	s.Require().NoError(err)

	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	clientOpts := options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource})

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, clientOpts)
	s.Require().NoError(err)
	s.Require().NoError(client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err())

	s.client = client
	s.dbName = firstNonEmptyEnv("DB_TEST_NAME", "dns_migrate_test")
	_ = client.Database(s.dbName).Drop(connectCtx)

	s.storeCfg = &store.Config{
		DbURI:      uri,
		Name:       s.dbName,
		Username:   username,
		Password:   password,
		AuthSource: authSource,
	}
}

func (s *MigrateSuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Database(s.dbName).Drop(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

func (s *MigrateSuite) SetupTest() {
	if s.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.Database(s.dbName).Collection(subscriptionsCollection).Drop(ctx)
}

// seedDoc inserts a subscription-shaped document with the given _id subtype.
func (s *MigrateSuite) seedDoc(ctx context.Context, subtype byte, tier string) uuid.UUID {
	s.T().Helper()
	id := uuid.New()
	doc := bson.D{
		{Key: "_id", Value: primitive.Binary{Subtype: subtype, Data: id[:]}},
		{Key: "account_id", Value: primitive.NewObjectID()},
		{Key: "active_until", Value: time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Millisecond)},
		{Key: "is_active", Value: true},
		{Key: "tier", Value: tier},
		{Key: "token_hash", Value: "hash-" + tier},
		{Key: "updated_at", Value: time.Now().UTC().Truncate(time.Millisecond)},
		{Key: "notified", Value: false},
		{Key: "limits", Value: bson.D{{Key: "max_queries_per_month", Value: int32(0)}}},
	}
	_, err := s.client.Database(s.dbName).Collection(subscriptionsCollection).InsertOne(ctx, doc)
	s.Require().NoError(err)
	return id
}

// subtypeOf reads a single document's _id subtype directly from the raw BSON.
func (s *MigrateSuite) subtypeOf(ctx context.Context, accountIDQuery primitive.ObjectID) byte {
	s.T().Helper()
	var raw bson.Raw
	err := s.client.
		Database(s.dbName).
		Collection(subscriptionsCollection).
		FindOne(ctx, bson.D{{Key: "account_id", Value: accountIDQuery}}).
		Decode(&raw)
	s.Require().NoError(err)
	subtype, _, ok := raw.Lookup("_id").BinaryOK()
	s.Require().True(ok, "_id must be binary")
	return subtype
}

func (s *MigrateSuite) TestMigrateRewritesLegacySubtypes() {
	ctx := context.Background()

	id00a := s.seedDoc(ctx, 0x00, "Tier 2")
	id00b := s.seedDoc(ctx, 0x00, "Tier 3")
	id03 := s.seedDoc(ctx, 0x03, "Tier 2")
	id04 := s.seedDoc(ctx, 0x04, "Tier 1")

	coll := s.client.Database(s.dbName).Collection(subscriptionsCollection)
	before := s.snapshotByUUID(ctx, coll)
	s.Require().Len(before, 4)

	stats, err := MigrateSubscriptionUUIDSubtype(ctx, s.client, s.dbName)
	s.Require().NoError(err)
	s.Require().Equal(0, stats.Failed)
	s.Require().Equal(4, stats.Scanned)
	s.Require().Equal(3, stats.Migrated)
	s.Require().Equal(1, stats.Skipped)

	after := s.snapshotByUUID(ctx, coll)
	s.Require().Len(after, 4)
	for _, id := range []uuid.UUID{id00a, id00b, id03, id04} {
		doc, ok := after[id]
		s.Require().True(ok, "subscription %s missing after migration", id)

		subtype, data, bok := doc.Lookup("_id").BinaryOK()
		s.Require().True(bok)
		s.Require().Equal(byte(0x04), subtype, "id=%s", id)
		s.Require().Equal(id[:], data, "id=%s", id)

		s.assertFieldsPreserved(before[id], doc)
	}

	// Idempotency: second run is a no-op.
	stats2, err := MigrateSubscriptionUUIDSubtype(ctx, s.client, s.dbName)
	s.Require().NoError(err)
	s.Require().Equal(Stats{Scanned: 4, Migrated: 0, Skipped: 4, Failed: 0}, stats2)
}

func (s *MigrateSuite) TestMigrateResumesAfterPartialInsert() {
	ctx := context.Background()
	coll := s.client.Database(s.dbName).Collection(subscriptionsCollection)

	id := uuid.New()
	accountID := primitive.NewObjectID()
	base := bson.D{
		{Key: "account_id", Value: accountID},
		{Key: "active_until", Value: time.Now().Add(24 * time.Hour).UTC().Truncate(time.Millisecond)},
		{Key: "is_active", Value: true},
		{Key: "tier", Value: "Tier 2"},
	}

	legacy := append(bson.D{{Key: "_id", Value: primitive.Binary{Subtype: 0x00, Data: id[:]}}}, base...)
	migrated := append(bson.D{{Key: "_id", Value: primitive.Binary{Subtype: 0x04, Data: id[:]}}}, base...)
	_, err := coll.InsertOne(ctx, legacy)
	s.Require().NoError(err)
	_, err = coll.InsertOne(ctx, migrated)
	s.Require().NoError(err)

	count, err := coll.CountDocuments(ctx, bson.D{})
	s.Require().NoError(err)
	s.Require().Equal(int64(2), count)

	stats, err := MigrateSubscriptionUUIDSubtype(ctx, s.client, s.dbName)
	s.Require().NoError(err)
	s.Require().Equal(0, stats.Failed)
	s.Require().Equal(1, stats.Migrated)
	s.Require().Equal(1, stats.Skipped)

	count, err = coll.CountDocuments(ctx, bson.D{})
	s.Require().NoError(err)
	s.Require().Equal(int64(1), count)

	subtype := s.subtypeOf(ctx, accountID)
	s.Require().Equal(byte(0x04), subtype)
}

// TestCodecWiredThroughNewMongoDB is an end-to-end regression guard for the
// clientOpts.SetRegistry(buildUUIDRegistry()) call in libs/store.connect().
func (s *MigrateSuite) TestCodecWiredThroughNewMongoDB() {
	ctx := context.Background()

	db, err := store.NewMongoDB(s.storeCfg)
	s.Require().NoError(err)
	defer func() { _ = db.Disconnect() }()

	type wireCheckDoc struct {
		ID uuid.UUID `bson:"_id"`
	}
	id := uuid.New()

	coll := db.GetClient().Database(s.dbName).Collection("codec_wire_check")
	_ = coll.Drop(ctx)

	_, err = coll.InsertOne(ctx, wireCheckDoc{ID: id})
	s.Require().NoError(err)

	var raw bson.Raw
	s.Require().NoError(coll.FindOne(ctx, bson.D{}).Decode(&raw))

	subtype, data, ok := raw.Lookup("_id").BinaryOK()
	s.Require().True(ok, "_id must decode as primitive.Binary")
	s.Require().Equal(id[:], data)
	s.Require().Equal(byte(0x04), subtype,
		"connect() must call clientOpts.SetRegistry(buildUUIDRegistry()); got subtype 0x%02x", subtype)
}

func (s *MigrateSuite) snapshotByUUID(ctx context.Context, coll *mongo.Collection) map[uuid.UUID]bson.Raw {
	s.T().Helper()
	cursor, err := coll.Find(ctx, bson.D{})
	s.Require().NoError(err)
	defer cursor.Close(ctx)

	out := make(map[uuid.UUID]bson.Raw)
	for cursor.Next(ctx) {
		var raw bson.Raw
		s.Require().NoError(cursor.Decode(&raw))
		_, data, ok := raw.Lookup("_id").BinaryOK()
		s.Require().True(ok)
		var id uuid.UUID
		copy(id[:], data)
		cp := make(bson.Raw, len(raw))
		copy(cp, raw)
		out[id] = cp
	}
	s.Require().NoError(cursor.Err())
	return out
}

func (s *MigrateSuite) assertFieldsPreserved(before, after bson.Raw) {
	s.T().Helper()
	preservedKeys := []string{"account_id", "active_until", "is_active", "tier", "token_hash", "updated_at", "notified", "limits"}
	for _, key := range preservedKeys {
		beforeVal := before.Lookup(key)
		afterVal := after.Lookup(key)
		s.Require().Equal(beforeVal.Type, afterVal.Type, "type mismatch for field %q", key)
		s.Require().Equal(beforeVal.Value, afterVal.Value, "value mismatch for field %q", key)
	}
}

func firstNonEmptyEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
