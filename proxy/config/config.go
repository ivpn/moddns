package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/proxy/model"
)

// Config represents the application configuration
type Config struct {
	Server              *ServerConfig
	Services            *ServicesConfig
	Cache               *cache.Config
	CollectorQueryLogs  CollectorConfig
	CollectorStatistics CollectorConfig
	Emitter             *EmitterConfig
	Upstream            *UpstreamConfig
	PlainDNS            *PlainDNSConfig
	TLS                 *TLSConfig
	DoH                 *DoHConfig
	DoT                 *DoTConfig
	DoQ                 *DoQConfig
	Sentry              *SentryConfig
	Log                 *LogConfig
	TrustedProxies      []string
	ProfileIDMinLength  int
}

// LogConfig represents the logging configuration
type LogConfig struct {
	AdGuardLogLevel string
	ZerologLevel    string
}

// SentryConfig represents the Sentry configuration
type SentryConfig struct {
	DSN         string
	Environment string
	Release     string
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Name           string
	DnsCheckDomain string
	DnsCheckPort   string
}

// ServicesConfig configures ASN-based services blocking.
type ServicesConfig struct {
	CatalogPath        string
	CatalogReloadEvery time.Duration
	GeoIPASNDBPath     string
}

// UpstreamConfig represents the upstream configuration
type UpstreamConfig struct {
	Upstreams map[string]string
	Default   string
}

// TLSConfig represents the TLS configuration
type TLSConfig struct {
	CertPath string
	KeyPath  string
}

// PlainDNSConfig represents the plain DNS configuration
type PlainDNSConfig struct {
	UDPListenAddr int
	TCPListenAddr int
}

// DoHConfig represents the DNS-over-HTTPS configuration
type DoHConfig struct {
	ListenAddr int
}

// DoTConfig represents the DNS-over-TLS configuration
type DoTConfig struct {
	ListenAddr int
}

// DoQConfig represents the DNS-over-QUIC configuration
type DoQConfig struct {
	ListenAddr int
}

// GetEnvInt returns the integer value of an environment variable
func GetEnvInt(env string) (int, error) {
	var envValInt int
	envValStr := os.Getenv(env)
	if envValStr == "" {
		envValInt = 0
	} else {
		var err error
		envValInt, err = strconv.Atoi(envValStr)
		if err != nil {
			return 0, err
		}
	}
	return envValInt, nil
}

// LoadUpstreamConfig loads upstream DNS server configurations from environment variable
func LoadUpstreamConfig(upstreamsEnv, defaultRecursorEnv string) (*UpstreamConfig, error) {
	// Get the upstream configuration string
	upstreamStr := os.Getenv(upstreamsEnv)
	if upstreamStr == "" {
		return nil, fmt.Errorf("%s not found in environment", upstreamsEnv)
	}

	defaultRecursor := os.Getenv(defaultRecursorEnv)
	if upstreamStr == "" {
		return nil, fmt.Errorf("%s not found in environment", defaultRecursorEnv)
	}

	// Parse the configuration
	upstreams := make(map[string]string)
	pairs := strings.Split(upstreamStr, ",")

	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid upstream configuration format: %s", pair)
		}
		upstreams[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}

	if len(upstreams) == 0 {
		return nil, fmt.Errorf("no upstream configurations found")
	}

	return &UpstreamConfig{
		Upstreams: upstreams,
		Default:   defaultRecursor,
	}, nil
}

