package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

	mongoImage := firstNonEmpty(os.Getenv("TEST_MONGO_IMAGE"), "mongo:8.0.9")
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
	want := map[string]int64{
		"statistics_15min": 86400, "statistics_1h": 691200,
		"statistics_1d_30d": 2592000, "statistics_1d_90d": 7776000, "statistics_1d_1y": 31536000,
	}
	s.Len(statisticsCollectionNames, len(want), "the repository must span exactly the migrated collections")

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
				TimeField             string `bson:"timeField"`
				MetaField             string `bson:"metaField"`
				Granularity           string `bson:"granularity"`
				BucketMaxSpanSeconds  int32  `bson:"bucketMaxSpanSeconds"`
				BucketRoundingSeconds int32  `bson:"bucketRoundingSeconds"`
			} `bson:"timeseries"`
		}
		s.Require().NoError(bson.Unmarshal(spec.Options, &opts))
		s.Equal(expire, opts.Expire, name)
		s.Equal("bucket_start", opts.Timeseries.TimeField)
		s.Equal("meta", opts.Timeseries.MetaField)
		// One-day buckets bound the TTL overrun to about a day.
		s.EqualValues(86400, opts.Timeseries.BucketMaxSpanSeconds, name)
		s.EqualValues(86400, opts.Timeseries.BucketRoundingSeconds, name)
		s.Empty(opts.Timeseries.Granularity, name)
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
	for _, c := range []string{"statistics_15min", "statistics_1h", "statistics_1d_30d", "statistics_1d_90d", "statistics_1d_1y"} {
		s.Contains(names, c)
	}

	s.migrate(ctx, "down")
	names, err = db.ListCollectionNames(ctx, bson.D{})
	s.Require().NoError(err)
	for _, c := range []string{"statistics_15min", "statistics_1h", "statistics_1d_30d", "statistics_1d_90d", "statistics_1d_1y"} {
		s.NotContains(names, c)
	}
}

// specRef: api-endpoint-behaviour.md J6
func (s *StatisticsRepositorySuite) TestDeleteProfileStatistics_RemovesOnlyThatProfileFromAllCollections() {
	now := time.Now().UTC()
	for _, c := range statisticsCollectionNames {
		s.insert(c, "p1", now.Add(-time.Hour), 1)
		s.insert(c, "p2", now.Add(-time.Hour), 1)
	}

	deleted, err := s.repo.DeleteProfileStatistics(context.Background(), "p1", nil)
	s.Require().NoError(err)
	s.EqualValues(len(statisticsCollectionNames), deleted, "deleted documents are counted across collections")

	for _, c := range statisticsCollectionNames {
		s.EqualValues(0, s.count(c, "p1"), c)
		s.EqualValues(1, s.count(c, "p2"), c)
	}
}

func (s *StatisticsRepositorySuite) starts(coll, profileID string) []time.Time {
	cur, err := s.client.Database(s.dbName).Collection(coll).Find(context.Background(),
		bson.D{{Key: "meta.profile_id", Value: profileID}}, options.Find().SetSort(bson.D{{Key: "bucket_start", Value: 1}}))
	s.Require().NoError(err)
	var docs []struct {
		At time.Time `bson:"bucket_start"`
	}
	s.Require().NoError(cur.All(context.Background(), &docs))
	out := make([]time.Time, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.At.UTC())
	}
	return out
}

