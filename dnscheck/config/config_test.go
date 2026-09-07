package config

import (
	"net"
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
	if len(cfg.Server.IPRanges) != 1 || cfg.Server.IPRanges[0].Net.String() != "10.5.0.0/16" || cfg.Server.IPRanges[0].Label != "" {
		t.Errorf("IPRanges = %v, want one unlabelled 10.5.0.0/16", cfg.Server.IPRanges)
	}

	// PoPs live in unrelated blocks, so a comma-separated list is accepted;
	// whitespace and a trailing comma are tolerated.
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "198.51.100.7/32, 203.0.113.0/24,")
	cfg, err = New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Server.IPRanges) != 2 {
		t.Fatalf("IPRanges = %v, want two entries", cfg.Server.IPRanges)
	}
	if !cfg.Server.ContainsIP(net.ParseIP("198.51.100.7")) || !cfg.Server.ContainsIP(net.ParseIP("203.0.113.9")) {
		t.Errorf("ContainsIP does not cover both configured ranges: %v", cfg.Server.IPRanges)
	}
	if cfg.Server.ContainsIP(net.ParseIP("198.51.100.8")) {
		t.Errorf("/32 entry must match a single address only")
	}

	// One bad entry fails the whole list.
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "198.51.100.7/32,10.5.")
	if _, err := New(); err == nil {
		t.Fatal("expected an error when one list entry is not CIDR")
	}

	// Entries may carry an operator-facing label; it never affects matching.
	t.Setenv("DNS_AUTH_SERVER_IP_RANGE", "tor1=198.51.100.7/32, lab = 203.0.113.0/24,192.0.2.0/24")
	cfg, err = New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Server.IPRangesString(); got != "tor1 198.51.100.7/32, lab 203.0.113.0/24, 192.0.2.0/24" {
		t.Errorf("IPRangesString() = %q", got)
	}
	if !cfg.Server.ContainsIP(net.ParseIP("198.51.100.7")) || !cfg.Server.ContainsIP(net.ParseIP("192.0.2.9")) {
		t.Errorf("labelled and unlabelled entries must both match: %v", cfg.Server.IPRanges)
	}

	for _, bad := range []string{"=198.51.100.7/32", "tor1=", "tor1=10.5."} {
		t.Setenv("DNS_AUTH_SERVER_IP_RANGE", bad)
		if _, err := New(); err == nil {
			t.Errorf("expected an error for DNS_AUTH_SERVER_IP_RANGE=%q", bad)
		}
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
