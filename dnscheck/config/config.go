package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
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
	// IPRanges are the CIDR blocks our resolvers query from; at least one is
	// required. PoPs sit in unrelated address blocks, so this is a list.
	IPRanges []*net.IPNet
}

// ContainsIP reports whether ip falls inside any configured range.
func (c *AuthoritativeDNSServerConfig) ContainsIP(ip net.IP) bool {
	for _, r := range c.IPRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
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

	ipRanges, err := parseIPRanges(os.Getenv("DNS_AUTH_SERVER_IP_RANGE"))
	if err != nil {
		return nil, err
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
			IPRanges:  ipRanges,
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

// parseIPRanges parses a comma-separated list of CIDR blocks. Every entry must
// parse and at least one is required.
func parseIPRanges(raw string) ([]*net.IPNet, error) {
	var ranges []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, n, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("DNS_AUTH_SERVER_IP_RANGE entries must be CIDR notation (e.g. 10.5.0.0/16 or 198.51.100.7/32), got %q", part)
		}
		ranges = append(ranges, n)
	}
	if len(ranges) == 0 {
		return nil, errors.New("DNS_AUTH_SERVER_IP_RANGE environment variable is required (comma-separated CIDR list)")
	}
	return ranges, nil
}
