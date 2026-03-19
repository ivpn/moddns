package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	CUSTOM_RULES = "custom_rules"
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

// Incr atomically increments the integer value of a key by one.
// Returns the new value after incrementing, or an error if the operation fails.
func (c *RedisCache) Incr(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	incrCmd := c.client.Incr(ctx, key)
	if err := incrCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to increment value")
		return 0, err
	}
	val := incrCmd.Val()
	log.Trace().Str("key", key).Int64("value", val).Msg("Cache: incremented value")

	// Set expiration only when the key is first created (val == 1)
	if expiration > 0 && val == 1 {
		expireCmd := c.client.Expire(ctx, key, expiration)
		if err := expireCmd.Err(); err != nil {
			log.Err(err).Str("key", key).Msg("Cache: failed to set expiration after increment")
			return val, err
		}
	}

	return val, nil
}

// Set sets a value in the cache with an expiration
func (c *RedisCache) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	setCmd := c.client.Set(ctx, key, value, expiration)
	if err := setCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to set value")
		return err
	}
	log.Trace().Str("key", key).Msg("Cache: set value")
	return nil
}

// Get retrieves a value from the cache
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	getCmd := c.client.Get(ctx, key)
	if err := getCmd.Err(); err != nil {
		log.Trace().Err(err).Str("key", key).Msg("Cache: failed to get value")
		return "", err
	}
	val := getCmd.Val()
	log.Trace().Str("key", key).Msg("Cache: got value")
	return val, nil
}

// Del deletes a value from the cache
func (c *RedisCache) Del(ctx context.Context, key string) error {
	delCmd := c.client.Del(ctx, key)
	if err := delCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to delete value")
		return err
	}
	log.Trace().Str("key", key).Msg("Cache: deleted value")
	return nil
}

// AddBlocklist adds a blocklist to the cache
func (c *RedisCache) AddBlocklist(ctx context.Context, blocklistId string, data []byte) error {
	blocklistName := fmt.Sprintf("blocklist:%s", blocklistId)

	lines := strings.Split(string(data), "\n")
	intCmd := c.client.SAdd(ctx, blocklistName, lines)
	if err := intCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create blocklist")
		return err
	}
	log.Info().
		Str("blocklist_key", blocklistName).
		Msgf("Created blocklist")
	return nil
}

