package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const migration027 = "migrations/027_statistics_retention_collections"

// StatisticsRepositorySuite runs against a real MongoDB with the collections
// created by migration 027, so the time-series delete and $unionWith paths are
// exercised exactly as in production.
type StatisticsRepositorySuite struct {
	suite.Suite
	client    *mongo.Client
	repo      StatisticsRepository
	dbName    string
	container testcontainers.Container
}

func (s *StatisticsRepositorySuite) SetupSuite() {
	ctx := context.Background()

	mongoImage := firstNonEmpty(os.Getenv("TEST_MONGO_IMAGE"), "mongo:7.0.8")
	username := firstNonEmpty(os.Getenv("TEST_MONGO_USERNAME"), "testuser")
	password := firstNonEmpty(os.Getenv("TEST_MONGO_PASSWORD"), "testpass")
	authSource := firstNonEmpty(os.Getenv("DB_AUTH_SOURCE"), "admin")

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
	s.Require().NoError(err)
	s.container = container

	host, err := container.Host(ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(ctx, "27017/tcp")
	s.Require().NoError(err)

	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource}))
	s.Require().NoError(err)
	s.Require().NoError(client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err())

	s.client = client
	s.dbName = firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_test") + "_statistics"
}

func (s *StatisticsRepositorySuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Database(s.dbName).Drop(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

func (s *StatisticsRepositorySuite) SetupTest() {
	ctx := context.Background()
	s.Require().NoError(s.client.Database(s.dbName).Drop(ctx))
	s.migrate(ctx, "up")
	s.repo = NewStatisticsRepository(s.client, s.dbName)
}

// migrate runs a migration file the way golang-migrate's MongoDB driver does:
// each element of the JSON array is one runCommand.
func (s *StatisticsRepositorySuite) migrate(ctx context.Context, direction string) {
	raw, err := os.ReadFile(filepath.FromSlash(migration027 + "." + direction + ".json"))
	s.Require().NoError(err)
	var cmds []json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &cmds))
	for _, c := range cmds {
		var cmd bson.D
		s.Require().NoError(bson.UnmarshalExtJSON(c, false, &cmd))
		s.Require().NoError(s.client.Database(s.dbName).RunCommand(ctx, cmd).Err(), string(c))
	}
}

func (s *StatisticsRepositorySuite) insert(coll, profileID string, bucketStart time.Time, total int) {
	_, err := s.client.Database(s.dbName).Collection(coll).InsertOne(context.Background(), bson.D{
		{Key: "bucket_start", Value: bucketStart},
		{Key: "meta", Value: bson.D{{Key: "profile_id", Value: profileID}, {Key: "device_id", Value: "d1"}}},
		{Key: "queries", Value: bson.D{{Key: "total", Value: total}, {Key: "blocked", Value: 0}, {Key: "dnssec", Value: 0}}},
	})
	s.Require().NoError(err)
}

func (s *StatisticsRepositorySuite) count(coll, profileID string) int64 {
	n, err := s.client.Database(s.dbName).Collection(coll).CountDocuments(context.Background(), bson.D{{Key: "meta.profile_id", Value: profileID}})
	s.Require().NoError(err)
	return n
}

// specRef: api-endpoint-behaviour.md J11
func (s *StatisticsRepositorySuite) TestMigration027_CollectionOptions() {
	ctx := context.Background()
	want := map[string]int64{"statistics_30d": 2592000, "statistics_90d": 7776000, "statistics_1y": 31536000}

	specs, err := s.client.Database(s.dbName).ListCollectionSpecifications(ctx, bson.D{})
	s.Require().NoError(err)
	got := map[string]*mongo.CollectionSpecification{}
	for i := range specs {
		got[specs[i].Name] = specs[i]
	}

	for name, expire := range want {
		spec, ok := got[name]
		s.Require().True(ok, "collection %s missing", name)
		s.Equal("timeseries", spec.Type)

		var opts struct {
			Expire     int64 `bson:"expireAfterSeconds"`
			Timeseries struct {
				TimeField   string `bson:"timeField"`
				MetaField   string `bson:"metaField"`
				Granularity string `bson:"granularity"`
			} `bson:"timeseries"`
		}
		s.Require().NoError(bson.Unmarshal(spec.Options, &opts))
		s.Equal(expire, opts.Expire, name)
		s.Equal("bucket_start", opts.Timeseries.TimeField)
		s.Equal("meta", opts.Timeseries.MetaField)
		s.Equal("minutes", opts.Timeseries.Granularity)
	}
	_, legacy := got["statistics"]
	s.False(legacy, "legacy statistics collection must be gone")
}

// specRef: api-endpoint-behaviour.md J11 — the legacy collection is dropped when present, and down removes the new ones.
func (s *StatisticsRepositorySuite) TestMigration027_DropsLegacyAndRollsBack() {
	ctx := context.Background()
	db := s.client.Database(s.dbName)
	s.Require().NoError(db.Drop(ctx))
	s.Require().NoError(db.CreateCollection(ctx, "statistics", options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().SetTimeField("timestamp").SetMetaField("profile_id"))))

	s.migrate(ctx, "up")
	names, err := db.ListCollectionNames(ctx, bson.D{})
	s.Require().NoError(err)
	s.NotContains(names, "statistics")
	s.Contains(names, "statistics_30d")

	s.migrate(ctx, "down")
	names, err = db.ListCollectionNames(ctx, bson.D{})
	s.Require().NoError(err)
	s.NotContains(names, "statistics_30d")
	s.NotContains(names, "statistics_90d")
	s.NotContains(names, "statistics_1y")
}

