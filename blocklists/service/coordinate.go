package service

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// freshnessFraction is the fraction of a source's schedule interval within
// which a published list counts as fresh and is not re-downloaded. At 0.5 an
// hourly source refreshed by a peer instance under 30 minutes ago is skipped;
// a missed tick (crash, lock lost) still recovers on the next one. The margin
// also absorbs minutes of clock skew between instances.
const freshnessFraction = 0.5

// refreshOutcome classifies how a coordinated refresh attempt ended.
type refreshOutcome string

const (
	outcomeProcessed refreshOutcome = "processed"
	outcomeFresh     refreshOutcome = "fresh"
	outcomeLockHeld  refreshOutcome = "lock_held"
	outcomeFailed    refreshOutcome = "failed"
)

// fallbackInterval is used when a source's schedule spec does not parse. Setup
// rejects such sources anyway, so this only guards arithmetic on bad input.
const fallbackInterval = time.Hour

// scheduleInterval derives the period of a cron spec from the gap between its
// next two firings — exact for the fixed-interval schedules the sources use
// (staggered hourly entries and daily OISD windows).
func scheduleInterval(spec string) time.Duration {
	sched, err := cron.ParseStandard(spec)
	if err != nil {
		return fallbackInterval
	}
	next := sched.Next(time.Now())
	return sched.Next(next).Sub(next)
}

// isFresh reports whether the source's published metadata is newer than
// freshnessFraction of its schedule interval AND its live set is still
// present in the cache. Metadata that cannot be read, is absent, or predates
// the updated_at field counts as stale, so the refresh proceeds and
// publishing decides. The cache existence check runs only when a skip is
// imminent: fresh metadata cannot prove the published set survived (a cache
// flush or failed restore would otherwise leave the proxy without the list
// until the next tick, and a repair-by-restart would skip it too).
func (s *Service) isFresh(ctx context.Context, src model.BlocklistMetadata) bool {
	existing, err := s.Store.GetMetadata(ctx, map[string]any{"blocklist_id": src.BlocklistID})
	if err != nil || len(existing) != 1 {
		return false
	}
	updatedAt := existing[0].UpdatedAt
	if updatedAt.IsZero() {
		return false
	}
	window := time.Duration(freshnessFraction * float64(scheduleInterval(src.Schedule)))
	if time.Since(updatedAt) >= window {
		return false
	}
	exists, err := s.Cache.BlocklistExists(ctx, src.BlocklistID)
	if err != nil || !exists {
		return false
	}
	// A live main set without its expected exception set would reintroduce
	// the false positives the exceptions prevent, so it counts as lost too.
	if existing[0].ExceptionEntries > 0 {
		exists, err = s.Cache.BlocklistExceptionsExist(ctx, src.BlocklistID)
		return err == nil && exists
	}
	return true
}

// RefreshDue processes the source unless it was published recently. It is the
// job body behind both paths that already hold the source's distributed lock:
// scheduled ticks (gocron's locker) and the boot-time catch-up. The freshness
// check inside the lock is the durable backstop for the tick a peer instance
// just completed: the winner's updated_at makes every other instance skip.
// A nil, nil return means the source was fresh and nothing was downloaded.
func (s *Service) RefreshDue(src model.BlocklistMetadata) (*model.BlocklistMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	if s.isFresh(ctx, src) {
		s.Metrics.RecordRefreshSkipped(src.BlocklistID, metrics.SkipReasonFresh)
		return nil, nil
	}
	return s.ProcessBlocklist(src)
}

// refreshWithLock acquires the source's distributed lock, then refreshes via
// RefreshDue. Lock contention is a normal outcome (a peer is refreshing); a
// Redis error fails closed — publishing needs Redis anyway — and is counted
// so silent non-refreshing is observable. A nil Locker (tests,
// single-instance dev) runs without coordination.
func (s *Service) refreshWithLock(src model.BlocklistMetadata) refreshOutcome {
	ctx := context.Background()

	if s.Locker != nil {
		lock, err := s.Locker.TryLock(ctx, updater.SourceLockKey(src.BlocklistID), updater.SourceLockTTL)
		switch {
		case errors.Is(err, dislock.ErrNotAcquired):
			s.Metrics.RecordRefreshSkipped(src.BlocklistID, metrics.SkipReasonLockHeld)
			log.Debug().Str("source", src.Name).Msg("Skipping refresh: lock held by another instance")
			return outcomeLockHeld
		case err != nil:
			s.Metrics.RecordLockError(src.BlocklistID)
			log.Err(err).Str("source", src.Name).Msg("Skipping refresh: lock acquisition failed")
			return outcomeFailed
		}
		defer func() {
			_ = lock.Unlock(ctx)
		}()
	}

	meta, err := s.RefreshDue(src)
	if err != nil {
		return outcomeFailed
	}
	if meta == nil {
		return outcomeFresh
	}
	return outcomeProcessed
}

// CatchUp refreshes every source that is due, coordinating with peer
// instances per source. At boot it replaces an unconditional full refresh: on
// a healthy cluster the freshness checks make it a no-op, on a cold database
// it performs the initial full download, and after another instance was down
// it refreshes exactly the sources that went stale. It emits a single summary
// log so a startup can be assessed at a glance, naming any sources that
// failed.
func (s *Service) CatchUp(sources []model.BlocklistMetadata) {
	var failedSources []string
	counts := make(map[refreshOutcome]int, 4)
	for _, src := range sources {
		outcome := s.refreshWithLock(src)
		counts[outcome]++
		if outcome == outcomeFailed {
			failedSources = append(failedSources, src.Name)
		}
	}

	event := log.Info()
	if len(failedSources) > 0 {
		event = log.Warn().Strs("failed_sources", failedSources)
	}
	event.
		Int("processed", counts[outcomeProcessed]).
		Int("fresh", counts[outcomeFresh]).
		Int("lock_held", counts[outcomeLockHeld]).
		Int("failed", counts[outcomeFailed]).
		Int("total", len(sources)).
		Msg("Blocklist startup catch-up complete")
}

// PurgeStaleCoordinated runs PurgeStale under a distributed lock so only one
// instance purges at a time. Contention is normal at rollout time (every
// instance purges at boot); errors fail closed — skipping a purge only delays
// removal until the next boot.
func (s *Service) PurgeStaleCoordinated(sources []model.BlocklistMetadata) {
	ctx := context.Background()

	if s.Locker != nil {
		lock, err := s.Locker.TryLock(ctx, updater.PurgeLockKey, updater.PurgeLockTTL)
		switch {
		case errors.Is(err, dislock.ErrNotAcquired):
			log.Info().Msg("Skipping stale purge: another instance is purging")
			return
		case err != nil:
			log.Err(err).Msg("Skipping stale purge: lock acquisition failed")
			return
		}
		defer func() {
			_ = lock.Unlock(ctx)
		}()
	}

	s.PurgeStale(sources)
}