// CreateOrUpdateProfileSettings adds profile settings to the cache
func (c *RedisCache) CreateOrUpdateProfileSettings(ctx context.Context, settings *model.ProfileSettings, rollback bool) error {
	rdp := c.client.Pipeline()
	settingsBlocklist := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "blocklists")
	res := rdp.Del(ctx, settingsBlocklist)
	if err := res.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to remove existing settings blocklists")
		return err
	}
	// associate blocklists to selected settings
	for _, blocklistID := range settings.Privacy.Blocklists {
		// put settings model as blocklist value; this can be replaced
		blocklistsCmd := rdp.RPush(ctx, settingsBlocklist, blocklistID)
		if err := blocklistsCmd.Err(); err != nil {
			log.Err(err).Msg("Cache: failed to create settings blocklist")
			if rollback {
				rdp.Del(ctx, settingsBlocklist)
			}
			return err
		}
		log.Info().Str("settings_blocklist_key", settingsBlocklist).
			Msgf("Created/updated profile settings blocklist")
	}

	// associate blocked services to selected settings
	servicesKey := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "services")
	res = rdp.Del(ctx, servicesKey)
	if err := res.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to remove existing settings services")
		return err
	}
	if settings.Privacy != nil {
		for _, serviceID := range settings.Privacy.Services {
			cmd := rdp.RPush(ctx, servicesKey, serviceID)
			if err := cmd.Err(); err != nil {
				log.Err(err).Msg("Cache: failed to create settings services")
				if rollback {
					rdp.Del(ctx, servicesKey)
				}
				return err
			}
		}
		log.Info().Str("settings_services_key", servicesKey).Msg("Created/updated profile settings services")
	}

	// add logs settings
	logsSettings := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "logs")
	logsCmd := rdp.HSet(ctx, logsSettings, settings.Logs)
	if err := logsCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create logs settings")
		if rollback {
			rdp.Del(ctx, logsSettings)
		}
		return err
	}

	// add statistics settings
	if settings.Statistics == nil {
		settings.Statistics = &model.StatisticsSettings{
			Enabled: false,
		}
	}
	statsSettings := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "statistics")
	statsCmd := rdp.HSet(ctx, statsSettings, settings.Statistics)
	if err := statsCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create statistics settings")
		if rollback {
			log.Warn().Msg("Cache: rolling back statistics settings")
			rdp.Del(ctx, statsSettings)
		}
		return err
	}

	// add security DNSSEC settings
	dnssecSettings := fmt.Sprintf("settings:%s:%s:%s", settings.ProfileId, "security", "dnssec")
	securityDNSSECCmd := rdp.HSet(ctx, dnssecSettings, settings.Security.DNSSECSettings)
	if err := securityDNSSECCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create security DNSSEC settings")
		if rollback {
			log.Warn().Msg("Cache: rolling back security DNSSEC settings")
			rdp.Del(ctx, dnssecSettings)
		}
		return err
	}

	// add advanced settings
	advancedSettings := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "advanced")
	advancedCmd := rdp.HSet(ctx, advancedSettings, settings.Advanced)
	if err := advancedCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create advanced settings")
		if rollback {
			log.Warn().Msg("Cache: rolling back advanced settings")
			rdp.Del(ctx, advancedSettings)
		}
		return err
	}

	// add privacy settings
	privacySettings := fmt.Sprintf("settings:%s:%s", settings.ProfileId, "privacy")
	privacyCmd := rdp.HSet(ctx, privacySettings, settings.Privacy)
	if err := privacyCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create privacy settings")
		if rollback {
			log.Warn().Msg("Cache: rolling back privacy settings")
			rdp.Del(ctx, privacySettings)
		}
		return err
	}

	_, err := rdp.Exec(ctx)
	if err != nil {
		log.Err(err).Msg("Cache: failed to execute pipeline")
		return err
	}

	log.Info().Msg("Created/updated profile settings")

	return nil
}

// AppendServicesBlockedToProfileSettings appends multiple service IDs to the profile's services list in Redis.
func (c *RedisCache) AppendServicesBlockedToProfileSettings(ctx context.Context, profileId string, serviceIds ...string) error {
	key := fmt.Sprintf("settings:%s:%s", profileId, "services")
	if len(serviceIds) == 0 {
		return nil
	}
	cmd := c.client.RPush(ctx, key, serviceIds)
	if err := cmd.Err(); err != nil {
		log.Err(err).Str("key", key).Strs("service_ids", serviceIds).Msg("Cache: failed to append services blocked")
		return err
	}
	log.Info().Str("key", key).Strs("service_ids", serviceIds).Msg("Cache: appended services blocked")
	return nil
}

// RemoveServicesBlockedFromProfileSettings removes multiple service IDs from the profile's services list in Redis.
func (c *RedisCache) RemoveServicesBlockedFromProfileSettings(ctx context.Context, profileId string, serviceIds ...string) error {
	key := fmt.Sprintf("settings:%s:%s", profileId, "services")
	if len(serviceIds) == 0 {
		return nil
	}
	for _, id := range serviceIds {
		cmd := c.client.LRem(ctx, key, 0, id)
		if err := cmd.Err(); err != nil {
			log.Err(err).Str("key", key).Str("service_id", id).Msg("Cache: failed to remove services blocked")
			return err
		}
	}
	log.Info().Str("key", key).Strs("service_ids", serviceIds).Msg("Cache: removed services blocked")
	return nil
}

