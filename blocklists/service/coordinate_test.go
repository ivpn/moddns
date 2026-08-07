package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/internal/downloader"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/redis/go-redis/v9"
)

// coordMetrics extends fakeMetrics with the coordination counters.
type coordMetrics struct {
	metrics.NoopUpdates
	skipped    map[string][]string // source -> reasons
	lockErrors []string
}

func newCoordMetrics() *coordMetrics {
	return &coordMetrics{skipped: make(map[string][]string)}
}

func (c *coordMetrics) RecordRefreshSkipped(source, reason string) {
	c.skipped[source] = append(c.skipped[source], reason)
}

func (c *coordMetrics) RecordLockError(source string) {
	c.lockErrors = append(c.lockErrors, source)
}

// newCoordLocker returns a dislock locker backed by an in-memory Redis.
func newCoordLocker(t *testing.T) (*dislock.Locker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return dislock.New(client, updater.LockKeyPrefix), mr
}

func coordSource(id, schedule, url string) model.BlocklistMetadata {
	return model.BlocklistMetadata{BlocklistID: id, Name: id, SourceUrl: url, Schedule: schedule}
}

// newCoordService builds a Service whose store serves the given metadata and
// whose downloader talks to real (httptest) URLs.
func newCoordService(m metrics.Updates, locker *dislock.Locker, stored []model.BlocklistMetadata) (*Service, *fakeStore) {
	store := &fakeStore{metadata: stored}
	cfg := config.Config{Updater: &config.UpdaterConfig{ShrinkThreshold: 0.5}}
	return &Service{
		Cfg:        cfg,
		Store:      store,
		Cache:      &fakeCache{},
		Metrics:    m,
		Downloader: downloader.New(downloader.Config{Timeout: 2 * time.Second, MaxAttempts: 1}, m),
		Locker:     locker,
	}, store
}

