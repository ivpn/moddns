package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ivpn/dns/libs/cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisCache is a cache implementation using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache instance
func NewRedisCache(cfg *cache.Config) (*RedisCache, error) {
	rdb, err := cache.NewRedisClient(cfg)
	if err != nil {
		return nil, err
	}

	return &RedisCache{
		client: rdb,
	}, nil
}

// GetProfileBlocklists gets blocklists profile subscribes from the cache
func (c *RedisCache) GetProfileBlocklists(ctx context.Context, profileId string) ([]string, error) {
	settingsKey := "settings:" + profileId
	settingsBlocklists := fmt.Sprintf("%s:%s", settingsKey, "blocklists")
	cmd := c.client.LRange(ctx, settingsBlocklists, 0, -1)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetProfileServicesBlocked gets blocked services (service IDs) for a profile.
func (c *RedisCache) GetProfileServicesBlocked(ctx context.Context, profileId string) ([]string, error) {
	settingsKey := "settings:" + profileId
	settingsServicesBlocked := fmt.Sprintf("%s:%s", settingsKey, "services:blocked")
	cmd := c.client.LRange(ctx, settingsServicesBlocked, 0, -1)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetProfileLogsSettings gets blocklists profile subscribes from the cache
func (c *RedisCache) GetProfileLogsSettings(ctx context.Context, profileId string) (map[string]string, error) {
	return c.getProfileSettings(ctx, profileId, "logs")
}

// GetProfilePrivacySettings gets blocklists profile privacy settings from the cache
func (c *RedisCache) GetProfilePrivacySettings(ctx context.Context, profileId string) (map[string]string, error) {
	return c.getProfileSettings(ctx, profileId, "privacy")
}

// GetProfileDNSSECSettings gets DNSSEC settings from the cache
func (c *RedisCache) GetProfileDNSSECSettings(ctx context.Context, profileId string) (map[string]string, error) {
	return c.getProfileSettings(ctx, profileId, "security", "dnssec")
}

func (c *RedisCache) GetProfileAdvancedSettings(ctx context.Context, profileId string) (map[string]string, error) {
	return c.getProfileSettings(ctx, profileId, "advanced")
}

// GetProfileStatisticsSettings gets profile statistics settings from the cache
func (c *RedisCache) GetProfileStatisticsSettings(ctx context.Context, profileId string) (map[string]string, error) {
	return c.getProfileSettings(ctx, profileId, "statistics")
}

// GetProfileSettings is generic profile settings getter from cache
func (c *RedisCache) getProfileSettings(ctx context.Context, profileId string, settingsName ...string) (map[string]string, error) {
	if profileId == "" {
		return nil, fmt.Errorf("profile ID cannot be empty")
	}
	if len(settingsName) == 0 {
		return nil, fmt.Errorf("settings name cannot be empty")
	}

	settingsKey := []string{"settings", profileId}
	if len(settingsName) > 0 {
		settingsKey = append(settingsKey, settingsName...)
	}
	settings := strings.Join(settingsKey, ":")

	cmd := c.client.HGetAll(ctx, settings)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	if len(cmd.Val()) == 0 {
		errMsg := fmt.Sprintf("No %s settings found for profile %s", settingsName, profileId)
		log.Warn().Msg(errMsg)
		return nil, errors.New(errMsg)
	}
	return cmd.Val(), nil
}

// GetBlocklistEntry checks if a domain is present in the blocklist
func (c *RedisCache) GetBlocklistEntry(ctx context.Context, blocklistId string, fqdn string) (bool, error) {
	blocklistKey := "blocklist:" + blocklistId
	cmd := c.client.SIsMember(ctx, blocklistKey, fqdn)
	if err := cmd.Err(); err != nil {
		return false, err
	}
	return cmd.Val(), nil
}

// GetCustomRulesHashes gets list of custom rules set names
func (c *RedisCache) GetCustomRulesHashes(ctx context.Context, profileId string) ([]string, error) {
	customRulesSetKey := fmt.Sprintf("settings:%s:custom_rules", profileId)
	cmd := c.client.SMembers(ctx, customRulesSetKey)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetCustomRulesHash gets custom rules hash
func (c *RedisCache) GetCustomRulesHash(ctx context.Context, hashId string) (map[string]string, error) {
	cmd := c.client.HGetAll(ctx, hashId)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}
