package mongodb

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func domainsOf(logs []model.QueryLog) []string {
	out := make([]string, 0, len(logs))
	for _, l := range logs {
		out = append(out, l.DNSRequest.Domain)
	}
	return out
}

// specRef: api-endpoint-behaviour.md J1, J56 — the list reads every collection holding rows of the profile,
// newest first across them, and pages over the merged order.
func (s *QueryLogsRepositorySuite) TestGetQueryLogs_ReadsEveryCollection() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-1 * time.Minute), domain: "a"}, logRow{at: now.Add(-4 * time.Minute), domain: "d"})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Minute), domain: "b"})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-3 * time.Minute), domain: "c"})
	s.insertRows(queryLogsCollOneDay, "other", logRow{at: now.Add(-time.Minute), domain: "other"})

	got, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 0, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"a", "b", "c", "d"}, domainsOf(got), "shorter and longer collections are both read")

	page2, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 0, "", "", "", 2, 2)
	s.Require().NoError(err)
	s.Equal([]string{"c", "d"}, domainsOf(page2), "pages follow the merged order")

	byDomain, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneWeek, model.QueryLogStatusAll, 1, "", "", "domain", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"a", "b", "c", "d"}, domainsOf(byDomain), "the sort applies across collections")

	none, err := s.repo.GetQueryLogs(ctx, "nobody", model.RetentionOneWeek, model.QueryLogStatusAll, 0, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Empty(none)
}

// specRef: api-endpoint-behaviour.md J1, J56 — no read returns a row older than the current retention, even
// with a longer timespan and while the purge has not removed it yet.
func (s *QueryLogsRepositorySuite) TestGetQueryLogs_CappedAtRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-23 * time.Hour), domain: "kept"}, logRow{at: now.Add(-25 * time.Hour), domain: "too-old"})

	month, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneDay, model.QueryLogStatusAll, 720, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"kept"}, domainsOf(month))

	all, err := s.repo.GetQueryLogs(ctx, "p1", model.RetentionOneDay, model.QueryLogStatusAll, 0, "", "", "", 0, 0)
	s.Require().NoError(err)
	s.Equal([]string{"kept"}, domainsOf(all), "no timespan reads the whole retention")
}

// specRef: api-endpoint-behaviour.md J5, J56 — devices merge across collections, keep the latest last_seen and
// leave out rows older than the retention.
func (s *QueryLogsRepositorySuite) TestGetQueryLogDevices_ReadsEveryCollectionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-time.Minute), deviceID: "laptop"})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Hour), deviceID: "laptop"}, logRow{at: now.Add(-3 * time.Hour), deviceID: "phone"})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-25 * time.Hour), deviceID: "gone"})

	got, err := s.repo.GetQueryLogDevices(ctx, "p1", model.RetentionOneDay)
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal("laptop", got[0].DeviceId)
	s.True(now.Add(-time.Minute).Equal(got[0].LastSeen.UTC()), "latest across collections: %v", got[0].LastSeen)
	s.Equal("phone", got[1].DeviceId)
}

