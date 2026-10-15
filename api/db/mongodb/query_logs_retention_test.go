package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// logRow is one query-log row as the proxy writes it.
type logRow struct {
	at       time.Time
	status   string
	domain   string
	clientIP string
	deviceID string
	reasons  []string
}

func (s *QueryLogsRepositorySuite) insertRows(coll, profileId string, rows ...logRow) []primitive.ObjectID {
	docs := make([]any, 0, len(rows))
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, r := range rows {
		id := primitive.NewObjectID()
		ids = append(ids, id)
		status := r.status
		if status == "" {
			status = model.QueryLogStatusProcessed
		}
		docs = append(docs, bson.D{
			{Key: "_id", Value: id},
			{Key: "timestamp", Value: r.at},
			{Key: "profile_id", Value: profileId},
			{Key: "device_id", Value: r.deviceID},
			{Key: "status", Value: status},
			{Key: "reasons", Value: r.reasons},
			{Key: "dns_request", Value: bson.D{{Key: "domain", Value: r.domain}}},
			{Key: "client_ip", Value: r.clientIP},
		})
	}
	_, err := s.db.Collection(coll).InsertMany(context.Background(), docs)
	s.Require().NoError(err)
	return ids
}

func (s *QueryLogsRepositorySuite) ids(coll, profileId string) []primitive.ObjectID {
	cur, err := s.db.Collection(coll).Find(context.Background(), bson.D{{Key: "profile_id", Value: profileId}})
	s.Require().NoError(err)
	var rows []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	s.Require().NoError(cur.All(context.Background(), &rows))
	out := make([]primitive.ObjectID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// specRef: api-endpoint-behaviour.md J1, J56 — the list reads every collection up to the current retention,
// newest first across them, and pages over the merged order; longer collections are not read.
func (s *QueryLogsRepositorySuite) TestGetQueryLogs_UnionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-1 * time.Minute), domain: "a"}, logRow{at: now.Add(-3 * time.Minute), domain: "c"})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Minute), domain: "b"}, logRow{at: now.Add(-48 * time.Hour), domain: "d"})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-30 * time.Second), domain: "longer"})
	s.insertRows(queryLogsCollOneDay, "other", logRow{at: now.Add(-time.Minute), domain: "other"})

	domains := func(logs []model.QueryLog) []string {
		out := make([]string, 0, len(logs))
		for _, l := range logs {
			out = append(out, l.DNSRequest.Domain)
		}
		return out
	}

	got, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 0, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"a", "b", "c", "d"}, domains(got))

	page2, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 0, "", "", "", 2, 2)
	s.Require().NoError(err)
	s.Equal([]string{"c", "d"}, domains(page2), "pages follow the merged order")

	windowed, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 1, "", "", "domain", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"a", "b", "c"}, domains(windowed), "the timespan and sort apply across collections")

	hour, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneHour, model.QueryLogStatusAll, 0, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"a", "c"}, domains(hour), "the shortest retention reads only its own collection")
}

// specRef: api-endpoint-behaviour.md J5, J56 — devices merge across the collections read, keeping the latest last_seen.
func (s *QueryLogsRepositorySuite) TestGetQueryLogDevices_UnionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-time.Minute), deviceID: "laptop"})
	s.insertRows(queryLogsCollOneDay, "p1", logRow{at: now.Add(-2 * time.Hour), deviceID: "laptop"}, logRow{at: now.Add(-3 * time.Hour), deviceID: "phone"})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-time.Minute), deviceID: "longer"})

	got, err := s.repo.GetQueryLogDevices(ctx, "p1", model.RetentionOneDay)
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal("laptop", got[0].DeviceId)
	s.True(now.Add(-time.Minute).Equal(got[0].LastSeen.UTC()), "latest across collections: %v", got[0].LastSeen)
	s.Equal("phone", got[1].DeviceId)
}

