package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivpn/dns/blocklists/internal/downloader"
	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/store"
	"github.com/rs/zerolog"
)

// defaultMetricsPort is the default port for the metrics/health HTTP server.
// It matches the proxy convention (9153); blocklists runs on the DCN host, so
// it does not collide with the proxy/recursor metrics ports on the edge nodes.
const defaultMetricsPort = 9153

// Config represents the application configuration
type Config struct {
	Server   *ServerConfig
	DB       *store.Config
	Cache    *cache.Config
	Updater  *UpdaterConfig
	Sentry   *SentryConfig
	Metrics  *MetricsConfig
	Download downloader.Config
	// LogLevel is the minimum zerolog level emitted (LOG_LEVEL env). Defaults to
	// info so a normal run shows only the startup refresh summary and errors;
	// set to debug to see per-source/per-chunk progress.
	LogLevel zerolog.Level
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
		Metrics:  loadMetricsConfig(),
		Download: loadDownloadConfig(),
		LogLevel: parseLogLevel(os.Getenv("LOG_LEVEL")),
	}, nil
}

// parseLogLevel maps the LOG_LEVEL env value to a zerolog level. Unknown or empty
// values default to info, keeping a normal run's output to the startup summary
// and errors. Mirrors proxy/utils/log.go's ParseZerologLevel.
func parseLogLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "disabled":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel
	}
}

// loadDownloadConfig reads the gentle-downloader tuning knobs from the
// environment. Every field is optional: an unset or invalid value is left zero
// so downloader.New applies its prod-safe default. This keeps downloads gentle
// on source servers out of the box while allowing per-environment tuning.
//
//	DOWNLOAD_TIMEOUT                  per-attempt timeout (e.g. "30s")
//	DOWNLOAD_MAX_CONCURRENCY         max simultaneous downloads across all hosts
//	DOWNLOAD_PER_HOST_MIN_INTERVAL   min spacing between requests to one host (e.g. "2s")
//	DOWNLOAD_RETRY_MAX_ATTEMPTS      total attempts incl. the first (1 = no retry)
//	DOWNLOAD_RETRY_BASE_DELAY        first backoff delay; doubles each attempt
//	DOWNLOAD_RETRY_MAX_DELAY         cap on a single backoff / honoured Retry-After
//	DOWNLOAD_MAX_BODY_SIZE           max accepted response body in bytes
//	DOWNLOAD_USER_AGENT              User-Agent header sent on every request
func loadDownloadConfig() downloader.Config {
	return downloader.Config{
		Timeout:            envDuration("DOWNLOAD_TIMEOUT"),
		MaxConcurrency:     envInt("DOWNLOAD_MAX_CONCURRENCY"),
		PerHostMinInterval: envDuration("DOWNLOAD_PER_HOST_MIN_INTERVAL"),
		MaxAttempts:        envInt("DOWNLOAD_RETRY_MAX_ATTEMPTS"),
		BaseRetryDelay:     envDuration("DOWNLOAD_RETRY_BASE_DELAY"),
		MaxRetryDelay:      envDuration("DOWNLOAD_RETRY_MAX_DELAY"),
		MaxBodySize:        envInt64("DOWNLOAD_MAX_BODY_SIZE"),
		UserAgent:          os.Getenv("DOWNLOAD_USER_AGENT"),
	}
}

// envDuration parses a positive Go duration from key, or returns 0 when unset/invalid.
func envDuration(key string) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return 0
}

// envInt parses a positive int from key, or returns 0 when unset/invalid.
func envInt(key string) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return 0
}

// envInt64 parses a positive int64 from key, or returns 0 when unset/invalid.
func envInt64(key string) int64 {
	if v, err := strconv.ParseInt(os.Getenv(key), 10, 64); err == nil && v > 0 {
		return v
	}
	return 0
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
