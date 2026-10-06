package mongodb

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	statisticsColl15Min = "statistics_15min"
	statisticsColl1Hour = "statistics_1h"
	statisticsColl1d30d = "statistics_1d_30d"
	statisticsColl1d90d = "statistics_1d_90d"
	statisticsColl1d1y  = "statistics_1d_1y"

	collTypeTimeSeries = "timeseries"

	// statisticsVerifyTimeout bounds the check made at startup.
	statisticsVerifyTimeout = 5 * time.Second
)

var statisticsCollections = []string{statisticsColl15Min, statisticsColl1Hour, statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y}

// statisticsCollectionFor routes a document by tier and, for the 1-day tier, by
// retention; an empty or unknown retention is kept in the shortest collection rather
// than dropped.
func statisticsCollectionFor(tier model.StatisticsTier, retention model.StatisticsRetention) string {
	switch tier {
	case model.StatisticsTier1Hour:
		return statisticsColl1Hour
	case model.StatisticsTier1Day:
		switch retention {
		case model.StatisticsRetention90d:
			return statisticsColl1d90d
		case model.StatisticsRetention1y:
			return statisticsColl1d1y
		default:
			return statisticsColl1d30d
		}
	default:
		return statisticsColl15Min
	}
}

type collectionLister interface {
	ListCollectionSpecifications(ctx context.Context, filter any, opts ...*options.ListCollectionsOptions) ([]*mongo.CollectionSpecification, error)
}

type collectionInserter interface {
	InsertMany(ctx context.Context, documents []any, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error)
}

// StatisticsRepository inserts consented statistics into the time-series
// collections that an API migration creates. It never creates a collection:
// inserting into a missing one would silently create a plain collection.
type StatisticsRepository struct {
	lister   collectionLister
	inserter func(name string) collectionInserter
	verified atomic.Bool
}

// NewStatisticsRepository binds the repository and runs the first collection check with a
// timeout. While the check has not passed, every emit retries it; once it passes it is final.
func NewStatisticsRepository(client *mongo.Client, dbName string) *StatisticsRepository {
	db := client.Database(dbName)
	repo := newStatisticsRepository(db, func(name string) collectionInserter { return db.Collection(name) })
	repo.VerifyAtStartup()
	return repo
}

func newStatisticsRepository(lister collectionLister, inserter func(name string) collectionInserter) *StatisticsRepository {
	return &StatisticsRepository{lister: lister, inserter: inserter}
}

// VerifyAtStartup runs the first check under a bounded context.
func (r *StatisticsRepository) VerifyAtStartup() {
	ctx, cancel := context.WithTimeout(context.Background(), statisticsVerifyTimeout)
	defer cancel()
	r.Verify(ctx)
}

// Verify checks that all statistics collections exist as time-series. Once it passes it is
// never repeated, so a collection dropped while the proxy runs goes unnoticed.
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

	byColl := make(map[string][]any, len(statisticsCollections))
	for _, doc := range batch {
		name := statisticsCollectionFor(doc.Tier, doc.Retention)
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