// New creates a new Config instance
func New() (*Config, error) {
	trustedProxies := []string{"10.5.0.0/16"}
	if env := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); env != "" {
		candidates := strings.Split(env, ",")
		trustedProxies = trustedProxies[:0]
		for _, c := range candidates {
			if cidr := strings.TrimSpace(c); cidr != "" {
				trustedProxies = append(trustedProxies, cidr)
			}
		}
		if len(trustedProxies) == 0 {
			trustedProxies = []string{"10.5.0.0/16"}
		}
	}

	// Profile ID min length (default 10)
	profileIdMinLen := 10
	if v := os.Getenv("PROFILE_ID_MIN_LENGTH"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed < 64 { // bound
			profileIdMinLen = parsed
		}
	}
	udpListenAddr, err := GetEnvInt("PLAIN_DNS_UDP_LISTEN_ADDR")
	if err != nil {
		return nil, err
	}

	tcpListenAddr, err := GetEnvInt("PLAIN_DNS_TCP_LISTEN_ADDR")
	if err != nil {
		return nil, err
	}

	dohListenAddr, err := GetEnvInt("DOH_LISTEN_ADDR")
	if err != nil {
		return nil, err
	}

	dotListenAddr, err := GetEnvInt("DOT_LISTEN_ADDR")
	if err != nil {
		return nil, err
	}

	doqListenAddr, err := GetEnvInt("DOQ_LISTEN_ADDR")
	if err != nil {
		return nil, err
	}

	upstreamConfig, err := LoadUpstreamConfig("DNS_UPSTREAMS", "DNS_UPSTREAMS_DEFAULT")
	if err != nil {
		return nil, err
	}

	collectorQueryLogsCfg, err := NewCollectorConfig(model.TYPE_QUERY_LOGS)
	if err != nil {
		return nil, err
	}
	collectorStatisticsCfg, err := NewCollectorConfig(model.TYPE_STATISTICS)
	if err != nil {
		return nil, err
	}

	sinkCfg, err := NewSinkConfig(os.Getenv("EMITTER_SINK_TYPE"))
	if err != nil {
		return nil, err
	}

	dnsCheckDomain := os.Getenv("DNS_CHECK_DOMAIN")
	if len(dnsCheckDomain) == 0 {
		dnsCheckDomain = "test.moddns.net"
	}

	// Get AdGuard log level (default to "info" if not set or invalid)
	adguardLogLevel := strings.ToLower(os.Getenv("LOG_LEVEL_ADGUARD"))

	// Get Zerolog log level (default to "info" if not set or invalid)
	zerologLevel := strings.ToLower(os.Getenv("LOG_LEVEL_PROXY"))

	servicesCatalogPath := strings.TrimSpace(os.Getenv("SERVICES_CATALOG_PATH"))
	if servicesCatalogPath == "" {
		servicesCatalogPath = "/opt/services/catalog.yml"
	}
	servicesCatalogReloadEveryStr := strings.TrimSpace(os.Getenv("SERVICES_CATALOG_RELOAD"))
	if servicesCatalogReloadEveryStr == "" {
		servicesCatalogReloadEveryStr = "5m"
	}
	servicesCatalogReloadEvery, err := time.ParseDuration(servicesCatalogReloadEveryStr)
	if err != nil {
		return nil, err
	}

	geoIPASNDBPath := strings.TrimSpace(os.Getenv("GEOIP_DB_ASN_FILE"))

	cacheAddrs := strings.Split(os.Getenv("CACHE_ADDRESSES"), ",")

	return &Config{
		Server: &ServerConfig{
			Name:           os.Getenv("SERVER_NAME"),
			DnsCheckDomain: dnsCheckDomain,
			DnsCheckPort:   os.Getenv("DNS_CHECK_PORT"),
		},
		Services: &ServicesConfig{
			CatalogPath:        servicesCatalogPath,
			CatalogReloadEvery: servicesCatalogReloadEvery,
			GeoIPASNDBPath:     geoIPASNDBPath,
		},
		TrustedProxies:     trustedProxies,
		ProfileIDMinLength: profileIdMinLen,
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
		CollectorQueryLogs:  collectorQueryLogsCfg,
		CollectorStatistics: collectorStatisticsCfg,
		Emitter: &EmitterConfig{
			Type:       os.Getenv("EMITTER_SINK_TYPE"),
			SinkConfig: sinkCfg,
		},
		Upstream: upstreamConfig,
		PlainDNS: &PlainDNSConfig{
			UDPListenAddr: udpListenAddr,
			TCPListenAddr: tcpListenAddr,
		},
		TLS: &TLSConfig{
			CertPath: os.Getenv("TLS_CERT_PATH"),
			KeyPath:  os.Getenv("TLS_KEY_PATH"),
		},
		DoH: &DoHConfig{
			ListenAddr: dohListenAddr,
		},
		DoT: &DoTConfig{
			ListenAddr: dotListenAddr,
		},
		DoQ: &DoQConfig{
			ListenAddr: doqListenAddr,
		},
		Sentry: &SentryConfig{
			DSN:         os.Getenv("SENTRY_DSN"),
			Environment: os.Getenv("SENTRY_ENVIRONMENT"),
			Release:     os.Getenv("SENTRY_RELEASE"),
		},
		Log: &LogConfig{
			AdGuardLogLevel: adguardLogLevel,
			ZerologLevel:    zerologLevel,
		},
	}, nil
}