func TestScheduleInterval(t *testing.T) {
	tests := []struct {
		spec string
		want time.Duration
	}{
		// specRef: #G11 — staggered hourly source entries have a 1h period.
		{"17 * * * *", time.Hour},
		{"@hourly", time.Hour},
		// specRef: #G11 — daily OISD windows have a 24h period.
		{"0 3 * * *", 24 * time.Hour},
		// Unparseable spec falls back to a safe non-zero interval.
		{"not-a-cron", fallbackInterval},
	}
	for _, tc := range tests {
		if got := scheduleInterval(tc.spec); got != tc.want {
			t.Fatalf("scheduleInterval(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}

// specRef: #G11 — a source published within half its schedule interval is
// fresh: no lock is needed, nothing is downloaded.
func TestRefreshDue_SkipsFreshSource(t *testing.T) {
	m := newCoordMetrics()
	stored := []model.BlocklistMetadata{{BlocklistID: "blp_x", UpdatedAt: time.Now().UTC().Add(-10 * time.Minute)}}
	s, _ := newCoordService(m, nil, stored)

	// SourceUrl is unroutable: any download attempt would error, so a nil,nil
	// return proves the refresh was skipped before downloading.
	meta, err := s.RefreshDue(coordSource("blp_x", "0 * * * *", "http://127.0.0.1:1/x"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta != nil {
		t.Fatalf("expected fresh skip (nil meta), got %+v", meta)
	}
	if got := m.skipped["blp_x"]; len(got) != 1 || got[0] != metrics.SkipReasonFresh {
		t.Fatalf("skipped reasons = %v, want [fresh]", got)
	}
}

// specRef: #G11 — a source older than half its schedule interval is stale and
// is processed (downloaded and republished, updated_at advanced).
func TestRefreshDue_ProcessesStaleSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ads.example.com\ntracker.example.net\n"))
	}))
	t.Cleanup(srv.Close)

	m := newCoordMetrics()
	stored := []model.BlocklistMetadata{{BlocklistID: "blp_y", Entries: 2, UpdatedAt: time.Now().UTC().Add(-2 * time.Hour)}}
	s, _ := newCoordService(m, nil, stored)

	meta, err := s.RefreshDue(coordSource("blp_y", "0 * * * *", srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil {
		t.Fatal("expected processed metadata, got nil")
	}
	if meta.UpdatedAt.IsZero() || time.Since(meta.UpdatedAt) > time.Minute {
		t.Fatalf("updated_at not advanced: %v", meta.UpdatedAt)
	}
	if len(m.skipped["blp_y"]) != 0 {
		t.Fatalf("unexpected skips: %v", m.skipped)
	}
}

// specRef: #G11 — metadata without updated_at (pre-upgrade documents) counts
// as stale, so the first coordinated run performs a full refresh.
func TestIsFresh_ZeroUpdatedAtIsStale(t *testing.T) {
	s, _ := newCoordService(metrics.NoopUpdates{}, nil, []model.BlocklistMetadata{{BlocklistID: "blp_z"}})
	if s.isFresh(context.Background(), coordSource("blp_z", "0 * * * *", "")) {
		t.Fatal("zero updated_at must be stale")
	}
}

// specRef: #G10 — while a peer instance holds the source lock, the refresh is
// skipped as lock_held (not failed) and nothing is downloaded.
func TestRefreshWithLock_SkipsWhenPeerHoldsLock(t *testing.T) {
	locker, _ := newCoordLocker(t)
	m := newCoordMetrics()
	s, _ := newCoordService(m, locker, nil)

	peer, err := locker.TryLock(context.Background(), updater.SourceLockKey("blp_held"), time.Minute)
	if err != nil {
		t.Fatalf("peer lock: %v", err)
	}
	defer func() { _ = peer.Unlock(context.Background()) }()

	outcome := s.refreshWithLock(coordSource("blp_held", "0 * * * *", "http://127.0.0.1:1/x"))
	if outcome != outcomeLockHeld {
		t.Fatalf("outcome = %s, want %s", outcome, outcomeLockHeld)
	}
	if got := m.skipped["blp_held"]; len(got) != 1 || got[0] != metrics.SkipReasonLockHeld {
		t.Fatalf("skipped reasons = %v, want [lock_held]", got)
	}
}

// specRef: #G12 — a Redis-level lock failure fails closed: the refresh is
// skipped, counted as a lock error, and reported as failed.
func TestRefreshWithLock_FailsClosedOnLockerError(t *testing.T) {
	locker, mr := newCoordLocker(t)
	m := newCoordMetrics()
	s, _ := newCoordService(m, locker, nil)

	mr.Close() // Redis unreachable: TryLock returns a non-contention error.

	outcome := s.refreshWithLock(coordSource("blp_err", "0 * * * *", "http://127.0.0.1:1/x"))
	if outcome != outcomeFailed {
		t.Fatalf("outcome = %s, want %s", outcome, outcomeFailed)
	}
	if len(m.lockErrors) != 1 || m.lockErrors[0] != "blp_err" {
		t.Fatalf("lockErrors = %v, want [blp_err]", m.lockErrors)
	}
}

// specRef: #G10 — after winning the lock, the freshness re-check inside the
// lock still applies (the peer that just released it may have refreshed).
func TestRefreshWithLock_FreshInsideLock(t *testing.T) {
	locker, _ := newCoordLocker(t)
	m := newCoordMetrics()
	stored := []model.BlocklistMetadata{{BlocklistID: "blp_f", UpdatedAt: time.Now().UTC().Add(-5 * time.Minute)}}
	s, _ := newCoordService(m, locker, stored)

	outcome := s.refreshWithLock(coordSource("blp_f", "0 * * * *", "http://127.0.0.1:1/x"))
	if outcome != outcomeFresh {
		t.Fatalf("outcome = %s, want %s", outcome, outcomeFresh)
	}
}

// specRef: #G10 — the boot catch-up releases each source lock, so a
// subsequent acquire succeeds immediately.
func TestRefreshWithLock_ReleasesLock(t *testing.T) {
	locker, _ := newCoordLocker(t)
	stored := []model.BlocklistMetadata{{BlocklistID: "blp_r", UpdatedAt: time.Now().UTC()}}
	s, _ := newCoordService(metrics.NoopUpdates{}, locker, stored)

	_ = s.refreshWithLock(coordSource("blp_r", "0 * * * *", ""))

	lock, err := locker.TryLock(context.Background(), updater.SourceLockKey("blp_r"), time.Minute)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	_ = lock.Unlock(context.Background())
}

// specRef: #H5 — the stale purge runs under a distributed lock: while a peer
// holds it, nothing is deleted.
func TestPurgeStaleCoordinated_SkipsWhenPeerHoldsLock(t *testing.T) {
	locker, _ := newCoordLocker(t)
	stored := metaFor("blp_keep", "blp_stale")
	store := &fakeStore{metadata: stored}
	cache := &fakeCache{}
	s := &Service{
		Cfg:     config.Config{Updater: &config.UpdaterConfig{}},
		Store:   store,
		Cache:   cache,
		Metrics: metrics.NoopUpdates{},
		Locker:  locker,
	}

	peer, err := locker.TryLock(context.Background(), updater.PurgeLockKey, time.Minute)
	if err != nil {
		t.Fatalf("peer lock: %v", err)
	}
	defer func() { _ = peer.Unlock(context.Background()) }()

	s.PurgeStaleCoordinated(metaFor("blp_keep"))

	if len(store.deletedMeta) != 0 || len(cache.deleted) != 0 {
		t.Fatalf("purge ran under a held lock: meta=%v cache=%v", store.deletedMeta, cache.deleted)
	}
}

// specRef: #H5 — with the lock free, the coordinated purge behaves like
// PurgeStale and releases the lock afterwards.
func TestPurgeStaleCoordinated_PurgesAndReleases(t *testing.T) {
	locker, _ := newCoordLocker(t)
	store := &fakeStore{metadata: metaFor("blp_keep", "blp_stale")}
	cache := &fakeCache{}
	s := &Service{
		Cfg:     config.Config{Updater: &config.UpdaterConfig{}},
		Store:   store,
		Cache:   cache,
		Metrics: metrics.NoopUpdates{},
		Locker:  locker,
	}

	s.PurgeStaleCoordinated(metaFor("blp_keep"))

	if len(cache.deleted) != 1 || cache.deleted[0] != "blp_stale" {
		t.Fatalf("deleted cache = %v, want [blp_stale]", cache.deleted)
	}
	lock, err := locker.TryLock(context.Background(), updater.PurgeLockKey, time.Minute)
	if err != nil {
		t.Fatalf("purge lock was not released: %v", err)
	}
	_ = lock.Unlock(context.Background())
}

// specRef: #H6 — a stale set larger than the configured maximum means this
// instance's sources diverged from what the cluster published; the purge is
// refused entirely.
func TestPurgeStale_RefusesMassPurge(t *testing.T) {
	stored := metaFor("blp_keep", "blp_s1", "blp_s2", "blp_s3")
	store := &fakeStore{metadata: stored}
	cache := &fakeCache{}
	s := &Service{
		Cfg:     config.Config{Updater: &config.UpdaterConfig{MaxStalePurge: 2}},
		Store:   store,
		Cache:   cache,
		Metrics: metrics.NoopUpdates{},
	}

	s.PurgeStale(metaFor("blp_keep"))

	if len(store.deletedMeta) != 0 || len(store.deletedContent) != 0 || len(cache.deleted) != 0 {
		t.Fatalf("mass purge not refused: meta=%v content=%v cache=%v",
			store.deletedMeta, store.deletedContent, cache.deleted)
	}
}

// specRef: #H6 — a stale count at (not above) the maximum still purges.
func TestPurgeStale_AllowsPurgeAtMax(t *testing.T) {
	store := &fakeStore{metadata: metaFor("blp_keep", "blp_s1", "blp_s2")}
	cache := &fakeCache{}
	s := &Service{
		Cfg:     config.Config{Updater: &config.UpdaterConfig{MaxStalePurge: 2}},
		Store:   store,
		Cache:   cache,
		Metrics: metrics.NoopUpdates{},
	}

	s.PurgeStale(metaFor("blp_keep"))

	if len(cache.deleted) != 2 {
		t.Fatalf("deleted cache = %v, want 2 entries", cache.deleted)
	}
}
