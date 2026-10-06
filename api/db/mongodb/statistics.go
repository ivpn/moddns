package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Statistics tiers written by the proxy; created by migration 027. Purges and the
// profile-id listing span all of them.
const (
	statisticsColl15min = "statistics_15min"
	statisticsColl1h    = "statistics_1h"
	statisticsColl1d30d = "statistics_1d_30d"
	statisticsColl1d90d = "statistics_1d_90d"
	statisticsColl1d1y  = "statistics_1d_1y"
)

// statisticsBucketWidth is the bucket_start alignment of each tier.
var statisticsBucketWidth = map[string]time.Duration{
	statisticsColl15min: 15 * time.Minute,
	statisticsColl1h:    time.Hour,
	statisticsColl1d30d: 24 * time.Hour,
	statisticsColl1d90d: 24 * time.Hour,
	statisticsColl1d1y:  24 * time.Hour,
}

var statisticsCollectionNames = []string{
	statisticsColl15min, statisticsColl1h, statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y,
}

// StatisticsRepository is a MongoDB repository for the statistics time-series collections
type StatisticsRepository struct {
	colls []*mongo.Collection
}

// NewStatisticsRepository creates a new StatisticsRepository instance
func NewStatisticsRepository(client *mongo.Client, dbName string) StatisticsRepository {
	repo := StatisticsRepository{}
	for _, name := range statisticsCollectionNames {
		repo.colls = append(repo.colls, client.Database(dbName).Collection(name))
	}

	return repo
}

// tierCollections are the positions in statisticsCollectionNames a tier is read from.
var tierCollections = map[model.StatisticsTier][2]int{
	model.StatisticsTier15Min: {0, 1},
	model.StatisticsTier1H:    {1, 2},
	model.StatisticsTier1D:    {2, 5},
}

// statisticsCounterFields maps the response counter names to the flat stored
// fields (proxy-statistics-behaviour.md Y19).
var statisticsCounterFields = []struct{ out, stored string }{
	{"total", "total"}, {"blocked", "blocked"}, {"dnssec", "dnssec"},
	{"blocklist", "reason_blocklist"}, {"service", "reason_service"}, {"custom_rule", "reason_custom_rule"},
	{"rebinding", "reason_rebinding"}, {"default_rule", "reason_default_rule"}, {"other", "reason_other"},
	{"doh", "proto_doh"}, {"dot", "proto_dot"}, {"doq", "proto_doq"},
}

// sumGroup is a $group stage keyed by id that sums every counter; source maps a
// counter's response name to the field it reads.
func sumGroup(id any, counters []string, source func(string) string) bson.D {
	g := bson.D{{Key: "_id", Value: id}}
	for _, name := range counters {
		g = append(g, bson.E{Key: name, Value: bson.D{{Key: "$sum", Value: "$" + source(name)}}})
	}
	return bson.D{{Key: "$group", Value: g}}
}

// facetGroup sums counters already summed by the per-branch group.
func facetGroup(id any, counters ...string) bson.D {
	return sumGroup(id, counters, func(name string) string { return name })
}

type statisticsFacets struct {
	Series []struct {
		ID      time.Time `bson:"_id"`
		Total   int64     `bson:"total"`
		Blocked int64     `bson:"blocked"`
		Dnssec  int64     `bson:"dnssec"`
	} `bson:"series"`
	Totals []struct {
		Total   int64 `bson:"total"`
		Blocked int64 `bson:"blocked"`
		Dnssec  int64 `bson:"dnssec"`
	} `bson:"totals"`
	Reasons []struct {
		Blocklist   int64 `bson:"blocklist"`
		Service     int64 `bson:"service"`
		CustomRule  int64 `bson:"custom_rule"`
		Rebinding   int64 `bson:"rebinding"`
		DefaultRule int64 `bson:"default_rule"`
		Other       int64 `bson:"other"`
	} `bson:"reasons"`
	Protocols []struct {
		DoH int64 `bson:"doh"`
		DoT int64 `bson:"dot"`
		DoQ int64 `bson:"doq"`
	} `bson:"protocols"`
	Devices []struct {
		ID      string `bson:"_id"`
		Total   int64  `bson:"total"`
		Blocked int64  `bson:"blocked"`
	} `bson:"devices"`
}

// dateTruncBucket floors bucket_start to the series point. $dateTrunc bins are
// aligned to 2000-01-01T00:00Z, which for minute, hour and day units dividing a
// day is the same grid as the epoch floor used for the zero-fill.
func dateTruncBucket(bucket time.Duration) bson.D {
	unit, size := "minute", int64(bucket/time.Minute)
	switch {
	case bucket%(24*time.Hour) == 0:
		unit, size = "day", int64(bucket/(24*time.Hour))
	case bucket%time.Hour == 0:
		unit, size = "hour", int64(bucket/time.Hour)
	}
	return bson.D{{Key: "$dateTrunc", Value: bson.D{
		{Key: "date", Value: "$bucket_start"}, {Key: "unit", Value: unit}, {Key: "binSize", Value: size},
	}}}
}

