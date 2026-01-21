package cache

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/cache"
)

const CacheTypeRedis = "redis"

// Cache is an interface for caching functionalities
type Cache interface {
	CacheBase
	AddBlocklist(ctx context.Context, blocklistId string, data []byte) error
	CreateOrUpdateProfileSettings(ctx context.Context, settings *model.ProfileSettings, rollback bool) error
	AddCustomRule(ctx context.Context, profileId string, customRule *model.CustomRule) error
	RemoveCustomRule(ctx context.Context, profileId, customRuleId string) error
	DeleteProfileSettings(ctx context.Context, profileId string) error
	SetTOTPSecret(ctx context.Context, accountID, secret string, expiresIn time.Duration) error
	GetTOTPSecret(ctx context.Context, accountId string) (string, error)
	AppendBlocklistsToProfileSettings(ctx context.Context, profileId string, blocklistIds ...string) error
	RemoveBlocklistsFromProfileSettings(ctx context.Context, profileId string, blocklistIds ...string) error
	AppendServicesBlockedToProfileSettings(ctx context.Context, profileId string, serviceIds ...string) error
	RemoveServicesBlockedFromProfileSettings(ctx context.Context, profileId string, serviceIds ...string) error
	AddSubscription(ctx context.Context, subscriptionId string, activeUntil string, expiresIn time.Duration) error
	GetSubscription(ctx context.Context, subscriptionId string) (string, error)
	RemoveSubscription(ctx context.Context, subscriptionId string) error
}

// NewCache creates a new BlocklistCache instance
func NewCache(cacheCfg *cache.Config, cacheType string) (Cache, error) {
	switch cacheType { // nolint
	case CacheTypeRedis:
		return NewRedisCache(cacheCfg)
	}
	return nil, errors.New("unknown cache type")
}
