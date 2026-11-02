package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

// specRef: proxy-request-admission-behaviour.md #Q9
func TestLoadMaxGoroutines(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want uint
	}{
		{name: "default when unset", env: "", want: 10_000},
		{name: "override", env: "2500", want: 2500},
		{name: "zero disables the cap", env: "0", want: 0},
		{name: "non-numeric keeps default", env: "unbounded", want: 10_000},
		{name: "negative keeps default", env: "-5", want: 10_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MAX_GOROUTINES", tt.env)
			assert.Equal(t, tt.want, loadMaxGoroutines())
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q13
func TestLoadProfileSettingsCacheSize(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "default when unset", env: "", want: 20_000},
		{name: "override", env: "5000", want: 5000},
		{name: "zero keeps default", env: "0", want: 20_000},
		{name: "negative keeps default", env: "-1", want: 20_000},
		{name: "non-numeric keeps default", env: "lots", want: 20_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PROFILE_SETTINGS_CACHE_SIZE", tt.env)
			assert.Equal(t, tt.want, loadProfileSettingsCacheSize())
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestLoadCacheCommandTimeout(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "default when unset", env: "", want: time.Second},
		{name: "override", env: "250ms", want: 250 * time.Millisecond},
		{name: "zero defers to client defaults", env: "0", want: 0},
		{name: "invalid keeps default", env: "soon", want: time.Second},
		{name: "negative keeps default", env: "-1s", want: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CACHE_COMMAND_TIMEOUT", tt.env)
			assert.Equal(t, tt.want, loadCacheCommandTimeout())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y25
func TestWarnStatisticsSettingsTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		warn bool
	}{
		{name: "default 30s", ttl: 30 * time.Second},
		{name: "1ms as in the E2E stack", ttl: time.Millisecond},
		{name: "zero never expires", ttl: 0, warn: true},
		{name: "above 30s", ttl: 31 * time.Second, warn: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			old := log.Logger
			log.Logger = zerolog.New(&buf)
			t.Cleanup(func() { log.Logger = old })

			WarnStatisticsSettingsTTL(tt.ttl)

			if !tt.warn {
				assert.Empty(t, buf.String())
				return
			}
			assert.Contains(t, buf.String(), `"level":"warn"`)
			assert.Contains(t, buf.String(), "PROFILE_SETTINGS_CACHE_TTL")
		})
	}
}
