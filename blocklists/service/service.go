package service

import (
	"github.com/ivpn/dns/blocklists/cache"
	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/db"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/blocklists/updater"
)

type Service struct {
	Cfg        config.Config
	Store      db.Db
	Cache      cache.Cache
	Updater    updater.Updater
	Metrics    metrics.Updates
	Blocklists []model.BlocklistMetadata
}

// NewService creates a new Service instance. If m is nil, a no-op metrics
// implementation is used so instrumentation calls are always safe.
func New(cfg config.Config, store db.Db, cache cache.Cache, updater updater.Updater, m metrics.Updates) *Service {
	if m == nil {
		m = metrics.NoopUpdates{}
	}
	return &Service{
		Cfg:     cfg,
		Store:   store,
		Cache:   cache,
		Updater: updater,
		Metrics: m,
	}
}
