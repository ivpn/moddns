package maxmind

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	asnFixture  = "testdata/GeoLite2-ASN.mmdb"
	cityFixture = "testdata/GeoLite2-City.mmdb"
)

// specRef: dnscheck-behaviour.md #S2
func TestNewGeoLookupManagerRejectsMissingFile(t *testing.T) {
	_, err := NewGeoLookupManager(filepath.Join(t.TempDir(), "missing.mmdb"))
	if err == nil {
		t.Fatal("expected an error for a missing database file")
	}
}

// specRef: dnscheck-behaviour.md #S2
func TestNewGeoLookupManagerRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.mmdb")
	if err := os.WriteFile(path, []byte("this is not an mmdb file"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewGeoLookupManager(path)
	if err == nil {
		t.Fatal("expected an error for a corrupt database file")
	}
}

// specRef: dnscheck-behaviour.md #S3
func TestNewGeoLookupManagerRejectsNonASNDatabase(t *testing.T) {
	_, err := NewGeoLookupManager(cityFixture)
	if err == nil {
		t.Fatal("expected an error when the ASN path points at a City database")
	}
}

// specRef: dnscheck-behaviour.md #D5
func TestGetGeoLookupReturnsASNForKnownIP(t *testing.T) {
	g, err := NewGeoLookupManager(asnFixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer g.Close()

	got, err := g.GetGeoLookup("8.8.8.8")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ASN != 15169 || got.ASNOrganization != "GOOGLE" {
		t.Errorf("got ASN=%d org=%q, want 15169 GOOGLE", got.ASN, got.ASNOrganization)
	}
	if got.IPAddress != "8.8.8.8" {
		t.Errorf("got IPAddress=%q, want 8.8.8.8", got.IPAddress)
	}
}

// An address absent from the database is not an error: the record is empty and
// the caller falls back to its IP-range check.
//
// specRef: dnscheck-behaviour.md #D6
func TestGetGeoLookupUnknownIPYieldsEmptyRecord(t *testing.T) {
	g, err := NewGeoLookupManager(asnFixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer g.Close()

	got, err := g.GetGeoLookup("203.0.113.5")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got == nil {
		t.Fatal("got nil record for an unknown IP, want an empty record")
	}
	if got.ASN != 0 || got.ASNOrganization != "" {
		t.Errorf("got ASN=%d org=%q, want empty record", got.ASN, got.ASNOrganization)
	}
}

// specRef: dnscheck-behaviour.md #D6
func TestGetGeoLookupRejectsUnparseableIP(t *testing.T) {
	g, err := NewGeoLookupManager(asnFixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer g.Close()

	got, err := g.GetGeoLookup("not-an-ip")
	if err == nil {
		t.Fatal("expected an error for an unparseable IP")
	}
	if got != nil {
		t.Errorf("expected a nil record alongside the error, got %+v", got)
	}
}

// Readers are opened once at startup and shared by every request goroutine.
//
// specRef: dnscheck-behaviour.md #S1
func TestGetGeoLookupIsSafeForConcurrentUse(t *testing.T) {
	g, err := NewGeoLookupManager(asnFixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer g.Close()

	done := make(chan error, 32)
	for i := 0; i < 32; i++ {
		go func() {
			_, err := g.GetGeoLookup("8.8.8.8")
			done <- err
		}()
	}
	for i := 0; i < 32; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent lookup: %v", err)
		}
	}
}
