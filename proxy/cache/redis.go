package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/proxy/model"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisCache is a cache implementation using Redis
type RedisCache struct {
	dual *cache.DualClient
}

// NewRedisCache creates a new RedisCache instance
func NewRedisCache(cfg *cache.Config) (*RedisCache, error) {
	dc, err := cache.NewDualClient(cfg)
	if err != nil {
		return nil, err
	}

	return &RedisCache{dual: dc}, nil
}

// Close shuts down the underlying Redis clients.
func (c *RedisCache) Close() {
	c.dual.Close()
}

// client returns the currently active Redis client.
func (c *RedisCache) client() *redis.Client {
	return c.dual.Client()
}

// GetProfileBlocklists gets blocklists profile subscribes from the cache
func (c *RedisCache) GetProfileBlocklists(ctx context.Context, profileId string) ([]string, error) {
	settingsKey := "settings:" + profileId
	settingsBlocklists := fmt.Sprintf("%s:%s", settingsKey, "blocklists")
	cmd := c.client().LRange(ctx, settingsBlocklists, 0, -1)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetProfileServicesBlocked gets blocked services (service IDs) for a profile.
func (c *RedisCache) GetProfileServicesBlocked(ctx context.Context, profileId string) ([]string, error) {
	settingsKey := "settings:" + profileId
	settingsServicesBlocked := fmt.Sprintf("%s:%s", settingsKey, "services")
	cmd := c.client().LRange(ctx, settingsServicesBlocked, 0, -1)
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

	cmd := c.client().HGetAll(ctx, settings)
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
	cmd := c.client().SIsMember(ctx, blocklistKey, fqdn)
	if err := cmd.Err(); err != nil {
		return false, err
	}
	return cmd.Val(), nil
}

// GetCustomRulesHashes gets list of custom rules set names
func (c *RedisCache) GetCustomRulesHashes(ctx context.Context, profileId string) ([]string, error) {
	customRulesSetKey := fmt.Sprintf("settings:%s:custom_rules", profileId)
	cmd := c.client().SMembers(ctx, customRulesSetKey)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetCustomRulesHash gets custom rules hash
func (c *RedisCache) GetCustomRulesHash(ctx context.Context, hashId string) (map[string]string, error) {
	cmd := c.client().HGetAll(ctx, hashId)
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	return cmd.Val(), nil
}

// GetProfileSettingsBatch fetches privacy, logs, DNSSEC, and advanced settings
// for a profile in a single Redis pipeline round-trip (1 RTT instead of 4).
func (c *RedisCache) GetProfileSettingsBatch(ctx context.Context, profileId string) (*model.ProfileSettings, error) {
	if profileId == "" {
		return nil, fmt.Errorf("profile ID cannot be empty")
	}

	privacyKey := "settings:" + profileId + ":privacy"
	logsKey := "settings:" + profileId + ":logs"
	dnssecKey := "settings:" + profileId + ":security:dnssec"
	advancedKey := "settings:" + profileId + ":advanced"

	pipe := c.client().Pipeline()
	privacyCmd := pipe.HGetAll(ctx, privacyKey)
	logsCmd := pipe.HGetAll(ctx, logsKey)
	dnssecCmd := pipe.HGetAll(ctx, dnssecKey)
	advancedCmd := pipe.HGetAll(ctx, advancedKey)

	_, err := pipe.Exec(ctx)
	// Pipeline Exec returns the error of the first failed command, but
	// individual commands still hold their own results/errors. We only
	// treat a total pipeline failure (e.g. connection lost) as fatal.
	if err != nil && err != redis.Nil {
		// If all commands failed with the same error, it's a connection-level
		// failure (e.g. TCP reset, auth error) — return it so the caller can
		// log the real cause instead of a misleading "profile not found".
		if privacyCmd.Err() == err && logsCmd.Err() == err &&
			dnssecCmd.Err() == err && advancedCmd.Err() == err {
			return nil, fmt.Errorf("redis pipeline failed: %w", err)
		}
		// Otherwise it's a partial failure — handle per-command below.
		log.Warn().Err(err).Msg("Redis pipeline partial error, checking individual commands")
	}

	result := &model.ProfileSettings{}

	// Privacy
	switch {
	case privacyCmd.Err() != nil:
		result.PrivacyErr = privacyCmd.Err()
	case len(privacyCmd.Val()) == 0:
		result.PrivacyErr = fmt.Errorf("No [privacy] settings found for profile %s", profileId)
	default:
		result.Privacy = privacyCmd.Val()
	}

	// Logs
	switch {
	case logsCmd.Err() != nil:
		result.LogsErr = logsCmd.Err()
	case len(logsCmd.Val()) == 0:
		result.LogsErr = fmt.Errorf("No [logs] settings found for profile %s", profileId)
	default:
		result.Logs = logsCmd.Val()
	}

	// DNSSEC
	switch {
	case dnssecCmd.Err() != nil:
		result.DNSSECErr = dnssecCmd.Err()
	case len(dnssecCmd.Val()) == 0:
		result.DNSSECErr = fmt.Errorf("No [security dnssec] settings found for profile %s", profileId)
	default:
		result.DNSSEC = dnssecCmd.Val()
	}

	// Advanced
	switch {
	case advancedCmd.Err() != nil:
		result.AdvancedErr = advancedCmd.Err()
	case len(advancedCmd.Val()) == 0:
		result.AdvancedErr = fmt.Errorf("No [advanced] settings found for profile %s", profileId)
	default:
		result.Advanced = advancedCmd.Val()
	}

	return result, nil
}
