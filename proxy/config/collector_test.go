package config

import (
	"os"
	"testing"

	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specRef: proxy-statistics-behaviour.md #Y7
func TestLoadPopName(t *testing.T) {
	host, err := os.Hostname()
	require.NoError(t, err)

	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "explicit value", env: "ams1", want: "ams1"},
		{name: "surrounding whitespace is trimmed", env: "  fra1\t", want: "fra1"},
		{name: "unset falls back to the host name", env: "", want: host},
		{name: "blank falls back to the host name", env: "   ", want: host},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("POP_NAME", tt.env)
			assert.Equal(t, tt.want, loadPopName())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y7
func TestNewCollectorConfig_StatisticsCarriesPopName(t *testing.T) {
	t.Setenv("POP_NAME", "syd1")
	t.Setenv("COLLECTOR_SERVICE_STATISTICS_BATCH_SIZE", "")
	t.Setenv("COLLECTOR_SERVICE_STATISTICS_BATCH_INTERVAL", "")

	cfg, err := NewCollectorConfig(model.TYPE_STATISTICS)
	require.NoError(t, err)
	assert.Equal(t, "syd1", cfg.GetPopName())

	logsCfg, err := NewCollectorConfig(model.TYPE_QUERY_LOGS)
	require.NoError(t, err)
	assert.Empty(t, logsCfg.GetPopName(), "only statistics documents are labelled")
}