// specRef: api-endpoint-behaviour.md J8 — a bounded delete floors the instant to each tier's own bucket width.
func (s *StatisticsRepositorySuite) TestDeleteProfileStatistics_BoundIsFlooredPerTier() {
	d := func(day, h, m int) time.Time { return time.Date(2026, 9, day, h, m, 0, 0, time.UTC) }
	enabledAt := d(29, 14, 37) // mid-day, mid-hour, inside the 14:30 quarter

	for _, at := range []time.Time{d(29, 14, 15), d(29, 14, 30), d(29, 14, 45)} {
		s.insert("statistics_15min", "p1", at, 1)
	}
	for _, at := range []time.Time{d(29, 13, 0), d(29, 14, 0), d(29, 15, 0)} {
		s.insert("statistics_1h", "p1", at, 1)
	}
	for _, c := range []string{"statistics_1d_30d", "statistics_1d_90d", "statistics_1d_1y"} {
		for _, at := range []time.Time{d(28, 0, 0), d(29, 0, 0), d(30, 0, 0)} {
			s.insert(c, "p1", at, 1)
		}
	}
	s.insert("statistics_15min", "p2", d(29, 14, 15), 1)
	s.insert("statistics_1d_1y", "p2", d(28, 0, 0), 1)

	deleted, err := s.repo.DeleteProfileStatistics(context.Background(), "p1", &enabledAt)
	s.Require().NoError(err)
	s.EqualValues(5, deleted)

	s.Equal([]time.Time{d(29, 14, 30), d(29, 14, 45)}, s.starts("statistics_15min", "p1"), "15min keeps the bucket containing enabled_at and later")
	s.Equal([]time.Time{d(29, 14, 0), d(29, 15, 0)}, s.starts("statistics_1h", "p1"), "1h keeps the hour containing enabled_at and later")
	for _, c := range []string{"statistics_1d_30d", "statistics_1d_90d", "statistics_1d_1y"} {
		s.Equal([]time.Time{d(29, 0, 0), d(30, 0, 0)}, s.starts(c, "p1"), "%s keeps the day containing enabled_at and later", c)
	}
	s.EqualValues(1, s.count("statistics_15min", "p2"), "other profiles are untouched")
	s.EqualValues(1, s.count("statistics_1d_1y", "p2"))
}

