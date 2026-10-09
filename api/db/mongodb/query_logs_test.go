package mongodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueryLogsRepositorySuite creates the retention collections with the proxy's exact
// time-series options (proxy/emitter/mongodb/query_logs.go).
type QueryLogsRepositorySuite struct {
	suite.Suite
	client    *mongo.Client
	container testcontainers.Container
	repo      QueryLogsRepository
	db        *mongo.Database
}

func (s *QueryLogsRepositorySuite) SetupSuite() {
	s.client, s.container = startTestMongo(s.T())
	s.db = s.client.Database("dns_test_query_logs")
	s.repo = NewQueryLogsRepository(s.client, s.db.Name(), collNameQueryLogs)
}

func (s *QueryLogsRepositorySuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Disconnect(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

func (s *QueryLogsRepositorySuite) SetupTest() {
	ctx := context.Background()
	s.Require().NoError(s.db.Drop(ctx))
	for _, name := range queryLogsCollectionNames() {
		opts := options.CreateCollection().SetTimeSeriesOptions(
			options.TimeSeries().SetTimeField("timestamp").SetMetaField("profile_id").SetGranularity("seconds"),
		).SetExpireAfterSeconds(2592000)
		s.Require().NoError(s.db.CreateCollection(ctx, name, opts))
	}
}

func (s *QueryLogsRepositorySuite) insert(coll, profileId string, n int) {
	docs := make([]any, 0, n)
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		docs = append(docs, bson.D{
			{Key: "timestamp", Value: now.Add(-time.Duration(i) * time.Second)},
			{Key: "profile_id", Value: profileId},
			{Key: "status", Value: "processed"},
			{Key: "dns_request", Value: bson.D{{Key: "domain", Value: "example.com"}}},
		})
	}
	_, err := s.db.Collection(coll).InsertMany(context.Background(), docs)
	s.Require().NoError(err)
}

func (s *QueryLogsRepositorySuite) count(coll, profileId string) int64 {
	n, err := s.db.Collection(coll).CountDocuments(context.Background(), bson.D{{Key: "profile_id", Value: profileId}})
	s.Require().NoError(err)
	return n
}

// specRef: api-endpoint-behaviour.md J14 — profile ids are listed once across all retention collections.
func (s *QueryLogsRepositorySuite) TestListQueryLogProfileIDs_DistinctAcrossCollections() {
	s.insert(queryLogsCollOneHour, "p1", 3)
	s.insert(queryLogsCollOneDay, "p1", 2)
	s.insert(queryLogsCollOneDay, "p2", 1)
	s.insert(queryLogsCollOneMonth, "p3", 1)

	ids, err := s.repo.ListQueryLogProfileIDs(context.Background())
	s.Require().NoError(err)
	s.ElementsMatch([]string{"p1", "p2", "p3"}, ids)

	s.Require().NoError(s.db.Drop(context.Background()))
	ids, err = s.repo.ListQueryLogProfileIDs(context.Background())
	s.Require().NoError(err, "missing collections list nothing")
	s.Empty(ids)
}

// specRef: api-endpoint-behaviour.md J14 — the listing is a DISTINCT_SCAN of the automatic
// metaField+time index: no bucket or measurement is read.
func (s *QueryLogsRepositorySuite) TestListQueryLogProfileIDs_DoesNotUnpackMeasurements() {
	for i := 0; i < 20; i++ {
		s.insert(queryLogsCollOneHour, fmt.Sprintf("p%d", i), 50)
	}
	var out bson.M
	s.Require().NoError(s.db.RunCommand(context.Background(), bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "aggregate", Value: queryLogsCollOneHour},
			{Key: "cursor", Value: bson.D{}},
			{Key: "pipeline", Value: bson.A{bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$profile_id"}}}}}},
		}},
		{Key: "verbosity", Value: "executionStats"},
	}).Decode(&out))
	b, err := bson.MarshalExtJSON(out, false, false)
	s.Require().NoError(err)
	plan := string(b)
	s.Contains(plan, `"stage":"DISTINCT_SCAN"`, plan)
	s.Contains(plan, `"indexName":"profile_id_1_timestamp_1"`, "the automatic meta+time index serves the listing")
	s.Contains(plan, `"totalDocsExamined":0`, plan)
	s.NotContains(plan, "COLLSCAN")
}

// specRef: api-endpoint-behaviour.md J25 — the top-blocklists read is bounded by the automatic
// metaField+time index to the profile and window; no COLLSCAN.
func (s *QueryLogsRepositorySuite) TestGetQueryLogTopBlocklists_UsesMetaTimeIndex() {
	for i := 0; i < 20; i++ {
		s.insert(queryLogsCollOneDay, fmt.Sprintf("p%d", i), 50)
	}
	var out bson.M
	s.Require().NoError(s.db.RunCommand(context.Background(), bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "aggregate", Value: queryLogsCollOneDay},
			{Key: "cursor", Value: bson.D{}},
			{Key: "pipeline", Value: topBlocklistsPipeline("p3", time.Now().Add(-6*time.Hour), 50)},
		}},
		{Key: "verbosity", Value: "executionStats"},
	}).Decode(&out))
	b, err := bson.MarshalExtJSON(out, false, false)
	s.Require().NoError(err)
	plan := string(b)
	s.Contains(plan, `"indexName":"profile_id_1_timestamp_1"`, plan)
	s.NotContains(plan, "COLLSCAN", plan)

	got, err := s.repo.GetQueryLogTopBlocklists(context.Background(), "p3", "1d", 6, 50)
	s.Require().NoError(err)
	s.Empty(got, "processed rows without reasons count for nothing")
}

// specRef: api-endpoint-behaviour.md J12, J13 — deleting a profile's logs empties every retention collection.
func (s *QueryLogsRepositorySuite) TestDeleteQueryLogs_AllCollections() {
	for _, c := range queryLogsCollectionNames() {
		s.insert(c, "p1", 2)
		s.insert(c, "keep", 1)
	}
	s.Require().NoError(s.repo.DeleteQueryLogs(context.Background(), "p1"))
	for _, c := range queryLogsCollectionNames() {
		s.Zero(s.count(c, "p1"), c)
		s.EqualValues(1, s.count(c, "keep"), c)
	}
}

func TestQueryLogsRepositorySuite(t *testing.T) {
	suite.Run(t, new(QueryLogsRepositorySuite))
}