func (c *RedisCache) AddCustomRule(ctx context.Context, profileId string, customRule *model.CustomRule) error {
	customRulesSetName := fmt.Sprintf("settings:%s:%s", profileId, CUSTOM_RULES)

	// create custom rule hash
	customRuleHash := fmt.Sprintf("settings:%s:custom_rule:%s", profileId, customRule.ID.Hex())
	hashCmd := c.client.HSet(ctx, customRuleHash, customRule)
	if err := hashCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create profile custom rule hash")
		c.client.Del(ctx, customRuleHash) // simple rollback
		return err
	}
	log.Info().Str("custom_rule_hash", customRuleHash).Msg("Created profile custom rule hash")

	// add rule to set (create set if necessary)
	intCmd := c.client.SAdd(ctx, customRulesSetName, customRuleHash)
	if err := intCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to create/update profile custom rules set")
		return err
	}
	log.Info().Str("custom_rules_set", customRulesSetName).Msg("Created/updated profile custom rules set")
	return nil
}

// AddSubscription sets a simple presence key for an account's subscription with expiration
// Key format: subscription:<subscriptionId>
func (c *RedisCache) AddSubscription(ctx context.Context, subscriptionId, activeUntil string, expiresIn time.Duration) error {
	key := fmt.Sprintf("subscription:%s", subscriptionId)
	setCmd := c.client.Set(ctx, key, activeUntil, expiresIn)
	if err := setCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to add subscription key")
		return err
	}
	log.Info().Str("key", key).Dur("expires_in", expiresIn).Msg("Cache: added subscription key")
	return nil
}

// GetSubscription retrieves the activeUntil value for a subscription from the cache
func (c *RedisCache) GetSubscription(ctx context.Context, subscriptionId string) (string, error) {
	key := fmt.Sprintf("subscription:%s", subscriptionId)
	getCmd := c.client.Get(ctx, key)
	if err := getCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to get subscription key")
		return "", err
	}
	log.Info().Str("key", key).Msg("Cache: retrieved subscription key")
	return getCmd.Val(), nil
}

func (c *RedisCache) RemoveSubscription(ctx context.Context, subscriptionId string) error {
	key := fmt.Sprintf("subscription:%s", subscriptionId)
	delCmd := c.client.Del(ctx, key)
	if err := delCmd.Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to remove subscription key")
		return err
	}
	log.Info().Str("key", key).Msg("Cache: removed subscription key")
	return nil
}

func (c *RedisCache) RemoveCustomRule(ctx context.Context, profileId, customRuleId string) error {
	customRuleHash := fmt.Sprintf("settings:%s:custom_rule:%s", profileId, customRuleId)
	hashCmd := c.client.Del(ctx, customRuleHash)
	if err := hashCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to remove profile custom rule hash")
		return err
	}
	log.Info().Str("custom_rule_hash", customRuleHash).Msg("Removed profile custom rule hash")

	customRulesSetName := fmt.Sprintf("settings:%s:%s", profileId, CUSTOM_RULES)
	intCmd := c.client.SRem(ctx, customRulesSetName, customRuleHash)
	if err := intCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to remove profile custom rule from set")
		return err
	}
	return nil
}

