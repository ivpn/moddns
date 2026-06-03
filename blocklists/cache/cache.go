package cache

import (
	"context"
	"errors"

	"github.com/ivpn/dns/libs/cache"
)

const CacheTypeRedis = "redis"

// Cache is an interface for caching functionalities
type Cache interface {
	CreateOrUpdateBlocklist(ctx context.Context, blocklistId string, data []byte) error
	DeleteBlocklist(ctx context.Context, blocklistId string) error
	// Ping reports whether the cache backend is reachable (used for readiness).
	Ping(ctx context.Context) error
}

// NewCache creates a new BlocklistCache instance
func NewCache(cacheCfg *cache.Config, cacheType string) (Cache, error) {
	switch cacheType { // nolint
	case CacheTypeRedis:
		return NewRedisCache(cacheCfg)
	}
	return nil, errors.New("unknown cache type")
}