// specRef: api-endpoint-behaviour.md J20, J21, J24, J56 — top lists sum counts across the collections read.
func (s *QueryLogsRepositorySuite) TestTopLists_UnionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	blocked := func(domain, ip string, reasons ...string) logRow {
		return logRow{at: now.Add(-time.Minute), status: model.QueryLogStatusBlocked, domain: domain, clientIP: ip, reasons: reasons}
	}
	s.insertRows(queryLogsCollOneHour, "p1", blocked("x.com", "10.0.0.1", "blocklist: a"), blocked("y.com", "10.0.0.2", "blocklist: b"))
	s.insertRows(queryLogsCollSixHours, "p1", blocked("x.com", "10.0.0.2", "blocklist: b"), blocked("x.com", "10.0.0.2", "blocklist: b"))
	s.insertRows(queryLogsCollOneDay, "p1", blocked("longer.com", "10.0.0.9", "blocklist: z"))

	domains, err := s.repo.GetQueryLogTopDomains(ctx, "p1", model.RetentionSixHours, model.QueryLogStatusBlocked, 6, 10)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "x.com", Count: 3}, {Domain: "y.com", Count: 1}}, domains)

	limited, err := s.repo.GetQueryLogTopDomains(ctx, "p1", model.RetentionSixHours, model.QueryLogStatusBlocked, 6, 1)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "x.com", Count: 3}}, limited, "the limit applies after merging")

	clients, err := s.repo.GetQueryLogTopClients(ctx, "p1", model.RetentionSixHours, 6, 10)
	s.Require().NoError(err)
	s.Require().Len(clients, 2)
	s.Equal("10.0.0.2", clients[0].IP)
	s.EqualValues(3, clients[0].Count)
	s.Equal("10.0.0.1", clients[1].IP)

	lists, err := s.repo.GetQueryLogTopBlocklists(ctx, "p1", model.RetentionSixHours, 6, 10)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopBlocklist{{BlocklistID: "b", Count: 3}, {BlocklistID: "a", Count: 1}}, lists)
}

// specRef: api-endpoint-behaviour.md J20, J56 — covered_from is the oldest row in the window across the collections read.
func (s *QueryLogsRepositorySuite) TestGetQueryLogCoveredFrom_UnionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Hour)})
	s.insertRows(queryLogsCollOneDay, "p1", logRow{at: now.Add(-20 * time.Hour)})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-23 * time.Hour)})

	got, err := s.repo.GetQueryLogCoveredFrom(ctx, "p1", model.RetentionOneWeek, 24)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.True(now.Add(-20*time.Hour).Truncate(time.Minute).Equal(*got), "the shorter collection holds the oldest row: %v", *got)
}

// specRef: api-endpoint-behaviour.md J54 — lowering keeps the rows the new retention still covers, unchanged,
// in the new retention's collection, and deletes everything older; other profiles are untouched.
func (s *QueryLogsRepositorySuite) TestConformQueryLogs_Lowering() {
	ctx := context.Background()
	now := time.Now().UTC()
	young := s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-time.Hour), domain: "young1"}, logRow{at: now.Add(-23 * time.Hour), domain: "young2"})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-25 * time.Hour), domain: "old"}, logRow{at: now.Add(-20 * 24 * time.Hour), domain: "older"})
	youngWeek := s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Hour)})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-3 * 24 * time.Hour)})
	kept := s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-time.Minute)})
	s.insertRows(queryLogsCollOneMonth, "other", logRow{at: now.Add(-20 * 24 * time.Hour)})

	moved, deleted, err := s.repo.ConformQueryLogs(ctx, "p1", model.RetentionOneDay, now)
	s.Require().NoError(err)
	s.Equal(3, moved)
	s.EqualValues(3, deleted)

	s.ElementsMatch(append(append([]primitive.ObjectID{}, young...), youngWeek...), s.ids(queryLogsCollOneDay, "p1"), "same _id in the new retention's collection")
	s.Zero(s.count(queryLogsCollOneMonth, "p1"))
	s.Zero(s.count(queryLogsCollOneWeek, "p1"))
	s.Equal(kept, s.ids(queryLogsCollOneHour, "p1"), "shorter collections keep their rows")
	s.EqualValues(1, s.count(queryLogsCollOneMonth, "other"))

	var doc struct {
		DNSRequest struct {
			Domain string `bson:"domain"`
		} `bson:"dns_request"`
	}
	s.Require().NoError(s.db.Collection(queryLogsCollOneDay).FindOne(ctx, bson.D{{Key: "_id", Value: young[0]}}).Decode(&doc))
	s.Equal("young1", doc.DNSRequest.Domain, "the row is copied unchanged")
}

// specRef: api-endpoint-behaviour.md J13 — the reconcile also deletes rows older than the retention from the
// collections it keeps (the TTL removes whole buckets, up to an hour late).
func (s *QueryLogsRepositorySuite) TestConformQueryLogs_TrimsRowsOlderThanRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	keep := s.insertRows(queryLogsCollOneDay, "p1", logRow{at: now.Add(-23 * time.Hour)})
	s.insertRows(queryLogsCollOneDay, "p1", logRow{at: now.Add(-25 * time.Hour)})

	moved, deleted, err := s.repo.ConformQueryLogs(ctx, "p1", model.RetentionOneDay, now)
	s.Require().NoError(err)
	s.Zero(moved)
	s.EqualValues(1, deleted)
	s.Equal(keep, s.ids(queryLogsCollOneDay, "p1"))
}