// DeleteProfileSettings deletes profile settings from the cache
func (c *RedisCache) DeleteProfileSettings(ctx context.Context, profileId string) error {
	settingsBlocklist := fmt.Sprintf("settings:%s:%s", profileId, "blocklists")
	blocklistsCmd := c.client.Del(ctx, settingsBlocklist)
	if err := blocklistsCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile settings blocklists")
		return err
	}
	log.Info().Str("settings_blocklist_key", settingsBlocklist).
		Msg("Cache: Deleted profile settings blocklist")

	servicesKey := fmt.Sprintf("settings:%s:%s", profileId, "services")
	servicesCmd := c.client.Del(ctx, servicesKey)
	if err := servicesCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile settings services")
		return err
	}
	log.Info().Str("settings_services_key", servicesKey).
		Msg("Cache: Deleted profile settings services")

	// delete logs settings
	logsSettings := fmt.Sprintf("settings:%s:%s", profileId, "logs")
	logsCmd := c.client.Del(ctx, logsSettings)
	if err := logsCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile logs settings")
		return err
	}
	log.Info().Str("logs_settings_key", logsSettings).Msg("Cache: deleted profile logs settings")

	// delete privacy settings
	privacySettings := fmt.Sprintf("settings:%s:%s", profileId, "privacy")
	privacyCmd := c.client.Del(ctx, privacySettings)
	if err := privacyCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile privacy settings")
		return err
	}
	// delete advanced settings
	advancedSettings := fmt.Sprintf("settings:%s:%s", profileId, "advanced")
	advancedCmd := c.client.Del(ctx, advancedSettings)
	if err := advancedCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile advanced settings")
		return err
	}

	// delete security DNSSEC settings
	dnssecSettings := fmt.Sprintf("settings:%s:%s:%s", profileId, "security", "dnssec")
	dnssecCmd := c.client.Del(ctx, dnssecSettings)
	if err := dnssecCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile security settings")
		return err
	}

	customRulesSetName := fmt.Sprintf("settings:%s:%s", profileId, CUSTOM_RULES)

	// get all custom rule hashes
	customRulesListCmd := c.client.SMembers(ctx, customRulesSetName)
	if err := customRulesListCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to get profile custom rules")
		return err
	}

	// remove all custom rule hashes
	for _, customRuleHash := range customRulesListCmd.Val() {
		hashCmd := c.client.Del(ctx, customRuleHash)
		if err := hashCmd.Err(); err != nil {
			log.Err(err).Msg("Cache: failed to delete profile custom rule hash")
			return err
		}
	}

	// remove custom rules set
	customRulesCmd := c.client.Del(ctx, customRulesSetName)
	if err := customRulesCmd.Err(); err != nil {
		log.Err(err).Msg("Cache: failed to delete profile custom rules")
		return err
	}

	return nil
}

// AppendBlocklistsToProfileSettings appends multiple blocklist IDs to the profile's blocklists list in Redis.
// This operation is concurrency-safe because RPUSH is atomic in Redis.
func (c *RedisCache) AppendBlocklistsToProfileSettings(ctx context.Context, profileId string, blocklistIds ...string) error {
	if len(blocklistIds) == 0 {
		return nil
	}
	settingsBlocklist := fmt.Sprintf("settings:%s:%s", profileId, "blocklists")
	cmd := c.client.RPush(ctx, settingsBlocklist, blocklistIds)
	if err := cmd.Err(); err != nil {
		log.Err(err).
			Strs("blocklist_ids", blocklistIds).
			Msg("Cache: failed to append blocklists to profile settings")
		return err
	}
	log.Info().
		Strs("blocklist_ids", blocklistIds).
		Msg("Cache: appended blocklists to profile settings")
	return nil
}

// RemoveBlocklistsFromProfileSettings removes multiple blocklist IDs from the profile's blocklists list in Redis.
// This operation is concurrency-safe because LREM is atomic in Redis.
func (c *RedisCache) RemoveBlocklistsFromProfileSettings(ctx context.Context, profileId string, blocklistIds ...string) error {
	if len(blocklistIds) == 0 {
		return nil
	}
	settingsBlocklist := fmt.Sprintf("settings:%s:%s", profileId, "blocklists")
	for _, blocklistId := range blocklistIds {
		cmd := c.client.LRem(ctx, settingsBlocklist, 0, blocklistId)
		if err := cmd.Err(); err != nil {
			log.Err(err).
				Str("blocklist_id", blocklistId).
				Msg("Cache: failed to remove blocklist from profile settings")
			return err
		}
		log.Info().
			Str("blocklist_id", blocklistId).
			Msg("Cache: removed blocklist from profile settings")
	}
	return nil
}
