package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J14 — the query-logs reconcile cadence defaults to one hour and can be changed.
func TestQueryLogsReconcileIntervalConfig(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := New()
	require.NoError(t, err)
	require.Equal(t, time.Hour, cfg.Service.QueryLogsReconcileInterval)

	t.Setenv("QUERY_LOGS_RECONCILE_INTERVAL", "20m")
	cfg, err = New()
	require.NoError(t, err)
	require.Equal(t, 20*time.Minute, cfg.Service.QueryLogsReconcileInterval)
}

// specRef: api-endpoint-behaviour.md J14 — the interval must be a positive duration.
func TestQueryLogsReconcileIntervalConfig_RejectsInvalid(t *testing.T) {
	for _, v := range []string{"soon", "0s", "-1h", "0"} {
		t.Run(v, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("QUERY_LOGS_RECONCILE_INTERVAL", v)
			_, err := New()
			require.Error(t, err)
			require.Contains(t, err.Error(), "QUERY_LOGS_RECONCILE_INTERVAL")
		})
	}
}