// specRef: api-endpoint-behaviour.md J20, J21, J24, J56 — top lists sum counts across collections, up to the retention.
func (s *QueryLogsRepositorySuite) TestTopLists_ReadEveryCollectionUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	blocked := func(age time.Duration, domain, ip string, reasons ...string) logRow {
		return logRow{at: now.Add(-age), status: model.QueryLogStatusBlocked, domain: domain, clientIP: ip, reasons: reasons}
	}
	s.insertRows(queryLogsCollOneHour, "p1", blocked(time.Minute, "x.com", "10.0.0.1", "blocklist: a"), blocked(time.Minute, "y.com", "10.0.0.2", "blocklist: b"))
	s.insertRows(queryLogsCollOneDay, "p1", blocked(time.Minute, "x.com", "10.0.0.2", "blocklist: b"), blocked(2*time.Hour, "x.com", "10.0.0.2", "blocklist: b"))
	s.insertRows(queryLogsCollOneDay, "p1", blocked(7*time.Hour, "old.com", "10.0.0.9", "blocklist: z"))

	domains, err := s.repo.GetQueryLogTopDomains(ctx, "p1", model.RetentionSixHours, model.QueryLogStatusBlocked, 24, 10)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "x.com", Count: 3}, {Domain: "y.com", Count: 1}}, domains)

	limited, err := s.repo.GetQueryLogTopDomains(ctx, "p1", model.RetentionSixHours, model.QueryLogStatusBlocked, 24, 1)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "x.com", Count: 3}}, limited, "the limit applies after merging")

	clients, err := s.repo.GetQueryLogTopClients(ctx, "p1", model.RetentionSixHours, 24, 10)
	s.Require().NoError(err)
	s.Require().Len(clients, 2)
	s.Equal("10.0.0.2", clients[0].IP)
	s.EqualValues(3, clients[0].Count)
	s.Equal("10.0.0.1", clients[1].IP)

	lists, err := s.repo.GetQueryLogTopBlocklists(ctx, "p1", model.RetentionSixHours, 24, 10)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopBlocklist{{BlocklistID: "b", Count: 3}, {BlocklistID: "a", Count: 1}}, lists)
}

// specRef: api-endpoint-behaviour.md J20, J56 — covered_from is the oldest row in the window across collections,
// never older than the retention.
func (s *QueryLogsRepositorySuite) TestGetQueryLogCoveredFrom_AcrossCollectionsUpToRetention() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Hour)})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-20 * time.Hour)})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-30 * time.Hour)})

	got, err := s.repo.GetQueryLogCoveredFrom(ctx, "p1", model.RetentionOneDay, 48)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.True(now.Add(-20*time.Hour).Truncate(time.Minute).Equal(*got), "the row beyond the retention is left out: %v", *got)
}

// specRef: api-endpoint-behaviour.md J54, J13 — the trim deletes the profile's rows older than the retention from
// every longer collection; newer rows stay where they are, and shorter collections and other profiles are untouched.
func (s *QueryLogsRepositorySuite) TestTrimQueryLogs() {
	ctx := context.Background()
	now := time.Now().UTC()
	keptMonth := s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-time.Hour)}, logRow{at: now.Add(-23 * time.Hour)})
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-25 * time.Hour)}, logRow{at: now.Add(-20 * 24 * time.Hour)})
	keptWeek := s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-2 * time.Hour)})
	s.insertRows(queryLogsCollOneWeek, "p1", logRow{at: now.Add(-3 * 24 * time.Hour)})
	keptHour := s.insertRows(queryLogsCollOneHour, "p1", logRow{at: now.Add(-time.Minute)})
	other := s.insertRows(queryLogsCollOneMonth, "other", logRow{at: now.Add(-20 * 24 * time.Hour)})

	deleted, err := s.repo.TrimQueryLogs(ctx, "p1", model.RetentionOneDay, now)
	s.Require().NoError(err)
	s.EqualValues(3, deleted)

	s.ElementsMatch(keptMonth, s.ids(queryLogsCollOneMonth, "p1"), "rows inside the retention are not moved")
	s.ElementsMatch(keptWeek, s.ids(queryLogsCollOneWeek, "p1"))
	s.Equal(keptHour, s.ids(queryLogsCollOneHour, "p1"))
	s.Equal(other, s.ids(queryLogsCollOneMonth, "other"))

	again, err := s.repo.TrimQueryLogs(ctx, "p1", model.RetentionOneDay, now)
	s.Require().NoError(err)
	s.Zero(again, "a repeated trim deletes nothing")
}

// specRef: api-endpoint-behaviour.md J54 — the longest retention has no longer collection to trim.
func (s *QueryLogsRepositorySuite) TestTrimQueryLogs_LongestRetentionIsANoop() {
	ctx := context.Background()
	now := time.Now().UTC()
	s.insertRows(queryLogsCollOneMonth, "p1", logRow{at: now.Add(-40 * 24 * time.Hour)})

	deleted, err := s.repo.TrimQueryLogs(ctx, "p1", model.RetentionOneMonth, now)
	s.Require().NoError(err)
	s.Zero(deleted, "the collection's own TTL bounds it")
}
