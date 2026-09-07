package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// DefaultCacheTTL bounds how long a check record stays in memory. The frontend
// reads it within milliseconds of the DNS query, so this only needs to cover
// resolver retries.
const DefaultCacheTTL = 15 * time.Second

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
	// IPRange is the CIDR block our resolvers query from; required.
	IPRange *net.IPNet
}

// APIConfig represents the API configuration
type APIConfig struct {
	Port              string
	JWTSigningKey     string
	JWTExpirationTime time.Duration
	BasicAuthUser     string
	BasicAuthPassword string
	ApiAllowOrigin    string
}

// CacheConfig represents the cache configuration
type CacheConfig struct {
	TTL     time.Duration
	HMACKey string
}

// GeoLookupConfig represents access to the MaxMind GeoIP ASN database
type GeoLookupConfig struct {
	DBASNFile string
}

// IsValid check whether config section is valid
func (cfg *GeoLookupConfig) IsValid() error {
	if cfg.DBASNFile == "" {
		return errors.New("GEOIP_DB_ASN_FILE environment variable is required")
	}
	return nil
}

// New creates a new Config instance
func New() (*Config, error) {
	ttl := DefaultCacheTTL
	if raw := os.Getenv("CACHE_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("CACHE_TTL must be a positive duration, got %q", raw)
		}
		ttl = parsed
	}

	rawRange := os.Getenv("DNS_AUTH_SERVER_IP_RANGE")
	if rawRange == "" {
		return nil, errors.New("DNS_AUTH_SERVER_IP_RANGE environment variable is required")
	}
	_, ipRange, err := net.ParseCIDR(rawRange)
	if err != nil {
		return nil, fmt.Errorf("DNS_AUTH_SERVER_IP_RANGE must be CIDR notation (e.g. 10.5.0.0/16), got %q", rawRange)
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

	geoLookup := &GeoLookupConfig{
		DBASNFile: os.Getenv("GEOIP_DB_ASN_FILE"),
	}
	if err := geoLookup.IsValid(); err != nil {
		return nil, err
	}

	return &Config{
		Server: &AuthoritativeDNSServerConfig{
			Domain:    os.Getenv("DNS_AUTH_SERVER_DOMAIN"),
			IPAddress: os.Getenv("DNS_AUTH_SERVER_IP_ADDRESS"),
			ASN:       uint(asnUint),
			IPRange:   ipRange,
		},
		API: &APIConfig{
			Port:           os.Getenv("API_PORT"),
			ApiAllowOrigin: os.Getenv("API_ALLOW_ORIGIN"),
		},
		Cache: &CacheConfig{
			TTL:     ttl,
			HMACKey: cacheHMACKey,
		},
		GeoLookupConfig: geoLookup,
	}, nil
}
