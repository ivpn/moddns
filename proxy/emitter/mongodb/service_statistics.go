package mongodb

import (
	"context"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ServiceStatisticsRepository writes service-wide query counters. The
// collection is keyed by PoP and holds no profile or device field.
type ServiceStatisticsRepository struct {
	client   *mongo.Client
	database *mongo.Database
	DbName   string
	collName string
	coll     *mongo.Collection
}

// NewServiceStatisticsRepository creates the repository and its time-series
// collection when missing.
func NewServiceStatisticsRepository(client *mongo.Client, dbName, collName string) (*ServiceStatisticsRepository, error) {
	database := client.Database(dbName)

	repo := &ServiceStatisticsRepository{
		client:   client,
		database: database,
		DbName:   dbName,
		collName: collName,
		coll:     database.Collection(collName),
	}
	if err := repo.createCollection(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

// InsertBatch inserts one document per flush and PoP.
func (r *ServiceStatisticsRepository) InsertBatch(ctx context.Context, batch []model.ServiceStatistics) error {
	docs := make([]any, 0, len(batch))
	for i := range batch {
		batch[i].ID = primitive.NewObjectID()
		docs = append(docs, batch[i])
	}
	if len(docs) == 0 {
		return nil
	}

	_, err := r.coll.InsertMany(ctx, docs, &options.InsertManyOptions{
		Ordered: new(bool),
	})
	if err != nil {
		return err
	}
	log.Info().Str("collection_name", r.collName).Int("batch_size", len(docs)).Msg("Inserted batch of service stats")
	return nil
}

// createCollection creates the time-series collection with meta field "pop"
// and no expiry: the counters are anonymous and serve long-term reporting.
func (r *ServiceStatisticsRepository) createCollection(ctx context.Context) error {
	existingCollNames, err := r.database.ListCollectionNames(ctx, bson.D{}, nil)
	if err != nil {
		log.Err(err).Msg("Error listing collection names")
		return err
	}

	if contains(existingCollNames, r.collName) {
		log.Info().Msgf("%s collection already exists. continuing.", r.collName)
		return nil
	}
	err = r.database.CreateCollection(
		ctx,
		r.collName,
		&options.CreateCollectionOptions{
			TimeSeriesOptions: &options.TimeSeriesOptions{
				TimeField:   timeField,
				MetaField:   &metafieldPop,
				Granularity: &granularityMinutes,
			},
		},
	)
	if err != nil {
		log.Err(err).Msgf("Error creating collection [%s]", r.collName)
		return err
	}
	log.Info().Msgf("Successfully created %s collection for the first time.", r.collName)
	r.coll = r.database.Collection(r.collName)
	return nil
}
