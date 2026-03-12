package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server          *AuthoritativeDNSServerConfig
	API             *APIConfig
	Cache           *CacheConfig
	GeoLookupConfig *GeoLookupConfig
}

// AuthoritativeDNSServerConfig represents the authoritative DNS server configuration
type AuthoritativeDNSServerConfig struct {
	Domain    string
	IPAddress string
	ASN       uint
	IPRange   string
}

// APIConfig represents the API configuration
type APIConfig struct {
	Port              string
	JWTSigningKey     string
	JWTExpirationTime time.Duration
	BasicAuthUser     string
	BasicAuthPassword string
	ApiAllowOrigin    string
	TrustedProxies    []string
}

// CacheConfig represents the cache configuration
type CacheConfig struct {
	TTL     time.Duration
	HMACKey string
}

// GeoLookupConfig represents access to MaxMind GeoIP database
type GeoLookupConfig struct {
	DBFile    string
	DBASNFile string
}

// IsValid check whether config section is valid
func (cfg *GeoLookupConfig) IsValid() error {
	if cfg.DBFile == "" {
		return errors.New("[GeoIP] DBFile is required")

	}
	if cfg.DBASNFile == "" {
		return errors.New("[GeoIP] DBISP is required")

	}
	return nil
}

// defaultTrustedProxies covers RFC 1918 private ranges (Docker, K8s, typical reverse proxies).
var defaultTrustedProxies = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}

// parseTrustedProxies reads API_TRUSTED_PROXIES as a comma-separated list of CIDRs.
// Falls back to RFC 1918 defaults if unset.
func parseTrustedProxies() []string {
	v := os.Getenv("API_TRUSTED_PROXIES")
	if v == "" {
		return defaultTrustedProxies
	}
	var proxies []string
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			proxies = append(proxies, p)
		}
	}
	if len(proxies) == 0 {
		return defaultTrustedProxies
	}
	return proxies
}

// New creates a new Config instance
func New() (*Config, error) {
	cacheTTL := os.Getenv("CACHE_TTL")
	ttl, err := time.ParseDuration(cacheTTL)
	if err != nil {
		ttl = 1 * time.Minute
	}

	asn := os.Getenv("DNS_AUTH_SERVER_ASN")
	asnUint, err := strconv.ParseUint(asn, 0, 32)
	if err != nil {
		asnUint = 123456 // non-existent ASN
	}

	cacheHMACKey := os.Getenv("CACHE_HMAC_KEY")
	if cacheHMACKey == "" {
		return nil, errors.New("CACHE_HMAC_KEY environment variable is required")
	}

	return &Config{
		Server: &AuthoritativeDNSServerConfig{
			Domain:    os.Getenv("DNS_AUTH_SERVER_DOMAIN"),
			IPAddress: os.Getenv("DNS_AUTH_SERVER_IP_ADDRESS"),
			ASN:       uint(asnUint),
			IPRange:   os.Getenv("DNS_AUTH_SERVER_IP_RANGE"),
		},
		API: &APIConfig{
			Port:           os.Getenv("API_PORT"),
			ApiAllowOrigin: os.Getenv("API_ALLOW_ORIGIN"),
			TrustedProxies: parseTrustedProxies(),
		},
		Cache: &CacheConfig{
			TTL:     ttl,
			HMACKey: cacheHMACKey,
		},
		GeoLookupConfig: &GeoLookupConfig{
			DBFile:    os.Getenv("GEOIP_DB_FILE"),
			DBASNFile: os.Getenv("GEOIP_DB_ASN_FILE"),
		},
	}, nil
}