// specRef: api-endpoint-behaviour.md J8 — an instant exactly on a bucket start keeps that bucket in every tier.
func (s *StatisticsRepositorySuite) TestDeleteProfileStatistics_BoundOnBoundaryKeepsThatBucket() {
	d := func(day, h, m int) time.Time { return time.Date(2026, 9, day, h, m, 0, 0, time.UTC) }
	boundary := d(29, 0, 0)
	s.insert("statistics_15min", "p1", d(28, 23, 45), 1)
	s.insert("statistics_15min", "p1", d(29, 0, 0), 1)
	s.insert("statistics_1h", "p1", d(28, 23, 0), 1)
	s.insert("statistics_1h", "p1", d(29, 0, 0), 1)
	s.insert("statistics_1d_30d", "p1", d(28, 0, 0), 1)
	s.insert("statistics_1d_30d", "p1", d(29, 0, 0), 1)

	deleted, err := s.repo.DeleteProfileStatistics(context.Background(), "p1", &boundary)
	s.Require().NoError(err)
	s.EqualValues(3, deleted)

	s.Equal([]time.Time{d(29, 0, 0)}, s.starts("statistics_15min", "p1"))
	s.Equal([]time.Time{d(29, 0, 0)}, s.starts("statistics_1h", "p1"))
	s.Equal([]time.Time{d(29, 0, 0)}, s.starts("statistics_1d_30d", "p1"))
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

// specRef: api-endpoint-behaviour.md J9
func (s *StatisticsRepositorySuite) TestListStatisticsProfileIDs_DistinctAcrossCollections() {
	now := time.Now().UTC()
	s.insert("statistics_15min", "p1", now.Add(-time.Hour), 1)
	s.insert("statistics_15min", "p1", now.Add(-2*time.Hour), 1)
	s.insert("statistics_1h", "p1", now.Add(-time.Hour), 1)
	s.insert("statistics_1h", "p2", now.Add(-time.Hour), 1)
	s.insert("statistics_1d_1y", "p3", now.Add(-time.Hour), 1)

	ids, err := s.repo.ListStatisticsProfileIDs(context.Background())
	s.Require().NoError(err)
	s.ElementsMatch([]string{"p1", "p2", "p3"}, ids)

	empty := NewStatisticsRepository(s.client, s.dbName+"_empty")
	none, err := empty.ListStatisticsProfileIDs(context.Background())
	s.Require().NoError(err)
	s.Empty(none)
}

// specRef: api-endpoint-behaviour.md J9 — listing profile ids is served by the profile index, not a collection scan.
func (s *StatisticsRepositorySuite) TestListStatisticsProfileIDs_UsesProfileIndex() {
	now := time.Now().UTC()
	for _, c := range statisticsCollectionNames {
		for i := 0; i < 50; i++ {
			s.insert(c, fmt.Sprintf("p%d", i), now.Add(-time.Hour), 1)
		}
	}
	plan := s.explainPlan(bson.D{{Key: "aggregate", Value: statisticsCollectionNames[0]}, {Key: "cursor", Value: bson.D{}},
		{Key: "pipeline", Value: bson.A{bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$meta.profile_id"}}}}}}})
	s.Contains(plan, profileIndexName, plan)
	s.NotContains(plan, "COLLSCAN")
	s.True(strings.Contains(plan, "DISTINCT_SCAN") || strings.Contains(plan, "IXSCAN"), plan)
	s.T().Logf("distinct scan: %v, index scan: %v", strings.Contains(plan, "DISTINCT_SCAN"), strings.Contains(plan, "IXSCAN"))
}

// specRef: api-endpoint-behaviour.md J9 — one query returns only settings.statistics; missing profiles are absent, a missing block maps to nil.
func (s *StatisticsRepositorySuite) TestGetProfilesStatisticsSettings() {
	ctx := context.Background()
	coll := s.client.Database(s.dbName).Collection("profiles_purge")
	enabledAt := time.Date(2026, 9, 29, 10, 7, 0, 0, time.UTC)
	_, err := coll.InsertMany(ctx, []any{
		bson.D{{Key: "profile_id", Value: "on"}, {Key: "account_id", Value: "secret"}, {Key: "settings", Value: bson.D{
			{Key: "statistics", Value: bson.D{{Key: "enabled", Value: true}, {Key: "enabled_at", Value: enabledAt}}},
			{Key: "logs", Value: bson.D{{Key: "enabled", Value: true}}},
		}}},
		bson.D{{Key: "profile_id", Value: "off"}, {Key: "settings", Value: bson.D{{Key: "statistics", Value: bson.D{{Key: "enabled", Value: false}}}}}},
		bson.D{{Key: "profile_id", Value: "no-block"}, {Key: "settings", Value: bson.D{{Key: "logs", Value: bson.D{{Key: "enabled", Value: true}}}}}},
		bson.D{{Key: "profile_id", Value: "null-block"}, {Key: "settings", Value: bson.D{{Key: "statistics", Value: nil}}}},
		bson.D{{Key: "profile_id", Value: "other"}},
	})
	s.Require().NoError(err)

	repo := NewProfileRepository(s.client, s.dbName, "profiles_purge")
	got, err := repo.GetProfilesStatisticsSettings(ctx, []string{"on", "off", "no-block", "null-block", "missing"})
	s.Require().NoError(err)

	s.Len(got, 4, "only requested, existing profiles are returned")
	s.Require().NotNil(got["on"])
	s.True(got["on"].Enabled)
	s.Require().NotNil(got["on"].EnabledAt)
	s.True(got["on"].EnabledAt.Equal(enabledAt))
	s.Require().NotNil(got["off"])
	s.False(got["off"].Enabled)
	s.Contains(got, "no-block")
	s.Nil(got["no-block"])
	s.Contains(got, "null-block")
	s.Nil(got["null-block"])
	s.NotContains(got, "missing")
	s.NotContains(got, "other")

	empty, err := repo.GetProfilesStatisticsSettings(ctx, nil)
	s.Require().NoError(err)
	s.Empty(empty)
}

func TestStatisticsRepositorySuite(t *testing.T) {
	suite.Run(t, new(StatisticsRepositorySuite))
}