// specRef: api-endpoint-behaviour.md J4
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_SumsAcrossCollectionsWithinTimespan() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insert("statistics_30d", "p1", now.Add(-time.Hour), 10)
	s.insert("statistics_90d", "p1", now.Add(-2*time.Hour), 20)
	s.insert("statistics_1y", "p1", now.Add(-3*time.Hour), 30)
	s.insert("statistics_1y", "p1", now.Add(-72*time.Hour), 1000)
	s.insert("statistics_30d", "p2", now.Add(-time.Hour), 7)

	got, err := s.repo.GetProfileStatistics(ctx, "p1", 24)
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.EqualValues(60, got[0].Total)

	all, err := s.repo.GetProfileStatistics(ctx, "p1", 0)
	s.Require().NoError(err)
	s.EqualValues(1060, all[0].Total)

	none, err := s.repo.GetProfileStatistics(ctx, "nobody", 24)
	s.Require().NoError(err)
	s.Require().Len(none, 1)
	s.EqualValues(0, none[0].Total)
}

// specRef: api-endpoint-behaviour.md J6
func (s *StatisticsRepositorySuite) TestDeleteProfileStatistics_RemovesOnlyThatProfileFromAllCollections() {
	now := time.Now().UTC()
	for _, c := range statisticsCollectionNames {
		s.insert(c, "p1", now.Add(-time.Hour), 1)
		s.insert(c, "p2", now.Add(-time.Hour), 1)
	}

	s.Require().NoError(s.repo.DeleteProfileStatistics(context.Background(), "p1", nil))

	for _, c := range statisticsCollectionNames {
		s.EqualValues(0, s.count(c, "p1"), c)
		s.EqualValues(1, s.count(c, "p2"), c)
	}
}

// specRef: api-endpoint-behaviour.md J8
func (s *StatisticsRepositorySuite) TestDeleteProfileStatistics_CutoffKeepsLaterBuckets() {
	base := time.Now().UTC().Truncate(15 * time.Minute)
	before := base.Add(15 * time.Minute)
	for _, c := range statisticsCollectionNames {
		s.insert(c, "p1", base.Add(-15*time.Minute), 1)
		s.insert(c, "p1", base, 1)
		s.insert(c, "p1", before, 1)
		s.insert(c, "p1", before.Add(15*time.Minute), 1)
	}

	s.Require().NoError(s.repo.DeleteProfileStatistics(context.Background(), "p1", &before))

	for _, c := range statisticsCollectionNames {
		s.EqualValues(2, s.count(c, "p1"), "%s keeps buckets starting at or after the cutoff", c)
	}
}

const profileIndexName = "meta_profile_id_bucket_start"

// explainPlan returns the JSON of an explain command for the given command document.
func (s *StatisticsRepositorySuite) explainPlan(cmd bson.D) string {
	var out bson.M
	s.Require().NoError(s.client.Database(s.dbName).RunCommand(context.Background(),
		bson.D{{Key: "explain", Value: cmd}, {Key: "verbosity", Value: "queryPlanner"}}).Decode(&out))
	b, err := bson.MarshalExtJSON(out, false, false)
	s.Require().NoError(err)
	return string(b)
}

// specRef: api-endpoint-behaviour.md J11 — every retention collection carries the profile+time index.
func (s *StatisticsRepositorySuite) TestMigration027_ProfileIndexExists() {
	for _, c := range statisticsCollectionNames {
		names, err := s.client.Database(s.dbName).Collection(c).Indexes().ListSpecifications(context.Background())
		s.Require().NoError(err)
		found := false
		for _, n := range names {
			found = found || n.Name == profileIndexName
		}
		s.True(found, "%s lacks %s", c, profileIndexName)
	}
}

// specRef: api-endpoint-behaviour.md J4, J6, J8 — the read and both purge shapes are served by the profile index.
func (s *StatisticsRepositorySuite) TestProfileIndexServesReadAndPurge() {
	now := time.Now().UTC()
	for _, c := range statisticsCollectionNames {
		for i := 0; i < 50; i++ {
			s.insert(c, fmt.Sprintf("p%d", i), now.Add(-time.Hour), 1)
		}
	}
	coll := statisticsCollectionNames[0]
	match := bson.D{{Key: "meta.profile_id", Value: "p1"}}
	bounded := bson.D{{Key: "meta.profile_id", Value: "p1"}, {Key: "bucket_start", Value: bson.D{{Key: "$lt", Value: now}}}}

	plans := map[string]string{
		"read": s.explainPlan(bson.D{{Key: "aggregate", Value: coll}, {Key: "cursor", Value: bson.D{}},
			{Key: "pipeline", Value: bson.A{bson.D{{Key: "$match", Value: bounded}}}}}),
		"purge unbounded": s.explainPlan(bson.D{{Key: "delete", Value: coll}, {Key: "deletes", Value: bson.A{bson.D{{Key: "q", Value: match}, {Key: "limit", Value: 0}}}}}),
		"purge bounded":   s.explainPlan(bson.D{{Key: "delete", Value: coll}, {Key: "deletes", Value: bson.A{bson.D{{Key: "q", Value: bounded}, {Key: "limit", Value: 0}}}}}),
	}
	for name, plan := range plans {
		s.Contains(plan, profileIndexName, "%s does not use the profile index: %s", name, plan)
		s.NotContains(plan, "COLLSCAN", name)
	}
}

func TestStatisticsRepositorySuite(t *testing.T) {
	suite.Run(t, new(StatisticsRepositorySuite))
}
