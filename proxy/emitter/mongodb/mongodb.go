package mongodb

import (
	"github.com/ivpn/dns/libs/store"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	collNameQueryLogs    = "query_logs"
	collNameServiceStats = "service_statistics"
)

// MongoDB is a MongoDB database instance
type MongoDB struct {
	store.Store
	dbConfig *store.Config
	client   *mongo.Client
	*QueryLogsRepository
	*ServiceStatisticsRepository
}

// NewMongoDB creates a new MongoDB instance
func NewMongoDB(storeI store.Store, dbConfig *store.Config) (*MongoDB, error) {
	return &MongoDB{
		Store:    storeI,
		client:   storeI.GetClient(),
		dbConfig: dbConfig,
	}, nil
}

// RegisterRepositories registers MongoDB repositories
func (db *MongoDB) RegisterRepositories() error {
	var err error
	db.QueryLogsRepository, err = NewQueryLogsRepository(db.client, db.dbConfig.Name, collNameQueryLogs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create query logs repository")
		return err
	}
	db.ServiceStatisticsRepository, err = NewServiceStatisticsRepository(db.client, db.dbConfig.Name, collNameServiceStats)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create service statistics repository")
		return err
	}
	return nil
}
