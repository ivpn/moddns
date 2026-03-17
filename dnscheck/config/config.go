package config

import (
	"errors"
	"os"
	"strconv"
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
