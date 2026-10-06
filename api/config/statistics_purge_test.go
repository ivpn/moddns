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

// specRef: api-endpoint-behaviour.md J9 — the reconcile cadence defaults to one hour and can be changed.
func TestStatisticsReconcileIntervalConfig(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := New()
	require.NoError(t, err)
	require.Equal(t, time.Hour, cfg.Service.StatisticsReconcileInterval)

	t.Setenv("STATISTICS_RECONCILE_INTERVAL", "15m")
	cfg, err = New()
	require.NoError(t, err)
	require.Equal(t, 15*time.Minute, cfg.Service.StatisticsReconcileInterval)
}

// specRef: api-endpoint-behaviour.md J9 — the interval must be a positive duration.
func TestStatisticsReconcileIntervalConfig_RejectsInvalid(t *testing.T) {
	for _, v := range []string{"soon", "0s", "-1h", "0"} {
		t.Run(v, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("STATISTICS_RECONCILE_INTERVAL", v)
			_, err := New()
			require.Error(t, err)
			require.Contains(t, err.Error(), "STATISTICS_RECONCILE_INTERVAL")
		})
	}
}
