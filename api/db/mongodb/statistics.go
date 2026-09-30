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

// Retention collections written by the proxy; created by migration 027.
var statisticsCollectionNames = []string{"statistics_30d", "statistics_90d", "statistics_1y"}

// StatisticsRepository is a MongoDB repository for the statistics time-series collections
type StatisticsRepository struct {
	DbName string
	colls  []*mongo.Collection
}

// NewStatisticsRepository creates a new StatisticsRepository instance
func NewStatisticsRepository(client *mongo.Client, dbName string) StatisticsRepository {
	repo := StatisticsRepository{DbName: dbName}
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

// DeleteProfileStatistics deletes the profile's documents from every retention
// collection. Every collection is attempted; the errors are joined.
func (r *StatisticsRepository) DeleteProfileStatistics(ctx context.Context, profileId string, before *time.Time) error {
	filter := bson.D{primitive.E{Key: "meta.profile_id", Value: profileId}}
	if before != nil {
		filter = append(filter, bson.E{Key: "bucket_start", Value: bson.D{primitive.E{Key: "$lt", Value: *before}}})
	}

	var errs []error
	for _, coll := range r.colls {
		if _, err := coll.DeleteMany(ctx, filter); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
