package geoipdb

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oschwald/geoip2-golang/v2"
)

const (
	asnFixture   = "testdata/GeoLite2-ASN.mmdb"       // build 2026-09-04
	newerFixture = "testdata/GeoLite2-ASN.newer.mmdb" // same networks, build 2026-09-18
	cityFixture  = "testdata/GeoLite2-City.mmdb"
)

// installFixture copies src to dst with a stable mtime so a later replacement
// always changes the on-disk signature the reloader compares.
func installFixture(t *testing.T, src, dst string, mtime time.Time) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	// Same write-then-rename sequence geoipupdate uses.
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

func openTemp(t *testing.T) (*Reader, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	installFixture(t, asnFixture, path, time.Now().Add(-time.Hour))
	r, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, path
}

func mustASN(t *testing.T, r *Reader, ip string) *geoip2.ASN {
	t.Helper()
	rec, err := r.ASN(netip.MustParseAddr(ip))
	if err != nil {
		t.Fatalf("lookup %s: %v", ip, err)
	}
	return rec
}

func TestOpenRejectsMissingFile(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing.mmdb")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestOpenRejectsNonASNDatabase(t *testing.T) {
	if _, err := Open(cityFixture); err == nil {
		t.Fatal("expected an error when the path points at a City database")
	}
}

func TestASNReturnsRecordAndBuildTime(t *testing.T) {
	r, _ := openTemp(t)

	rec := mustASN(t, r, "8.8.8.8")
	if rec.AutonomousSystemNumber != 15169 {
		t.Errorf("got ASN %d, want 15169", rec.AutonomousSystemNumber)
	}
	if got := r.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-04" {
		t.Errorf("got build date %s, want 2026-09-04", got)
	}
	if rec := mustASN(t, r, "203.0.113.5"); rec.AutonomousSystemNumber != 0 {
		t.Errorf("unknown IP yielded ASN %d, want 0", rec.AutonomousSystemNumber)
	}
}

func TestReloadWithoutChangeIsNoop(t *testing.T) {
	r, _ := openTemp(t)

	changed, err := r.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if changed {
		t.Fatal("reload reported a change for an untouched file")
	}
	if s := r.Stats(); s.Reloads != 0 || s.Failures != 0 {
		t.Errorf("stats after noop: %+v", s)
	}
}

func TestReloadPicksUpReplacedFile(t *testing.T) {
	r, path := openTemp(t)
	installFixture(t, newerFixture, path, time.Now())

	changed, err := r.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !changed {
		t.Fatal("reload did not notice the replaced file")
	}
	s := r.Stats()
	if got := s.BuildTime.UTC().Format("2006-01-02"); got != "2026-09-18" {
		t.Errorf("got build date %s after reload, want 2026-09-18", got)
	}
	if s.Reloads != 1 || s.Failures != 0 || s.LastError != "" {
		t.Errorf("stats after reload: %+v", s)
	}
	if rec := mustASN(t, r, "8.8.8.8"); rec.AutonomousSystemNumber != 15169 {
		t.Errorf("lookup after reload: got ASN %d, want 15169", rec.AutonomousSystemNumber)
	}
}

// The overwrite here is deliberately in place (no rename): the loaded copy
// must not depend on the file's inode staying intact.
func TestReloadKeepsServingWhenReplacementIsCorrupt(t *testing.T) {
	r, path := openTemp(t)
	if err := os.WriteFile(path, []byte("not an mmdb"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := r.Reload()
	if err == nil {
		t.Fatal("expected an error for a corrupt replacement")
	}
	if changed {
		t.Fatal("a failed reload must not report a change")
	}
	s := r.Stats()
	if got := s.BuildTime.UTC().Format("2006-01-02"); got != "2026-09-04" {
		t.Errorf("build date changed to %s after a failed reload", got)
	}
	if s.Failures != 1 || s.LastError == "" {
		t.Errorf("stats after failed reload: %+v", s)
	}
	if rec := mustASN(t, r, "8.8.8.8"); rec.AutonomousSystemNumber != 15169 {
		t.Errorf("old database not served after failed reload: ASN %d", rec.AutonomousSystemNumber)
	}
}

func TestReloadKeepsServingWhenReplacementIsWrongType(t *testing.T) {
	r, path := openTemp(t)
	installFixture(t, cityFixture, path, time.Now())

	if _, err := r.Reload(); err == nil {
		t.Fatal("expected an error for a City database on the ASN path")
	}
	if rec := mustASN(t, r, "8.8.8.8"); rec.AutonomousSystemNumber != 15169 {
		t.Errorf("old database not served after wrong-type reload: ASN %d", rec.AutonomousSystemNumber)
	}
}

func TestReloadKeepsServingWhenFileVanishes(t *testing.T) {
	r, path := openTemp(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Reload(); err == nil {
		t.Fatal("expected an error when the file is gone")
	}
	if rec := mustASN(t, r, "8.8.8.8"); rec.AutonomousSystemNumber != 15169 {
		t.Errorf("old database not served after the file vanished: ASN %d", rec.AutonomousSystemNumber)
	}
}

// Lookups in flight while the file is swapped must never touch an unmapped
// reader. Run with -race.
func TestReloadIsSafeUnderConcurrentLookups(t *testing.T) {
	r, path := openTemp(t)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if rec, err := r.ASN(netip.MustParseAddr("8.8.8.8")); err != nil || rec.AutonomousSystemNumber != 15169 {
						t.Errorf("lookup during reload: rec=%+v err=%v", rec, err)
						return
					}
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		src := asnFixture
		if i%2 == 0 {
			src = newerFixture
		}
		installFixture(t, src, path, time.Now().Add(time.Duration(i)*time.Second))
		if _, err := r.Reload(); err != nil {
			t.Fatalf("reload %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()
}

func TestWatchReloadsOnInterval(t *testing.T) {
	r, path := openTemp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		r.Watch(ctx, 10*time.Millisecond)
		close(done)
	}()

	installFixture(t, newerFixture, path, time.Now())
	deadline := time.Now().Add(5 * time.Second)
	for r.Stats().Reloads == 0 {
		if time.Now().After(deadline) {
			t.Fatal("watcher did not reload the replaced file")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := r.Stats().BuildTime.UTC().Format("2006-01-02"); got != "2026-09-18" {
		t.Errorf("got build date %s after watched reload, want 2026-09-18", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop on context cancel")
	}
}

func TestCloseThenASNReturnsError(t *testing.T) {
	r, _ := openTemp(t)
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := r.ASN(netip.MustParseAddr("8.8.8.8")); err == nil {
		t.Fatal("expected an error from a closed reader")
	}
}

func BenchmarkASN(b *testing.B) {
	r, err := Open(asnFixture)
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	ip := netip.MustParseAddr("8.8.8.8")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := r.ASN(ip); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Baseline: the bare geoip2 reader without the reload guard.
func BenchmarkASNRawReader(b *testing.B) {
	db, err := geoip2.Open(asnFixture)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	ip := netip.MustParseAddr("8.8.8.8")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := db.ASN(ip); err != nil {
				b.Fatal(err)
			}
		}
	})
}
