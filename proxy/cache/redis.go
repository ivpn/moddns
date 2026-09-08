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
		// Profile ID goes in a structured (Sentry-denylisted) field, never in
		// the message or error text.
		log.Warn().Str("profile_id", profileId).Msgf("No %s settings found for profile", settingsName)
		return nil, fmt.Errorf("%w: %s", ErrSettingsNotFound, settingsName)
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

// GetProfileSettingsBatch fetches every per-profile input the proxy needs in
// two pipeline round-trips: the settings hashes, lists and the custom-rule
// set first, then the custom-rule hashes named by that set.
func (c *RedisCache) GetProfileSettingsBatch(ctx context.Context, profileId string) (*model.ProfileSettings, error) {
	if profileId == "" {
		return nil, fmt.Errorf("profile ID cannot be empty")
	}

	settingsKey := "settings:" + profileId
	pipe := c.client().Pipeline()
	privacyCmd := pipe.HGetAll(ctx, settingsKey+":privacy")
	logsCmd := pipe.HGetAll(ctx, settingsKey+":logs")
	dnssecCmd := pipe.HGetAll(ctx, settingsKey+":security:dnssec")
	rebindingCmd := pipe.HGetAll(ctx, settingsKey+":security:rebinding_protection")
	advancedCmd := pipe.HGetAll(ctx, settingsKey+":advanced")
	statisticsCmd := pipe.HGetAll(ctx, settingsKey+":statistics")
	blocklistsCmd := pipe.LRange(ctx, settingsKey+":blocklists", 0, -1)
	servicesCmd := pipe.LRange(ctx, settingsKey+":services", 0, -1)
	customRulesCmd := pipe.SMembers(ctx, settingsKey+":custom_rules")

	cmds := []redis.Cmder{privacyCmd, logsCmd, dnssecCmd, rebindingCmd, advancedCmd, statisticsCmd, blocklistsCmd, servicesCmd, customRulesCmd}
	if err := execPipeline(ctx, pipe, cmds); err != nil {
		return nil, err
	}

	result := &model.ProfileSettings{}
	result.Privacy, result.PrivacyErr = hashResult(privacyCmd, "privacy")
	result.Logs, result.LogsErr = hashResult(logsCmd, "logs")
	result.DNSSEC, result.DNSSECErr = hashResult(dnssecCmd, "security dnssec")
	// Missing hash = empty map = opt-in OFF.
	result.RebindingProtection, result.RebindingProtectionErr = hashResult(rebindingCmd, "security rebinding_protection")
	result.Advanced, result.AdvancedErr = hashResult(advancedCmd, "advanced")
	result.Statistics, result.StatisticsErr = hashResult(statisticsCmd, "statistics")
	result.Blocklists, result.BlocklistsErr = blocklistsCmd.Result()
	result.Services, result.ServicesErr = servicesCmd.Result()

	ruleIDs, err := customRulesCmd.Result()
	if err != nil {
		result.CustomRulesErr = err
		return result, nil
	}
	result.CustomRules, result.CustomRulesErr = c.getCustomRules(ctx, ruleIDs)
	return result, nil
}

// getCustomRules loads the named rule hashes in one pipeline. A rule whose
// hash is empty (a set member left behind by a deleted rule) is skipped.
func (c *RedisCache) getCustomRules(ctx context.Context, ruleIDs []string) ([]map[string]string, error) {
	if len(ruleIDs) == 0 {
		return nil, nil
	}
	pipe := c.client().Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ruleIDs))
	cmders := make([]redis.Cmder, len(ruleIDs))
	for i, id := range ruleIDs {
		cmds[i] = pipe.HGetAll(ctx, id)
		cmders[i] = cmds[i]
	}
	if err := execPipeline(ctx, pipe, cmders); err != nil {
		return nil, err
	}
	rules := make([]map[string]string, 0, len(ruleIDs))
	for i, cmd := range cmds {
		rule, err := cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("custom rule %s: %w", ruleIDs[i], err)
		}
		if len(rule) == 0 {
			continue
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// execPipeline runs the pipeline and returns an error for a connection-level
// failure. Only server replies (WRONGTYPE, MOVED, ...) implement redis.Error;
// a dial or transport failure surfaces as a plain error from Exec while the
// individual commands may carry no error at all, so classification goes by
// type, never by comparing per-command errors.
func execPipeline(ctx context.Context, pipe redis.Pipeliner, cmds []redis.Cmder) error {
	_, err := pipe.Exec(ctx)
	if err == nil || err == redis.Nil {
		return nil
	}
	var replyErr redis.Error
	if !errors.As(err, &replyErr) {
		return fmt.Errorf("redis pipeline failed: %w", err)
	}
	// A server reply error belongs to one command and is handled per command.
	log.Warn().Err(err).Int("commands", len(cmds)).Msg("Redis pipeline partial error, checking individual commands")
	return nil
}

// hashResult maps an HGETALL outcome to (value, error): a read error is kept
// as is, an empty hash becomes ErrSettingsNotFound.
func hashResult(cmd *redis.MapStringStringCmd, name string) (map[string]string, error) {
	val, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	if len(val) == 0 {
		return nil, fmt.Errorf("%w: [%s]", ErrSettingsNotFound, name)
	}
	return val, nil
}
