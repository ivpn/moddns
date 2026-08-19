package cache

import (
	"context"
	"errors"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/dislock"
)

const CacheTypeRedis = "redis"

// Cache is an interface for caching functionalities
type Cache interface {
	CreateOrUpdateBlocklist(ctx context.Context, blocklistId string, data []byte) error
	DeleteBlocklist(ctx context.Context, blocklistId string) error
	// BlocklistExists reports whether the live set for blocklistId is present
	// in the cache (used by the freshness check: metadata alone cannot prove
	// the published data survived, e.g. a cache flush).
	BlocklistExists(ctx context.Context, blocklistId string) (bool, error)
	// Ping reports whether the cache backend is reachable (used for readiness).
	Ping(ctx context.Context) error
	// Locker returns a distributed locker sharing the cache's backend, so
	// instances writing the same data coordinate through the same store.
	Locker(prefix string) *dislock.Locker
}

// NewCache creates a new BlocklistCache instance
func NewCache(cacheCfg *cache.Config, cacheType string) (Cache, error) {
	switch cacheType { // nolint
	case CacheTypeRedis:
		return NewRedisCache(cacheCfg)
	}
	return nil, errors.New("unknown cache type")
}
