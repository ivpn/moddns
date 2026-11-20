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

// specRef: proxy-request-admission-behaviour.md #Q10 #Q16
func TestLoadDoHConfig(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		http3   string
		want    DoHConfig
		wantErr bool
	}{
		{name: "DoH off", port: "", http3: "", want: DoHConfig{HTTP3: defaultDoH3Limits}},
		{name: "DoH without HTTP/3", port: "443", http3: "", want: DoHConfig{ListenAddr: 443, HTTP3: defaultDoH3Limits}},
		{name: "HTTP/3 enabled", port: "443", http3: "true", want: DoHConfig{ListenAddr: 443, HTTP3Enabled: true, HTTP3: defaultDoH3Limits}},
		{name: "HTTP/3 enabled with 1", port: "443", http3: "1", want: DoHConfig{ListenAddr: 443, HTTP3Enabled: true, HTTP3: defaultDoH3Limits}},
		{name: "unrecognised value keeps HTTP/3 off", port: "443", http3: "yes", want: DoHConfig{ListenAddr: 443, HTTP3: defaultDoH3Limits}},
		{name: "HTTP/3 without DoH port", port: "", http3: "true", wantErr: true},
		{name: "non-numeric port", port: "https", http3: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DOH_LISTEN_ADDR", tt.port)
			t.Setenv("DOH_HTTP3_ENABLED", tt.http3)

			got, err := loadDoHConfig()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, *got)
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestLoadDoH3Limits(t *testing.T) {
	tests := []struct {
		name    string
		conns   string
		perIP   string
		streams string
		want    DoH3Limits
	}{
		{name: "defaults", want: defaultDoH3Limits},
		{name: "overrides", conns: "100", perIP: "4", streams: "8", want: DoH3Limits{MaxConns: 100, MaxConnsPerIP: 4, MaxStreams: 8}},
		{name: "zero keeps defaults", conns: "0", perIP: "0", streams: "0", want: defaultDoH3Limits},
		{name: "negative keeps defaults", conns: "-1", perIP: "-1", streams: "-1", want: defaultDoH3Limits},
		{name: "non-numeric keeps defaults", conns: "many", perIP: "few", streams: "lots", want: defaultDoH3Limits},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DOH_HTTP3_MAX_CONNECTIONS", tt.conns)
			t.Setenv("DOH_HTTP3_MAX_CONNECTIONS_PER_IP", tt.perIP)
			t.Setenv("DOH_HTTP3_MAX_STREAMS", tt.streams)
			assert.Equal(t, tt.want, loadDoH3Limits())
		})
	}
}
