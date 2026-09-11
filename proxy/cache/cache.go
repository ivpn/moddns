package cache

import (
	"context"
	"errors"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/proxy/model"
)

const CacheTypeRedis = "redis"

// ErrSettingsNotFound is model.ErrSettingsNotFound, re-exported for callers
// that only know the cache.
var ErrSettingsNotFound = model.ErrSettingsNotFound

// Cache is the proxy's read-only view of the settings store. Per-profile
// inputs come from one GetProfileSettingsBatch call and travel on the request
// context; GetBlocklistEntry is the only per-query lookup.
type Cache interface {
	// GetProfileSettingsBatch fetches every per-profile input in one batch.
	// It returns an error only when the store is unreachable; per-key
	// outcomes are reported on the returned ProfileSettings.
	GetProfileSettingsBatch(ctx context.Context, profileId string) (*model.ProfileSettings, error)
	// GetBlocklistEntry reports whether fqdn is a member of the blocklist set.
	GetBlocklistEntry(ctx context.Context, blocklistId string, domain string) (bool, error)

	// Close shuts down the cache and releases resources.
	Close()
}

// NewCache creates a new BlocklistCache instance
func NewCache(cacheCfg *cache.Config, cacheType string) (Cache, error) {
	switch cacheType { // nolint
	case CacheTypeRedis:
		return NewRedisCache(cacheCfg)
	}
	return nil, errors.New("unknown cache type")
}
