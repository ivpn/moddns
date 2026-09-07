package config

import (
	"testing"
	"time"
)

// specRef: dnscheck-behaviour.md #S2
func TestNewRequiresASNDatabasePath(t *testing.T) {
	t.Setenv("CACHE_HMAC_KEY", "test-key")
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "10.5.0.0/16")
	t.Setenv("GEOIP_DB_ASN_FILE", "")

	if _, err := New(); err == nil {
		t.Fatal("expected an error when GEOIP_DB_ASN_FILE is unset")
	}

	t.Setenv("GEOIP_DB_ASN_FILE", "/opt/dnscheck/GeoLite2-ASN.mmdb")
	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GeoLookupConfig.DBASNFile != "/opt/dnscheck/GeoLite2-ASN.mmdb" {
		t.Errorf("DBASNFile = %q", cfg.GeoLookupConfig.DBASNFile)
	}
}

// specRef: dnscheck-behaviour.md #S4
func TestNewParsesIPRangeAsCIDR(t *testing.T) {
	t.Setenv("CACHE_HMAC_KEY", "test-key")
	t.Setenv("GEOIP_DB_ASN_FILE", "/opt/dnscheck/GeoLite2-ASN.mmdb")

	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "10.5.0.0/16")
	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.IPRange == nil || cfg.Server.IPRange.String() != "10.5.0.0/16" {
		t.Errorf("IPRange = %v, want 10.5.0.0/16", cfg.Server.IPRange)
	}

	// The range is what makes a query "ours"; without it every check would
	// depend on the ASN alone, so an unset value is a boot error.
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "")
	if _, err := New(); err == nil {
		t.Fatal("expected an error when DNS_AUTH_SERVER_IP_RANGE is unset")
	}

	// The legacy string-prefix form is rejected so a misconfiguration fails at
	// boot instead of silently matching the wrong addresses.
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "10.5.")
	if _, err := New(); err == nil {
		t.Fatal("expected an error for a non-CIDR IP range")
	}
}

// specRef: dnscheck-behaviour.md #S5
func TestNewCacheTTLDefaultsAndValidates(t *testing.T) {
	t.Setenv("CACHE_HMAC_KEY", "test-key")
	t.Setenv("GEOIP_DB_ASN_FILE", "/opt/dnscheck/GeoLite2-ASN.mmdb")
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "10.5.0.0/16")

	t.Setenv("CACHE_TTL", "")
	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cache.TTL != DefaultCacheTTL {
		t.Errorf("TTL = %v, want default %v", cfg.Cache.TTL, DefaultCacheTTL)
	}

	t.Setenv("CACHE_TTL", "30s")
	cfg, err = New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cache.TTL != 30*time.Second {
		t.Errorf("TTL = %v, want 30s", cfg.Cache.TTL)
	}

	for _, bad := range []string{"soon", "-5s", "0"} {
		t.Setenv("CACHE_TTL", bad)
		if _, err := New(); err == nil {
			t.Errorf("expected an error for CACHE_TTL=%q", bad)
		}
	}
}
