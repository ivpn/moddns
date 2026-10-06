package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/api/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md G7 — statistics.enabled_at never reaches the proxy's Redis hash.
func TestCreateOrUpdateProfileSettings_StatisticsEnabledAtNotWritten(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	settings := model.NewSettings()
	settings.ProfileId = "profile123"
	settings.Statistics.Enabled = true
	at := time.Now()
	settings.Statistics.EnabledAt = &at

	require.NoError(t, NewRedisCacheFromClient(client).CreateOrUpdateProfileSettings(context.Background(), settings, false))

	fields := mr.HGet("settings:profile123:statistics", "enabled")
	require.Equal(t, "1", fields)
	all, err := client.HGetAll(context.Background(), "settings:profile123:statistics").Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"enabled": "1"}, all)
}

// specRef: api-endpoint-behaviour.md G7 — the proxy reads exactly {enabled: "1"|"0"} from the statistics hash.
func TestStatisticsHashEncoding(t *testing.T) {
	for _, tc := range []struct {
		enabled bool
		want    string
	}{{true, "1"}, {false, "0"}} {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })

		settings := model.NewSettings()
		settings.ProfileId = "profile123"
		settings.Statistics.Enabled = tc.enabled
		require.NoError(t, NewRedisCacheFromClient(client).CreateOrUpdateProfileSettings(context.Background(), settings, false))

		all, err := client.HGetAll(context.Background(), "settings:profile123:statistics").Result()
		require.NoError(t, err)
		require.Equal(t, map[string]string{"enabled": tc.want}, all)
	}
}
