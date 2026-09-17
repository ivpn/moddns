package mongodb

import (
	"context"

	"github.com/ivpn/dns/libs/store"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
)

type MongoDBEmitter struct {
	DB *MongoDB
}

func NewMongoDBEmitter(dbCfg *store.Config) (*MongoDBEmitter, error) {
	storeI, err := store.New(store.DbTypeMongoDb, dbCfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create database struct")
		return nil, err
	}
	mongoDB, err := NewMongoDB(storeI, dbCfg)
	if err != nil {
		return nil, err
	}
	if err = mongoDB.RegisterRepositories(); err != nil {
		return nil, err
	}

	return &MongoDBEmitter{
		DB: mongoDB,
	}, nil
}

func (e *MongoDBEmitter) EmitQueryLogs(ctx context.Context, data []model.EventQueryLog) error {
	return e.DB.InsertBatch(ctx, data)
}

func (e *MongoDBEmitter) EmitServiceStatistics(ctx context.Context, data []model.ServiceStatistics) error {
	return e.DB.AddBatch(ctx, data)
}

func (e *MongoDBEmitter) Disconnect() error {
	return e.DB.Disconnect()
}
