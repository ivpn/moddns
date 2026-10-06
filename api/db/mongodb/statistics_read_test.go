package mongodb

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// blockProcessingSupported reports whether a server version string (buildInfo
// "version", for example "8.0.9") has block-wise time-series processing, which
// arrived in 8.0.
func blockProcessingSupported(version string) bool {
	major, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= 8
}

// specRef: api-endpoint-behaviour.md J47 — the block_group guard only applies where block processing exists.
func TestBlockProcessingSupported(t *testing.T) {
	for version, want := range map[string]bool{
		"7.0.8": false, "6.3.2": false, "7.0.14-rc1": false,
		"8.0.0": true, "8.0.9": true, "8.2.3": true, "10.1.0": true,
		"": false, "garbage": false,
	} {
		require.Equal(t, want, blockProcessingSupported(version), version)
	}
}

// statDoc is one stored measurement in the flat layout (proxy-statistics-behaviour.md Y19).
type statDoc struct {
	profile, device string
	at              time.Time
	total, blocked  int
	reasons         [6]int // blocklist, service, custom_rule, rebinding, default_rule, other
	protocols       [3]int // doh, dot, doq
}

func (s *StatisticsRepositorySuite) insertDoc(coll string, d statDoc) {
	_, err := s.client.Database(s.dbName).Collection(coll).InsertOne(context.Background(), bson.D{
		{Key: "bucket_start", Value: d.at},
		{Key: "meta", Value: bson.D{{Key: "profile_id", Value: d.profile}, {Key: "device_id", Value: d.device}}},
		{Key: "total", Value: d.total}, {Key: "blocked", Value: d.blocked}, {Key: "dnssec", Value: d.total / 2},
		{Key: "reason_blocklist", Value: d.reasons[0]}, {Key: "reason_service", Value: d.reasons[1]}, {Key: "reason_custom_rule", Value: d.reasons[2]},
		{Key: "reason_rebinding", Value: d.reasons[3]}, {Key: "reason_default_rule", Value: d.reasons[4]}, {Key: "reason_other", Value: d.reasons[5]},
		{Key: "proto_doh", Value: d.protocols[0]}, {Key: "proto_dot", Value: d.protocols[1]}, {Key: "proto_doq", Value: d.protocols[2]},
	})
	s.Require().NoError(err)
}

// specRef: api-endpoint-behaviour.md J4, J43, J44 — one aggregation sums every collection of the tier within [from, to) and facets series, totals, reasons, protocols and devices.
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_FacetsAcrossTierCollections() {
	ctx := context.Background()
	from := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	b := func(h, m int) time.Time { return from.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }

	// 1h tier: two measurements of one (device, bucket), as written by two proxy instances (Y19).
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "laptop", at: b(1, 0), total: 10, blocked: 4, reasons: [6]int{3, 1, 0, 0, 0, 0}, protocols: [3]int{6, 4, 0}})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "laptop", at: b(1, 0), total: 5, blocked: 1, reasons: [6]int{0, 0, 1, 0, 0, 0}, protocols: [3]int{5, 0, 0}})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "laptop", at: b(2, 0), total: 20, blocked: 2, reasons: [6]int{0, 0, 0, 1, 1, 0}, protocols: [3]int{0, 20, 0}})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "", at: b(3, 0), total: 7, protocols: [3]int{0, 0, 7}})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "phone", at: b(23, 0), total: 1, blocked: 1, reasons: [6]int{0, 0, 0, 0, 0, 1}})
	// Excluded: before from, exactly at to, other profile, other tiers.
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "laptop", at: from.Add(-time.Minute), total: 1000})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "laptop", at: to, total: 1000})
	s.insertDoc(statisticsColl1h, statDoc{profile: "p2", device: "laptop", at: b(1, 0), total: 1000})
	s.insertDoc(statisticsColl15min, statDoc{profile: "p1", device: "laptop", at: b(1, 0), total: 1000})
	s.insertDoc(statisticsColl1d1y, statDoc{profile: "p1", device: "laptop", at: from, total: 1000})

	got, err := s.repo.GetProfileStatistics(ctx, "p1", model.StatisticsTier1H, from, to, time.Hour)
	s.Require().NoError(err)

	s.Equal(model.StatisticsTotals{Total: 43, Blocked: 8, Dnssec: 5 + 2 + 10 + 3 + 0}, got.Totals)
	s.Equal(model.StatisticsReasons{Blocklist: 3, Service: 1, CustomRule: 1, Rebinding: 1, DefaultRule: 1, Other: 1}, got.Reasons)
	s.Equal(model.StatisticsProtocols{DoH: 11, DoT: 24, DoQ: 7}, got.Protocols)

	s.Require().Len(got.Series, 4)
	for i, want := range []struct {
		at    time.Time
		total int64
	}{{b(1, 0), 15}, {b(2, 0), 20}, {b(3, 0), 7}, {b(23, 0), 1}} {
		s.True(got.Series[i].Ts.Equal(want.at), "point %d at %s", i, got.Series[i].Ts)
		s.Equal(want.total, got.Series[i].Total)
	}
	s.EqualValues(5, got.Series[0].Blocked)

	s.Equal([]model.StatisticsDevice{
		{DeviceID: "laptop", Total: 35, Blocked: 7},
		{DeviceID: "", Total: 7, Blocked: 0},
		{DeviceID: "phone", Total: 1, Blocked: 1},
	}, got.Devices)
}

