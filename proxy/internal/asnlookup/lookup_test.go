package asnlookup

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	asnFixture   = "testdata/GeoLite2-ASN.mmdb"       // build 2026-09-04
	newerFixture = "testdata/GeoLite2-ASN.newer.mmdb" // same networks, build 2026-09-18
)

func install(t *testing.T, src, dst string, mtime time.Time) {
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

// specRef: proxy-filtering-behaviour.md #GEO1
func TestNewRequiresPath(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestASNReturnsNumberAndZeroForUnknownOrNil(t *testing.T) {
	l, err := New(asnFixture)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()

	if asn, err := l.ASN(net.ParseIP("8.8.8.8")); err != nil || asn != 15169 {
		t.Errorf("got asn=%d err=%v, want 15169", asn, err)
	}
	if asn, err := l.ASN(net.ParseIP("203.0.113.5")); err != nil || asn != 0 {
		t.Errorf("unknown IP: got asn=%d err=%v, want 0", asn, err)
	}
	if asn, err := l.ASN(nil); err != nil || asn != 0 {
		t.Errorf("nil IP: got asn=%d err=%v, want 0", asn, err)
	}
	var none *Lookup
	if asn, err := none.ASN(net.ParseIP("8.8.8.8")); err != nil || asn != 0 {
		t.Errorf("nil lookup: got asn=%d err=%v, want 0", asn, err)
	}
}

// A database refreshed on disk (write + rename, as geoipupdate does) is served
// after the next reload without reopening the Lookup.
//
// specRef: proxy-filtering-behaviour.md #GEO2
func TestReloadServesReplacedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	install(t, asnFixture, path, time.Now().Add(-time.Hour))
	l, err := New(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()
	if got := l.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-04" {
		t.Fatalf("initial build date %s, want 2026-09-04", got)
	}

	install(t, newerFixture, path, time.Now())
	changed, err := l.Reload()
	if err != nil || !changed {
		t.Fatalf("reload: changed=%v err=%v", changed, err)
	}
	if got := l.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-18" {
		t.Errorf("build date after reload %s, want 2026-09-18", got)
	}
	if asn, err := l.ASN(net.ParseIP("8.8.8.8")); err != nil || asn != 15169 {
		t.Errorf("lookup after reload: asn=%d err=%v", asn, err)
	}
}

// A corrupt replacement is rejected and the loaded database keeps serving.
//
// specRef: proxy-filtering-behaviour.md #GEO3
func TestReloadKeepsOldDatabaseWhenReplacementIsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	install(t, asnFixture, path, time.Now().Add(-time.Hour))
	l, err := New(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()

	if err := os.WriteFile(path, []byte("not an mmdb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reload(); err == nil {
		t.Fatal("expected an error for a corrupt replacement")
	}
	if s := l.Stats(); s.Failures != 1 || s.LastError == "" {
		t.Errorf("stats after failed reload: %+v", s)
	}
	if asn, err := l.ASN(net.ParseIP("8.8.8.8")); err != nil || asn != 15169 {
		t.Errorf("old database not served after failed reload: asn=%d err=%v", asn, err)
	}
}
