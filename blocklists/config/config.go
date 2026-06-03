package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/store"
)

// defaultMetricsPort is the default port for the metrics/health HTTP server.
// It matches the proxy convention (9153); blocklists runs on the DCN host, so
// it does not collide with the proxy/recursor metrics ports on the edge nodes.
const defaultMetricsPort = 9153

// Config represents the application configuration
type Config struct {
	Server  *ServerConfig
	DB      *store.Config
	Cache   *cache.Config
	Updater *UpdaterConfig
	Sentry  *SentryConfig
	Metrics *MetricsConfig
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Name string
}

// MetricsConfig represents the metrics/health HTTP server configuration.
type MetricsConfig struct {
	// Port is the listen port for /metrics and /health/*. 0 disables the server.
	Port int
}

// UpdaterConfig represents the updater configuration
type UpdaterConfig struct {
	Type       string
	SourcesDir string
	// ShrinkThreshold is the maximum fraction by which a blocklist may shrink
	// between updates before the swap is rejected. e.g. 0.5 rejects an update
	// whose validated domain count drops more than 50% vs the previous run.
	ShrinkThreshold float64
}

// defaultShrinkThreshold is the default value for UpdaterConfig.ShrinkThreshold.
const defaultShrinkThreshold = 0.5

// SentryConfig represents the Sentry configuration
type SentryConfig struct {
	DSN         string
	Environment string
	Release     string
}

// New creates a new Config instance
func New() (*Config, error) {
	updaterType := os.Getenv("UPDATER_TYPE")
	if updaterType == "" {
		updaterType = updater.UpdaterTypeStandard
	}

	cacheAddrs := strings.Split(os.Getenv("CACHE_ADDRESSES"), ",")

	return &Config{
		Server: &ServerConfig{
			Name: os.Getenv("SERVER_NAME"),
		},
		DB: &store.Config{
			DbURI:    os.Getenv("DB_URI"),
			Name:     os.Getenv("DB_NAME"),
			Username: os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
			AuthSource: func() string {
				v := os.Getenv("DB_AUTH_SOURCE")
				if v == "" {
					return "dns"
				}
				return v
			}(),
			MigrationsSource: os.Getenv("DB_MIGRATIONS_SOURCE"),
		},
		Cache: &cache.Config{
			Address:               os.Getenv("CACHE_ADDRESS"),
			FailoverAddresses:     cacheAddrs,
			Username:              os.Getenv("CACHE_USERNAME"),
			Password:              os.Getenv("CACHE_PASSWORD"),
			FailoverPassword:      os.Getenv("CACHE_FAILOVER_PASSWORD"),
			FailoverUsername:      os.Getenv("CACHE_FAILOVER_USERNAME"),
			MasterName:            os.Getenv("CACHE_MASTER_NAME"),
			TLSEnabled:            os.Getenv("CACHE_TLS_ENABLED") == "true",
			CertFile:              os.Getenv("CACHE_CERT_FILE"),
			KeyFile:               os.Getenv("CACHE_KEY_FILE"),
			CACertFile:            os.Getenv("CACHE_CA_CERT_FILE"),
			TLSInsecureSkipVerify: os.Getenv("CACHE_TLS_INSECURE_SKIP_VERIFY") == "true",
		},
		Updater: &UpdaterConfig{
			Type:            updaterType,
			SourcesDir:      os.Getenv("UPDATER_SOURCES_DIR"),
			ShrinkThreshold: loadShrinkThreshold(),
		},
		Sentry: &SentryConfig{
			DSN:         os.Getenv("SENTRY_DSN"),
			Environment: os.Getenv("SENTRY_ENVIRONMENT"),
			Release:     os.Getenv("SENTRY_RELEASE"),
		},
		Metrics: loadMetricsConfig(),
	}, nil
}

// loadShrinkThreshold reads BLOCKLIST_SHRINK_THRESHOLD (a fraction in [0,1]).
// Invalid or out-of-range values fall back to defaultShrinkThreshold.
func loadShrinkThreshold() float64 {
	v, err := strconv.ParseFloat(os.Getenv("BLOCKLIST_SHRINK_THRESHOLD"), 64)
	if err != nil || v < 0 || v > 1 {
		return defaultShrinkThreshold
	}
	return v
}

// loadMetricsConfig reads the metrics/health server configuration from the
// environment. METRICS_PORT defaults to defaultMetricsPort; set it to 0 to
// disable the server entirely.
func loadMetricsConfig() *MetricsConfig {
	cfg := &MetricsConfig{Port: defaultMetricsPort}
	if v, err := strconv.Atoi(os.Getenv("METRICS_PORT")); err == nil && v >= 0 {
		cfg.Port = v
	}
	return cfg
}
