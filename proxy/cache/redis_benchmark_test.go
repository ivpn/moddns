package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	toxiclient "github.com/Shopify/toxiproxy/v2/client"
	"github.com/ivpn/dns/proxy/model"
	gocache "github.com/patrickmn/go-cache"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/modules/toxiproxy"
	"github.com/testcontainers/testcontainers-go/network"
)

const benchProfileID = "bench-profile-001"

// benchEnv holds the test infrastructure for cache benchmarks.
type benchEnv struct {
	cache   *RedisCache
	cleanup func()
}

// seedTestData populates Redis with profile settings matching what HandleBefore reads.
// Connects directly to Redis (bypassing toxiproxy) for fast, latency-free seeding.
func seedTestData(ctx context.Context, t testing.TB, redisAddr string) {
	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	defer rdb.Close()

	pipe := rdb.Pipeline()
	pipe.HSet(ctx, "settings:"+benchProfileID+":privacy", map[string]interface{}{
		"enabled": "true",
	})
	pipe.HSet(ctx, "settings:"+benchProfileID+":logs", map[string]interface{}{
		"enabled":         "true",
		"log_domains":     "true",
		"log_clients_ips": "false",
	})
	pipe.HSet(ctx, "settings:"+benchProfileID+":security:dnssec", map[string]interface{}{
		"enabled":     "true",
		"send_do_bit": "true",
	})
	pipe.HSet(ctx, "settings:"+benchProfileID+":advanced", map[string]interface{}{
		"recursor": "default",
	})
	_, err := pipe.Exec(ctx)
	require.NoError(t, err)
}

// getSettingsSequential mimics the old HandleBefore code path:
// 4 sequential Redis round-trips for profile settings.
func getSettingsSequential(ctx context.Context, c *RedisCache, profileId string) *model.ProfileSettings {
	result := &model.ProfileSettings{}
	result.Privacy, result.PrivacyErr = c.GetProfilePrivacySettings(ctx, profileId)
	result.Logs, result.LogsErr = c.GetProfileLogsSettings(ctx, profileId)
	result.DNSSEC, result.DNSSECErr = c.GetProfileDNSSECSettings(ctx, profileId)
	result.Advanced, result.AdvancedErr = c.GetProfileAdvancedSettings(ctx, profileId)
	return result
}

// setupBenchEnv starts Redis + Toxiproxy containers on a shared Docker network,
// seeds test data, and returns a RedisCache routed through the latency proxy.
func setupBenchEnv(t testing.TB, latencyMs int) *benchEnv {
	ctx := context.Background()

	// Shared Docker network so toxiproxy can reach Redis by hostname.
	nw, err := network.New(ctx)
	require.NoError(t, err)

	// Start Redis.
	redisC, err := redis.Run(ctx, "redis:7",
		network.WithNetwork([]string{"redis"}, nw),
	)
	require.NoError(t, err)

	// Get Redis direct address for seeding (bypass toxiproxy).
	redisHost, err := redisC.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisC.MappedPort(ctx, "6379")
	require.NoError(t, err)
	directAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

	seedTestData(ctx, t, directAddr)

	// Start Toxiproxy. Expose port 8666 for the Redis proxy.
	toxiC, err := toxiproxy.Run(ctx, "ghcr.io/shopify/toxiproxy:2.12.0",
		network.WithNetwork([]string{"toxiproxy"}, nw),
		testcontainers.WithExposedPorts("8666/tcp"),
	)
	require.NoError(t, err)

	// Create toxiproxy proxy: 0.0.0.0:8666 → redis:6379 via Docker network.
	toxiURI, err := toxiC.URI(ctx)
	require.NoError(t, err)
	toxiClient := toxiclient.NewClient(toxiURI)

	p, err := toxiClient.CreateProxy("redis", "0.0.0.0:8666", "redis:6379")
	require.NoError(t, err)

	if latencyMs > 0 {
		_, err = p.AddToxic("latency", "latency", "upstream", 1.0,
			toxiclient.Attributes{"latency": latencyMs, "jitter": 0},
		)
		require.NoError(t, err)
	}

	// Get the external (host-mapped) address for the proxy port.
	toxiHost, err := toxiC.Host(ctx)
	require.NoError(t, err)
	toxiPort, err := toxiC.MappedPort(ctx, "8666")
	require.NoError(t, err)
	proxyAddr := fmt.Sprintf("%s:%s", toxiHost, toxiPort.Port())

	// Build a RedisCache that goes through the latency proxy.
	rdb := goredis.NewClient(&goredis.Options{Addr: proxyAddr})
	rc := &RedisCache{client: rdb}

	cleanup := func() {
		rdb.Close()
		testcontainers.TerminateContainer(toxiC)
		testcontainers.TerminateContainer(redisC)
		nw.Remove(ctx)
	}

	return &benchEnv{cache: rc, cleanup: cleanup}
}

// BenchmarkGetProfileSettings compares 4-sequential-calls vs 1-pipeline-call
// at various simulated network latencies (via toxiproxy).
//
// Run: go test -bench=BenchmarkGetProfileSettings -benchtime=10s -count=3 ./cache/
func BenchmarkGetProfileSettings(b *testing.B) {
	// 0ms  = baseline (localhost-like, same Docker network)
	// 5ms  = future same-network Redis deployment
	// 20ms = moderate latency
	// 50ms = current remote Redis scenario
	latencies := []int{0, 5, 20, 50}

	for _, latency := range latencies {
		b.Run(fmt.Sprintf("latency_%dms", latency), func(b *testing.B) {
			env := setupBenchEnv(b, latency)
			b.Cleanup(env.cleanup)

			ctx := context.Background()

			b.Run("sequential", func(b *testing.B) {
				// Verify correctness before timing.
				ps := getSettingsSequential(ctx, env.cache, benchProfileID)
				require.Nil(b, ps.PrivacyErr)
				require.NotNil(b, ps.Privacy)
				require.Nil(b, ps.LogsErr)
				require.NotNil(b, ps.Logs)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					getSettingsSequential(ctx, env.cache, benchProfileID)
				}
			})

			b.Run("pipeline", func(b *testing.B) {
				// Verify correctness before timing.
				ps, err := env.cache.GetProfileSettingsBatch(ctx, benchProfileID)
				require.NoError(b, err)
				require.Nil(b, ps.PrivacyErr)
				require.NotNil(b, ps.Privacy)
				require.Nil(b, ps.LogsErr)
				require.NotNil(b, ps.Logs)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					env.cache.GetProfileSettingsBatch(ctx, benchProfileID)
				}
			})

			b.Run("cached", func(b *testing.B) {
				// Pre-warm the in-memory cache.
				ps, err := env.cache.GetProfileSettingsBatch(ctx, benchProfileID)
				require.NoError(b, err)
				require.Nil(b, ps.PrivacyErr)

				localCache := gocache.New(30*time.Second, time.Minute)
				localCache.Set(benchProfileID, ps, gocache.DefaultExpiration)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					localCache.Get(benchProfileID)
				}
			})
		})
	}
}
