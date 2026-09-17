package mongodb

import (
	"context"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ServiceStatisticsRepository adds service-wide query counters into one
// document per PoP and hour. It is a regular collection: a time-series
// collection cannot increment measurement fields in place.
type ServiceStatisticsRepository struct {
	collName string
	coll     *mongo.Collection
}

// NewServiceStatisticsRepository binds the repository to its collection. The
// collection needs no explicit creation, indexes or TTL: documents are keyed
// by _id and kept for good.
func NewServiceStatisticsRepository(client *mongo.Client, dbName, collName string) (*ServiceStatisticsRepository, error) {
	return &ServiceStatisticsRepository{
		collName: collName,
		coll:     client.Database(dbName).Collection(collName),
	}, nil
}

// AddBatch increments each document's counters, creating it on first sight.
// Increments are atomic, so several proxy instances and restarts converge on
// the exact hourly total.
func (r *ServiceStatisticsRepository) AddBatch(ctx context.Context, batch []model.ServiceStatistics) error {
	if len(batch) == 0 {
		return nil
	}
	writes := make([]mongo.WriteModel, 0, len(batch))
	for _, doc := range batch {
		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "_id", Value: doc.ID}}).
			SetUpdate(bson.D{
				{Key: "$inc", Value: bson.D{
					{Key: "queries.total", Value: doc.Queries.Total},
					{Key: "queries.blocked", Value: doc.Queries.Blocked},
					{Key: "queries.dnssec", Value: doc.Queries.DNSSEC},
				}},
				{Key: "$setOnInsert", Value: bson.D{
					{Key: "timestamp", Value: doc.Timestamp},
					{Key: "pop", Value: doc.Pop},
				}},
			}).
			SetUpsert(true))
	}

	_, err := r.coll.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return err
	}
	log.Info().Str("collection_name", r.collName).Int("batch_size", len(writes)).Msg("Added batch of service stats")
	return nil
}
