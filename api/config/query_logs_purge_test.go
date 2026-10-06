package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J14 — the unconsented query-logs sweep cadence defaults to one hour and can be changed.
func TestQueryLogsPurgeIntervalConfig(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := New()
	require.NoError(t, err)
	require.Equal(t, time.Hour, cfg.Service.QueryLogsPurgeInterval)

	t.Setenv("QUERY_LOGS_PURGE_INTERVAL", "20m")
	cfg, err = New()
	require.NoError(t, err)
	require.Equal(t, 20*time.Minute, cfg.Service.QueryLogsPurgeInterval)
}

// specRef: api-endpoint-behaviour.md J14 — the interval must be a positive duration.
func TestQueryLogsPurgeIntervalConfig_RejectsInvalid(t *testing.T) {
	for _, v := range []string{"soon", "0s", "-1h", "0"} {
		t.Run(v, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("QUERY_LOGS_PURGE_INTERVAL", v)
			_, err := New()
			require.Error(t, err)
			require.Contains(t, err.Error(), "QUERY_LOGS_PURGE_INTERVAL")
		})
	}
}
