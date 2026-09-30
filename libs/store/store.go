package store

import (
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
)

const (
	DbTypeMongoDb = "mongodb"
)

// Store is an interface for database functionalities
type Store interface {
	GetClient() *mongo.Client
	Disconnect() error
	Migrate() error
}

// NewStore creates a new Db instance
func New(dbType string, dbConfig *Config) (Store, error) {
	if dbType == DbTypeMongoDb {
		return NewMongoDB(dbConfig)
	}
	return nil, errors.New("unknown db type")
}