// specRef: api-endpoint-behaviour.md J4, J40 — the 1d tier reads all three retention collections.
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_DailyTierUnionsRetentionCollections() {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range []string{statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y} {
		s.insertDoc(c, statDoc{profile: "p1", device: "d", at: from.Add(time.Duration(i) * 24 * time.Hour), total: 10 * (i + 1)})
	}
	// Hourly writes to the day tier are additive measurements stamped at the day start.
	s.insertDoc(statisticsColl1d1y, statDoc{profile: "p1", device: "d", at: from.Add(2 * 24 * time.Hour), total: 5})

	got, err := s.repo.GetProfileStatistics(context.Background(), "p1", model.StatisticsTier1D, from, from.AddDate(0, 0, 7), 24*time.Hour)
	s.Require().NoError(err)
	s.EqualValues(65, got.Totals.Total)
	s.Require().Len(got.Series, 3)
	s.EqualValues(35, got.Series[2].Total)
}

// specRef: api-endpoint-behaviour.md J41 — point widths floor on the epoch-aligned UTC grid the zero-fill uses.
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_PointWidthsMatchGoTruncation() {
	ctx := context.Background()
	base := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	var ats []time.Time
	for _, m := range []int{0, 7, 14, 15, 44, 59, 60, 61, 119, 120, 1439, 1440, 1441, 3000} {
		at := base.Add(time.Duration(m) * time.Minute)
		ats = append(ats, at)
		s.insertDoc(statisticsColl15min, statDoc{profile: "p1", device: "d", at: at, total: 1})
		s.insertDoc(statisticsColl1h, statDoc{profile: "p1", device: "d", at: at, total: 1})
		s.insertDoc(statisticsColl1d30d, statDoc{profile: "p1", device: "d", at: at, total: 1})
	}
	for _, tc := range []struct {
		tier   model.StatisticsTier
		bucket time.Duration
	}{
		{model.StatisticsTier15Min, 15 * time.Minute},
		{model.StatisticsTier1H, time.Hour},
		{model.StatisticsTier1D, 24 * time.Hour},
	} {
		want := map[time.Time]int64{}
		for _, at := range ats {
			want[at.Truncate(tc.bucket)]++
		}
		got, err := s.repo.GetProfileStatistics(ctx, "p1", tc.tier, base, base.Add(4*24*time.Hour), tc.bucket)
		s.Require().NoError(err)
		s.Require().Len(got.Series, len(want), tc.bucket.String())
		for _, p := range got.Series {
			s.Equal(want[p.Ts], p.Total, "%s point %s", tc.bucket, p.Ts)
		}
	}
}

