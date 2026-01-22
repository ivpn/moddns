package cache

import (
	"context"
	"errors"

	"github.com/ivpn/dns/libs/cache"
)

const CacheTypeRedis = "redis"

// Cache is an interface for caching functionalities
type Cache interface {
	GetProfileBlocklists(ctx context.Context, profileId string) ([]string, error)
	GetProfileServicesBlocked(ctx context.Context, profileId string) ([]string, error)
	GetProfileLogsSettings(ctx context.Context, profileId string) (map[string]string, error)
	GetProfileDNSSECSettings(ctx context.Context, profileId string) (map[string]string, error)
	GetProfileAdvancedSettings(ctx context.Context, profileId string) (map[string]string, error)
	GetProfileStatisticsSettings(ctx context.Context, profileId string) (map[string]string, error)
	GetProfilePrivacySettings(ctx context.Context, profileId string) (map[string]string, error)
	GetBlocklistEntry(ctx context.Context, blocklistId string, domain string) (bool, error)
	GetCustomRulesHashes(ctx context.Context, profileId string) ([]string, error)
	GetCustomRulesHash(ctx context.Context, hashId string) (map[string]string, error)
}

// NewCache creates a new BlocklistCache instance
func NewCache(cacheCfg *cache.Config, cacheType string) (Cache, error) {
	switch cacheType { // nolint
	case CacheTypeRedis:
		return NewRedisCache(cacheCfg)
	}
	return nil, errors.New("unknown cache type")
}