// specRef: api-endpoint-behaviour.md J54 — a move interrupted after a batch's insert and before its delete
// leaves every row exactly once when it runs again.
func (s *QueryLogsRepositorySuite) TestConformQueryLogs_RerunAfterInterruptedBatch() {
	ctx := context.Background()
	now := time.Now().UTC()
	rows := []logRow{{at: now.Add(-time.Hour), domain: "a"}, {at: now.Add(-2 * time.Hour), domain: "b"}}
	ids := s.insertRows(queryLogsCollOneWeek, "p1", rows...)
	// The interrupted batch: copies already in the target, source not yet deleted.
	cur, err := s.db.Collection(queryLogsCollOneWeek).Find(ctx, bson.D{{Key: "profile_id", Value: "p1"}})
	s.Require().NoError(err)
	var docs []bson.D
	s.Require().NoError(cur.All(ctx, &docs))
	copies := make([]any, 0, len(docs))
	for _, d := range docs {
		copies = append(copies, d)
	}
	_, err = s.db.Collection(queryLogsCollOneDay).InsertMany(ctx, copies)
	s.Require().NoError(err)

	_, _, err = s.repo.ConformQueryLogs(ctx, "p1", model.RetentionOneDay, now)
	s.Require().NoError(err)

	s.ElementsMatch(ids, s.ids(queryLogsCollOneDay, "p1"), "each row exactly once")
	s.Zero(s.count(queryLogsCollOneWeek, "p1"))
}

// specRef: api-endpoint-behaviour.md J54 — moves larger than one batch move every row.
func (s *QueryLogsRepositorySuite) TestConformQueryLogs_SeveralBatches() {
	ctx := context.Background()
	now := time.Now().UTC()
	rows := make([]logRow, 0, queryLogsMoveBatch+5)
	for i := 0; i < queryLogsMoveBatch+5; i++ {
		rows = append(rows, logRow{at: now.Add(-time.Duration(i) * time.Second)})
	}
	s.insertRows(queryLogsCollOneMonth, "p1", rows...)

	moved, _, err := s.repo.ConformQueryLogs(ctx, "p1", model.RetentionOneWeek, now)
	s.Require().NoError(err)
	s.Equal(len(rows), moved)
	s.EqualValues(len(rows), s.count(queryLogsCollOneWeek, "p1"))
	s.Zero(s.count(queryLogsCollOneMonth, "p1"))
}

// specRef: api-endpoint-behaviour.md J54 — a batch's source and target deletes use the meta+time index
// bounded to the batch's time range, not every bucket of the profile.
func (s *QueryLogsRepositorySuite) TestMoveBatchDelete_ExaminesOnlyTheBatchRange() {
	ctx := context.Background()
	hour := time.Now().UTC().Truncate(time.Hour)
	const hours = 120
	var docs []any
	for i := range hours {
		at := hour.Add(-time.Duration(i) * time.Hour)
		for range 10 {
			docs = append(docs, bson.D{{Key: "timestamp", Value: at}, {Key: "profile_id", Value: "p1"}})
		}
		for o := range 200 {
			docs = append(docs, bson.D{{Key: "timestamp", Value: at}, {Key: "profile_id", Value: fmt.Sprintf("other%d", o)}})
		}
	}
	coll := s.db.Collection(queryLogsCollOneMonth)
	_, err := coll.InsertMany(ctx, docs)
	s.Require().NoError(err)

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}).SetLimit(100)
	cur, err := coll.Find(ctx, bson.D{{Key: "profile_id", Value: "p1"}}, opts)
	s.Require().NoError(err)
	var batch []bson.D
	s.Require().NoError(cur.All(ctx, &batch))

	var out bson.M
	s.Require().NoError(s.db.RunCommand(ctx, bson.D{
		{Key: "explain", Value: bson.D{{Key: "delete", Value: queryLogsCollOneMonth}, {Key: "deletes", Value: bson.A{
			bson.D{{Key: "q", Value: queryLogsBatchFilter("p1", batch)}, {Key: "limit", Value: 0}}}}}},
		{Key: "verbosity", Value: "executionStats"}}).Decode(&out))
	stats := out["executionStats"].(bson.M)
	plan, err := bson.MarshalExtJSON(out["queryPlanner"].(bson.M)["winningPlan"], false, false)
	s.Require().NoError(err)

	s.Contains(string(plan), "profile_id_1_timestamp_1", "the batch delete must use the meta+time index: %s", plan)
	s.Less(toInt(stats["totalDocsExamined"]), hours/4,
		"the batch delete examined %v of the profile's %d buckets", stats["totalDocsExamined"], hours)
}
