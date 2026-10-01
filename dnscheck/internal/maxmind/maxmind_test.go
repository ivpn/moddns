package maxmind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	asnFixture   = "testdata/GeoLite2-ASN.mmdb"       // build 2026-09-04
	newerFixture = "testdata/GeoLite2-ASN.newer.mmdb" // same networks, build 2026-09-18
	cityFixture  = "testdata/GeoLite2-City.mmdb"
)

// installFixture mirrors geoipupdate: write a temporary file, then rename it
// over the target.
func installFixture(t *testing.T, src, dst string, mtime time.Time) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	tmp := dst + ".temporary"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmp, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		t.Fatal(err)
	}
}

// A database refreshed on disk is served after the next reload; the process
// never has to restart.
//
// specRef: dnscheck-behaviour.md #S6
func TestGeoLookupManagerReloadsReplacedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	installFixture(t, asnFixture, path, time.Now().Add(-time.Hour))
	g, err := NewGeoLookupManager(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer g.Close()
	if got := g.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-04" {
		t.Fatalf("initial build date %s, want 2026-09-04", got)
	}

	installFixture(t, newerFixture, path, time.Now())
	changed, err := g.Reload()
	if err != nil || !changed {
		t.Fatalf("reload: changed=%v err=%v", changed, err)
	}
	if got := g.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-18" {
		t.Errorf("build date after reload %s, want 2026-09-18", got)
	}
	got, err := g.GetGeoLookup("8.8.8.8")
	if err != nil || got.ASN != 15169 {
		t.Errorf("lookup after reload: %+v err=%v", got, err)
	}
}

// A broken replacement (corrupt file or City edition on the ASN path) is
// rejected and the previously loaded database keeps answering.
//
// specRef: dnscheck-behaviour.md #S7
func TestGeoLookupManagerKeepsOldDatabaseWhenReplacementIsBad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	installFixture(t, asnFixture, path, time.Now().Add(-time.Hour))
	g, err := NewGeoLookupManager(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer g.Close()

	installFixture(t, cityFixture, path, time.Now())
	if _, err := g.Reload(); err == nil {
		t.Fatal("expected an error for a City database on the ASN path")
	}
	if err := os.WriteFile(path, []byte("not an mmdb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Reload(); err == nil {
		t.Fatal("expected an error for a corrupt replacement")
	}

	if s := g.Stats(); s.Failures != 2 || s.BuildTime.UTC().Format("2006-01-02") != "2026-09-04" {
		t.Errorf("stats after failed reloads: %+v", s)
	}
	got, err := g.GetGeoLookup("8.8.8.8")
	if err != nil || got.ASN != 15169 {
		t.Errorf("old database not served after failed reloads: %+v err=%v", got, err)
	}
}

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
