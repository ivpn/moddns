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

// GetProfileStatistics sums the profile's query totals across all retention collections
func (r *StatisticsRepository) GetProfileStatistics(ctx context.Context, profileId string, timespan int) ([]model.StatisticsAggregated, error) {
	matchFilter := bson.D{
		primitive.E{Key: "meta.profile_id", Value: profileId},
	}

	if timespan != 0 {
		now := time.Now()
		matchFilter = append(matchFilter, bson.E{
			Key: "bucket_start",
			Value: bson.D{
				primitive.E{Key: "$lte", Value: now},
				primitive.E{Key: "$gte", Value: now.Add(time.Duration(-timespan) * time.Hour)},
			},
		})
	}

	match := bson.D{primitive.E{Key: "$match", Value: matchFilter}}

	pipeline := mongo.Pipeline{match}
	for _, name := range statisticsCollectionNames[1:] {
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
			primitive.E{Key: "coll", Value: name},
			primitive.E{Key: "pipeline", Value: bson.A{match}},
		}}})
	}
	pipeline = append(pipeline, bson.D{primitive.E{Key: "$group", Value: bson.D{
		primitive.E{Key: "_id", Value: nil},
		// Note: "total" needs to be the same as in the model
		primitive.E{Key: "total", Value: bson.D{
			primitive.E{Key: "$sum", Value: "$queries.total"},
		}},
	}}})

	cursor, err := r.colls[0].Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	results := make([]model.StatisticsAggregated, 0)
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []model.StatisticsAggregated{{Total: 0}}, nil
	}

	return results, nil
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