// statisticsBranch is the per-collection prefix of the read: the profile's
// buckets in [from, to), collapsed to one document per (series point, device).
// A $match followed by a $group over flat counters is what lets the server use
// block-wise time-series processing.
func statisticsBranch(profileId string, from, to time.Time, bucket time.Duration) bson.A {
	counters := make([]string, len(statisticsCounterFields))
	stored := make(map[string]string, len(statisticsCounterFields))
	for i, f := range statisticsCounterFields {
		counters[i] = f.out
		stored[f.out] = f.stored
	}
	key := bson.D{{Key: "ts", Value: dateTruncBucket(bucket)}, {Key: "device", Value: "$meta.device_id"}}
	return bson.A{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "meta.profile_id", Value: profileId},
			{Key: "bucket_start", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lt", Value: to}}},
		}}},
		sumGroup(key, counters, func(name string) string { return stored[name] }),
	}
}

// statisticsReadPipeline builds the J4 aggregation for one tier: the branch of
// its first collection, a $unionWith of the same branch for the others, then one $facet.
func statisticsReadPipeline(colls []string, profileId string, from, to time.Time, bucket time.Duration) mongo.Pipeline {
	pipeline := mongo.Pipeline{}
	for _, stage := range statisticsBranch(profileId, from, to, bucket) {
		pipeline = append(pipeline, stage.(bson.D))
	}
	for _, name := range colls[1:] {
		pipeline = append(pipeline, bson.D{{Key: "$unionWith", Value: bson.D{
			{Key: "coll", Value: name},
			{Key: "pipeline", Value: statisticsBranch(profileId, from, to, bucket)},
		}}})
	}
	return append(pipeline, bson.D{{Key: "$facet", Value: bson.D{
		{Key: "series", Value: bson.A{facetGroup("$_id.ts", "total", "blocked", "dnssec"), bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}}}},
		{Key: "totals", Value: bson.A{facetGroup(nil, "total", "blocked", "dnssec")}},
		{Key: "reasons", Value: bson.A{facetGroup(nil, "blocklist", "service", "custom_rule", "rebinding", "default_rule", "other")}},
		{Key: "protocols", Value: bson.A{facetGroup(nil, "doh", "dot", "doq")}},
		{Key: "devices", Value: bson.A{facetGroup("$_id.device", "total", "blocked"), bson.D{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}, {Key: "_id", Value: 1}}}}}},
	}}})
}

// tierCollectionNames lists the collections a tier is stored in.
func tierCollectionNames(tier model.StatisticsTier) ([]string, error) {
	r, ok := tierCollections[tier]
	if !ok {
		return nil, errors.New("unknown statistics tier")
	}
	return statisticsCollectionNames[r[0]:r[1]], nil
}

// GetProfileStatistics aggregates the profile's statistics of one tier with
// bucket_start in [from, to) in one round trip.
func (r *StatisticsRepository) GetProfileStatistics(ctx context.Context, profileId string, tier model.StatisticsTier, from, to time.Time, bucket time.Duration) (*model.StatisticsAggregate, error) {
	names, err := tierCollectionNames(tier)
	if err != nil {
		return nil, err
	}
	pos := tierCollections[tier][0]
	cursor, err := r.colls[pos].Aggregate(ctx, statisticsReadPipeline(names, profileId, from, to, bucket))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []statisticsFacets
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	agg := &model.StatisticsAggregate{}
	if len(docs) == 0 {
		return agg, nil
	}
	f := docs[0]

	for _, p := range f.Series {
		agg.Series = append(agg.Series, model.StatisticsPoint{Ts: p.ID.UTC(), Total: p.Total, Blocked: p.Blocked, Dnssec: p.Dnssec})
	}
	if len(f.Totals) > 0 {
		agg.Totals = model.StatisticsTotals(f.Totals[0])
	}
	if len(f.Reasons) > 0 {
		agg.Reasons = model.StatisticsReasons(f.Reasons[0])
	}
	if len(f.Protocols) > 0 {
		agg.Protocols = model.StatisticsProtocols(f.Protocols[0])
	}
	for _, d := range f.Devices {
		agg.Devices = append(agg.Devices, model.StatisticsDevice{DeviceID: d.ID, Total: d.Total, Blocked: d.Blocked})
	}
	return agg, nil
}

// DeleteProfileStatistics deletes the profile's documents from every tier. Every
// collection is attempted; the errors are joined. A non-nil before is floored per
// tier (documents are stamped at their own bucket start), so each tier keeps the
// bucket containing it.
func (r *StatisticsRepository) DeleteProfileStatistics(ctx context.Context, profileId string, before *time.Time) error {
	var errs []error
	for i, coll := range r.colls {
		filter := bson.D{primitive.E{Key: "meta.profile_id", Value: profileId}}
		if before != nil {
			cutoff := before.UTC().Truncate(statisticsBucketWidth[statisticsCollectionNames[i]])
			filter = append(filter, bson.E{Key: "bucket_start", Value: bson.D{primitive.E{Key: "$lt", Value: cutoff}}})
		}
		if _, err := coll.DeleteMany(ctx, filter); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ListStatisticsProfileIDs returns the distinct meta.profile_id values across the
// retention collections. It aggregates with $group (a cursor, so no 16 MB result
// document) instead of distinct.
func (r *StatisticsRepository) ListStatisticsProfileIDs(ctx context.Context) ([]string, error) {
	group := bson.D{primitive.E{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$meta.profile_id"}}}}

	pipeline := mongo.Pipeline{group}
	for _, name := range statisticsCollectionNames[1:] {
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
			primitive.E{Key: "coll", Value: name},
			primitive.E{Key: "pipeline", Value: bson.A{group}},
		}}})
	}
	// The per-collection groups already emitted {_id: profile_id}; merge duplicates across collections.
	pipeline = append(pipeline, bson.D{primitive.E{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$_id"}}}})

	cursor, err := r.colls[0].Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}