// specRef: api-endpoint-behaviour.md J4, J42 — nothing in range is an empty aggregate.
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_Empty() {
	now := time.Now().UTC()
	got, err := s.repo.GetProfileStatistics(context.Background(), "nobody", model.StatisticsTier1H, now.Add(-time.Hour), now, time.Hour)
	s.Require().NoError(err)
	s.Equal(model.StatisticsTotals{}, got.Totals)
	s.Empty(got.Series)
	s.Empty(got.Devices)
}

// specRef: api-endpoint-behaviour.md J4 — an unknown tier is an error, never a read of some default collection.
func (s *StatisticsRepositorySuite) TestGetProfileStatistics_UnknownTier() {
	now := time.Now().UTC()
	_, err := s.repo.GetProfileStatistics(context.Background(), "p1", model.StatisticsTier("1min"), now.Add(-time.Hour), now, time.Hour)
	s.Error(err)
}

// specRef: api-endpoint-behaviour.md J47 — the first $match and every $unionWith branch of each tier's read use the profile index.
func (s *StatisticsRepositorySuite) TestReadPipelineUsesProfileIndexOnEveryTier() {
	now := time.Now().UTC()
	for _, c := range statisticsCollectionNames {
		for i := 0; i < 30; i++ {
			s.insertDoc(c, statDoc{profile: "p" + string(rune('a'+i)), device: "d", at: now.Add(-time.Hour), total: 1})
		}
	}
	for _, tier := range []model.StatisticsTier{model.StatisticsTier15Min, model.StatisticsTier1H, model.StatisticsTier1D} {
		names, err := tierCollectionNames(tier)
		s.Require().NoError(err)
		plan := s.explainPlan(bson.D{{Key: "aggregate", Value: names[0]}, {Key: "cursor", Value: bson.D{}},
			{Key: "pipeline", Value: statisticsReadPipeline(names, "pb", now.Add(-24*time.Hour), now, time.Hour)}})
		s.NotContains(plan, "COLLSCAN", string(tier))
		s.GreaterOrEqual(strings.Count(plan, profileIndexName), len(names), "%s: %s", tier, plan)
	}
}

// specRef: api-endpoint-behaviour.md J47
//
// This guards a performance property, not a result: the LAST_YEAR read must run
// as block-wise time-series processing (block_group). If it fails after a server
// upgrade, MongoDB no longer block-processes this $match+$group shape and the
// year view is several times slower; re-measure with BENCH_STATISTICS and decide
// before accepting the upgrade.
func (s *StatisticsRepositorySuite) TestLastYearReadUsesBlockGroup_ServerUpgradeGuard() {
	var info struct {
		Version string `bson:"version"`
	}
	s.Require().NoError(s.client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "buildInfo", Value: 1}}).Decode(&info))
	if !blockProcessingSupported(info.Version) {
		s.T().Skipf("MongoDB %s has no block-wise time-series processing (8.0+): nothing to guard", info.Version)
	}

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		s.insertDoc(statisticsColl1d1y, statDoc{profile: "p1", device: "d", at: now.Add(-time.Duration(i) * 24 * time.Hour), total: 1})
	}
	names, err := tierCollectionNames(model.StatisticsTier1D)
	s.Require().NoError(err)
	var out bson.M
	s.Require().NoError(s.client.Database(s.dbName).RunCommand(context.Background(), bson.D{
		{Key: "explain", Value: bson.D{{Key: "aggregate", Value: names[0]}, {Key: "cursor", Value: bson.D{}},
			{Key: "pipeline", Value: statisticsReadPipeline(names, "p1", now.Add(-365*24*time.Hour), now, 24*time.Hour)}}},
		{Key: "verbosity", Value: "queryPlanner"},
	}).Decode(&out))
	b, err := bson.MarshalExtJSON(out, false, false)
	s.Require().NoError(err)
	s.Contains(string(b), "block_group", "block-wise time-series $group is gone on this server version: LAST_YEAR reads will be much slower")
}
