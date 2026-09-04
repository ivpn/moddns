package config

import "testing"

// specRef: dnscheck-behaviour.md #S2
func TestNewRequiresASNDatabasePath(t *testing.T) {
	t.Setenv("CACHE_HMAC_KEY", "test-key")
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
