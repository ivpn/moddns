package updater

import (
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/redis/go-redis/v9"
)

type lockMetrics struct {
	metrics.NoopUpdates
	skipped    map[string][]string
	lockErrors []string
}

func newLockMetrics() *lockMetrics {
	return &lockMetrics{skipped: make(map[string][]string)}
}

func (l *lockMetrics) RecordRefreshSkipped(source, reason string) {
	l.skipped[source] = append(l.skipped[source], reason)
}

func (l *lockMetrics) RecordLockError(source string) {
	l.lockErrors = append(l.lockErrors, source)
}

func newTestGocronLocker(t *testing.T, m metrics.Updates) (*gocronLocker, *dislock.Locker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	dl := dislock.New(client, LockKeyPrefix)
	return NewDistributedLocker(dl, m).(*gocronLocker), dl, mr
}

// specRef: #G10 — the scheduler's locker acquires the per-source key and a
// second acquisition (a peer's tick) is refused and counted as lock_held.
func TestGocronLocker_ContentionIsCountedAsLockHeld(t *testing.T) {
	m := newLockMetrics()
	gl, dl, _ := newTestGocronLocker(t, m)

	key := SourceLockKey("blp_x")
	peer, err := dl.TryLock(t.Context(), key, SourceLockTTL)
	if err != nil {
		t.Fatalf("peer lock: %v", err)
	}
	defer func() { _ = peer.Unlock(t.Context()) }()

	lock, err := gl.Lock(t.Context(), key)
	if !errors.Is(err, dislock.ErrNotAcquired) {
		t.Fatalf("err = %v, want ErrNotAcquired", err)
	}
	if lock != nil {
		t.Fatal("lock must be nil on contention")
	}
	if got := m.skipped["blp_x"]; len(got) != 1 || got[0] != metrics.SkipReasonLockHeld {
		t.Fatalf("skipped = %v, want [lock_held]", got)
	}
}

// specRef: #G12 — a Redis-level failure during acquisition is counted as a
// lock error (the tick is skipped by the scheduler).
func TestGocronLocker_RedisErrorIsCountedAsLockError(t *testing.T) {
	m := newLockMetrics()
	gl, _, mr := newTestGocronLocker(t, m)
	mr.Close()

	_, err := gl.Lock(t.Context(), SourceLockKey("blp_y"))
	if err == nil || errors.Is(err, dislock.ErrNotAcquired) {
		t.Fatalf("err = %v, want a non-contention error", err)
	}
	if len(m.lockErrors) != 1 || m.lockErrors[0] != "blp_y" {
		t.Fatalf("lockErrors = %v, want [blp_y]", m.lockErrors)
	}
}

// specRef: #G10 — each source job is registered under its lock-key name, so
// the key gocron hands the distributed locker is per-source, not one shared
// lock for the whole scheduler.
func TestSetup_RegistersJobPerSourceWithLockKeyName(t *testing.T) {
	u, err := NewStandardUpdater(nil)
	if err != nil {
		t.Fatalf("NewStandardUpdater: %v", err)
	}
	t.Cleanup(u.Stop)

	sources := []model.BlocklistMetadata{
		{BlocklistID: "blp_a", Name: "A", Schedule: "1 * * * *"},
		{BlocklistID: "blp_b", Name: "B", Schedule: "2 * * * *"},
	}
	for _, src := range sources {
		if err := u.Setup(src, func() (*model.BlocklistMetadata, error) { return nil, nil }); err != nil {
			t.Fatalf("Setup(%s): %v", src.BlocklistID, err)
		}
	}

	names := make(map[string]bool)
	for _, job := range u.scheduler.Jobs() {
		names[job.Name()] = true
	}
	for _, want := range []string{"source:blp_a", "source:blp_b"} {
		if !names[want] {
			t.Fatalf("job names = %v, missing %q", names, want)
		}
	}
}

// An invalid cron spec must be rejected at Setup time, not at first tick.
func TestSetup_RejectsInvalidSchedule(t *testing.T) {
	u, err := NewStandardUpdater(nil)
	if err != nil {
		t.Fatalf("NewStandardUpdater: %v", err)
	}
	t.Cleanup(u.Stop)

	src := model.BlocklistMetadata{BlocklistID: "blp_bad", Name: "bad", Schedule: "not-a-cron"}
	if err := u.Setup(src, func() (*model.BlocklistMetadata, error) { return nil, nil }); err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}
