package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ALLOWED_DOMAINS", "test.com")
	t.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53")
}

// specRef: api-endpoint-behaviour.md J8 — defaults derive the schedule; the delay override is opt-in.
func TestStatisticsPurgeConfig(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := New()
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, cfg.Service.ProxySettingsCacheTTL)
	require.Zero(t, cfg.Service.StatisticsPurgeDelay)

	t.Setenv("PROXY_SETTINGS_CACHE_TTL", "1m")
	t.Setenv("STATISTICS_PURGE_DELAY", "5s")
	cfg, err = New()
	require.NoError(t, err)
	require.Equal(t, time.Minute, cfg.Service.ProxySettingsCacheTTL)
	require.Equal(t, 5*time.Second, cfg.Service.StatisticsPurgeDelay)

	t.Setenv("PROXY_SETTINGS_CACHE_TTL", "soon")
	_, err = New()
	require.Error(t, err)
}
