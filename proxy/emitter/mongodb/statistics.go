package mongodb

import (
	"context"
	"sync/atomic"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	statisticsColl30d = "statistics_30d"
	statisticsColl90d = "statistics_90d"
	statisticsColl1y  = "statistics_1y"

	collTypeTimeSeries = "timeseries"
)

var statisticsCollections = []string{statisticsColl30d, statisticsColl90d, statisticsColl1y}

// statisticsCollectionFor routes a retention setting to its collection; an
// empty or unknown value is kept in the shortest one rather than dropped.
func statisticsCollectionFor(retention string) string {
	switch retention {
	case "90d":
		return statisticsColl90d
	case "1y":
		return statisticsColl1y
	default:
		return statisticsColl30d
	}
}

type collectionLister interface {
	ListCollectionSpecifications(ctx context.Context, filter interface{}, opts ...*options.ListCollectionsOptions) ([]*mongo.CollectionSpecification, error)
}

type collectionInserter interface {
	InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error)
}

// StatisticsRepository inserts consented statistics into the time-series
// collections that an API migration creates. It never creates a collection:
// inserting into a missing one would silently create a plain collection.
type StatisticsRepository struct {
	lister   collectionLister
	inserter func(name string) collectionInserter
	verified atomic.Bool
}

// NewStatisticsRepository binds the repository and checks the collections once.
func NewStatisticsRepository(client *mongo.Client, dbName string) *StatisticsRepository {
	db := client.Database(dbName)
	repo := newStatisticsRepository(db, func(name string) collectionInserter { return db.Collection(name) })
	repo.Verify(context.Background())
	return repo
}

func newStatisticsRepository(lister collectionLister, inserter func(name string) collectionInserter) *StatisticsRepository {
	return &StatisticsRepository{lister: lister, inserter: inserter}
}

// Ready reports whether every statistics collection has been verified as time-series.
func (r *StatisticsRepository) Ready() bool {
	return r.verified.Load()
}

// Verify checks that all statistics collections exist as time-series and, once
// they do, stops the checks for good.
func (r *StatisticsRepository) Verify(ctx context.Context) bool {
	if r.verified.Load() {
		return true
	}
	specs, err := r.lister.ListCollectionSpecifications(ctx, bson.D{{Key: "name", Value: bson.D{{Key: "$in", Value: statisticsCollections}}}})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list statistics collections; consented statistics writes are disabled")
		return false
	}
	types := make(map[string]string, len(specs))
	for _, spec := range specs {
		types[spec.Name] = spec.Type
	}
	ok := true
	for _, name := range statisticsCollections {
		if types[name] != collTypeTimeSeries {
			log.Error().Str("collection_name", name).Str("collection_type", types[name]).Msg("Statistics collection is missing or not time-series; consented statistics writes are disabled")
			ok = false
		}
	}
	if ok {
		r.verified.Store(true)
		log.Info().Msg("Statistics collections verified")
	}
	return ok
}

// InsertStatistics writes a batch grouped by retention collection. While the
// collections are not verified the batch is dropped.
func (r *StatisticsRepository) InsertStatistics(ctx context.Context, batch []model.Statistics) error {
	if len(batch) == 0 {
		return nil
	}
	if !r.Verify(ctx) {
		log.Warn().Int("batch_size", len(batch)).Msg("Dropping consented statistics batch: collections are not ready")
		return nil
	}

	byColl := make(map[string][]interface{}, len(statisticsCollections))
	for _, doc := range batch {
		name := statisticsCollectionFor(doc.Retention)
		byColl[name] = append(byColl[name], doc)
	}
	var firstErr error
	for name, docs := range byColl {
		if _, err := r.inserter(name).InsertMany(ctx, docs, options.InsertMany().SetOrdered(false)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		log.Info().Str("collection_name", name).Int("batch_size", len(docs)).Msg("Inserted batch of statistics")
	}
	return firstErr
}
