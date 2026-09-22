package emitter

import (
	"context"
	"errors"

	"github.com/ivpn/dns/libs/store"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/emitter/mongodb"
	"github.com/ivpn/dns/proxy/model"
)

type Emitter interface {
	EmitQueryLogs(ctx context.Context, data []model.EventQueryLog) error
	EmitServiceStatistics(ctx context.Context, data []model.ServiceStatistics) error
	Disconnect() error
}

func NewEmitter(sinkCfg config.SinkConfig) (Emitter, error) {
	switch sinkConfig := sinkCfg.(type) {
	case store.Config:
		mongoEmitter, err := mongodb.NewMongoDBEmitter(&sinkConfig)
		if err != nil {
			return nil, err
		}
		return mongoEmitter, nil
	default:
		return nil, errors.New("unknown sink config type")
	}
}
