package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	libscache "github.com/ivpn/dns/libs/cache"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

// startRedis runs a throwaway Redis and returns a RedisCache bound to it plus
// a raw client for seeding. Skips when no container runtime is available.
func startRedis(t *testing.T) (*RedisCache, *goredis.Client) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()

	redisC, err := redis.Run(ctx, "redis:7")
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(redisC) })

	host, err := redisC.Host(ctx)
	require.NoError(t, err)
	port, err := redisC.MappedPort(ctx, "6379")
	require.NoError(t, err)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	return &RedisCache{dual: libscache.NewSingleClient(rdb)}, rdb
}

// specRef: proxy-request-admission-behaviour.md #S10
func TestGetProfileSettingsBatch_PopulatesListsAndRules(t *testing.T) {
	c, rdb := startRedis(t)
	ctx := context.Background()
	const profileID = "batchprofile1"
	key := "settings:" + profileID

	pipe := rdb.Pipeline()
	pipe.HSet(ctx, key+":privacy", map[string]interface{}{"default_rule": "allow", "blocklists_subdomains_rule": "block"})
	pipe.HSet(ctx, key+":logs", map[string]interface{}{"enabled": "true"})
	pipe.HSet(ctx, key+":statistics", map[string]interface{}{"enabled": "false"})
	pipe.RPush(ctx, key+":blocklists", "bl-ads", "bl-tracking")
	pipe.RPush(ctx, key+":services", "google")
	pipe.HSet(ctx, key+":custom_rule:r1", map[string]interface{}{"value": "ads.example", "action": "block", "syntax": "domain"})
	pipe.HSet(ctx, key+":custom_rule:r2", map[string]interface{}{"value": "cdn.example", "action": "allow", "syntax": "domain"})
	// A set member whose hash no longer exists must be skipped, not fail the batch.
	pipe.SAdd(ctx, key+":custom_rules", key+":custom_rule:r1", key+":custom_rule:r2", key+":custom_rule:gone")
	_, err := pipe.Exec(ctx)
	require.NoError(t, err)

	got, err := c.GetProfileSettingsBatch(ctx, profileID)
	require.NoError(t, err)

	require.NoError(t, got.PrivacyErr)
	assert.Equal(t, "allow", got.Privacy["default_rule"])
	require.NoError(t, got.LogsErr)
	require.NoError(t, got.StatisticsErr)
	assert.Equal(t, "false", got.Statistics["enabled"])

	// Absent hashes are reported as not-found, never as store failures.
	assert.ErrorIs(t, got.DNSSECErr, ErrSettingsNotFound)
	assert.ErrorIs(t, got.AdvancedErr, ErrSettingsNotFound)
	assert.ErrorIs(t, got.RebindingProtectionErr, ErrSettingsNotFound)
	assert.NoError(t, got.StoreError())

	require.NoError(t, got.BlocklistsErr)
	assert.Equal(t, []string{"bl-ads", "bl-tracking"}, got.Blocklists, "subscription order preserved")
	require.NoError(t, got.ServicesErr)
	assert.Equal(t, []string{"google"}, got.Services)

	require.NoError(t, got.CustomRulesErr)
	require.Len(t, got.CustomRules, 2, "stale set member with an empty hash is skipped")
	values := map[string]string{}
	for _, rule := range got.CustomRules {
		values[rule["value"]] = rule["action"]
	}
	assert.Equal(t, map[string]string{"ads.example": "block", "cdn.example": "allow"}, values)
}

// specRef: proxy-request-admission-behaviour.md #S10
func TestGetProfileSettingsBatch_UnknownProfile(t *testing.T) {
	c, _ := startRedis(t)
	ctx := context.Background()

	got, err := c.GetProfileSettingsBatch(ctx, "nosuchprofile")
	require.NoError(t, err, "an empty profile is a per-key outcome, not a batch failure")

	assert.ErrorIs(t, got.PrivacyErr, ErrSettingsNotFound)
	assert.NoError(t, got.StoreError())
	assert.Empty(t, got.Blocklists)
	assert.Empty(t, got.Services)
	assert.Empty(t, got.CustomRules)
	assert.NoError(t, got.BlocklistsErr)
	assert.NoError(t, got.ServicesErr)
	assert.NoError(t, got.CustomRulesErr)
}

// specRef: proxy-request-admission-behaviour.md #S10
func TestGetProfileSettingsBatch_NoRules(t *testing.T) {
	c, rdb := startRedis(t)
	ctx := context.Background()
	const profileID = "batchprofile2"
	require.NoError(t, rdb.HSet(ctx, "settings:"+profileID+":privacy", "default_rule", "block").Err())

	got, err := c.GetProfileSettingsBatch(ctx, profileID)
	require.NoError(t, err)
	require.NoError(t, got.PrivacyErr)
	assert.NoError(t, got.CustomRulesErr)
	assert.Empty(t, got.CustomRules, "no second pipeline is needed for a profile without rules")
}

// specRef: proxy-request-admission-behaviour.md #S10 #S4
func TestGetProfileSettingsBatch_StoreUnreachable(t *testing.T) {
	// A closed port on loopback refuses immediately; no container needed.
	rdb := goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  0,
	})
	t.Cleanup(func() { _ = rdb.Close() })
	c := &RedisCache{dual: libscache.NewSingleClient(rdb)}

	got, err := c.GetProfileSettingsBatch(context.Background(), "anyprofile")

	require.Error(t, err, "a connection-level failure is the batch's error, not a per-key outcome")
	assert.Nil(t, got)
	assert.False(t, errors.Is(err, ErrSettingsNotFound))
}

func TestGetProfileSettingsBatch_EmptyProfileID(t *testing.T) {
	c := &RedisCache{}
	got, err := c.GetProfileSettingsBatch(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, got)
}
