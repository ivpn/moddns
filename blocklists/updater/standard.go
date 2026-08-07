package updater

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/rs/zerolog/log"
)

const (
	// LockKeyPrefix namespaces every blocklists coordination key in Redis.
	LockKeyPrefix = "blocklists:lock:"
	// SourceLockTTL is the expiry of a per-source lock. It must comfortably
	// exceed the longest single-source runtime (service processingTimeout is
	// 2m) so the lock survives one full update, while bounding how long a
	// crashed holder blocks peers.
	SourceLockTTL = 10 * time.Minute
	// PurgeLockTTL is the expiry of the stale-purge lock.
	PurgeLockTTL = 5 * time.Minute

	sourceLockKeyPrefix = "source:"
	// PurgeLockKey serializes PurgeStale across instances.
	PurgeLockKey = "purge"
)

// SourceLockKey returns the lock key coordinating updates of one source. It
// is also each cron job's name, so the key gocron hands the distributed
// locker is identical to the one the boot-time catch-up acquires explicitly.
func SourceLockKey(blocklistID string) string {
	return sourceLockKeyPrefix + blocklistID
}

// NewDistributedLocker adapts a dislock.Locker to the gocron.Locker
// interface, so the scheduler acquires the per-source lock before each tick
// and silently skips the run when a peer instance already holds it. Lock
// outcomes are recorded on m here because gocron skips the job body — the
// only place a lost tick is observable is the locker itself.
func NewDistributedLocker(locker *dislock.Locker, m metrics.Updates) gocron.Locker {
	if m == nil {
		m = metrics.NoopUpdates{}
	}
	return &gocronLocker{locker: locker, metrics: m}
}

type gocronLocker struct {
	locker  *dislock.Locker
	metrics metrics.Updates
}

func (g *gocronLocker) Lock(ctx context.Context, key string) (gocron.Lock, error) {
	lock, err := g.locker.TryLock(ctx, key, SourceLockTTL)
	if err != nil {
		source := strings.TrimPrefix(key, sourceLockKeyPrefix)
		if errors.Is(err, dislock.ErrNotAcquired) {
			g.metrics.RecordRefreshSkipped(source, metrics.SkipReasonLockHeld)
			log.Debug().Str("source", source).Msg("Skipping scheduled refresh: lock held by another instance")
		} else {
			g.metrics.RecordLockError(source)
			log.Err(err).Str("source", source).Msg("Skipping scheduled refresh: lock acquisition failed")
		}
		return nil, err
	}
	return lock, nil
}

// StandardUpdater schedules per-source refresh jobs on a gocron scheduler.
// With a distributed locker installed, every instance runs the identical
// schedules and the locker picks one winner per source per tick.
type StandardUpdater struct {
	scheduler gocron.Scheduler
}

// NewStandardUpdater creates the scheduler. locker may be nil (tests,
// single-instance dev), in which case jobs run unconditionally.
func NewStandardUpdater(locker gocron.Locker) (*StandardUpdater, error) {
	var opts []gocron.SchedulerOption
	if locker != nil {
		opts = append(opts, gocron.WithDistributedLocker(locker))
	}
	s, err := gocron.NewScheduler(opts...)
	if err != nil {
		return nil, err
	}
	return &StandardUpdater{
		scheduler: s,
	}, nil
}

// Setup adds a new, single blocklist to the scheduler. The job name doubles
// as the distributed-lock key; without an explicit name every job would fall
// back to the shared closure's function name and all sources would contend
// on a single lock.
func (u *StandardUpdater) Setup(source model.BlocklistMetadata, blocklistFunc func() (*model.BlocklistMetadata, error)) error {
	job, err := u.scheduler.NewJob(
		gocron.CronJob(source.Schedule, false),
		gocron.NewTask(func() {
			start := time.Now()
			log.Debug().Str("source", source.Name).Msg("Processing blocklist")
			meta, err := blocklistFunc()
			if err != nil {
				log.Err(err).Str("blocklist_id", source.BlocklistID).Str("source", source.Name).Msg("Failed to process blocklist")
				return
			}
			if meta == nil {
				// Refreshed moments ago by a peer instance; nothing downloaded.
				log.Debug().Str("source", source.Name).Msg("Skipped scheduled refresh: source is fresh")
				return
			}

			log.Info().
				Str("source", source.Name).
				Dur("duration", time.Duration(time.Since(start).Milliseconds())).
				Int("entries", meta.Entries).
				Msg("Blocklist refresh complete")
		}),
		gocron.WithName(SourceLockKey(source.BlocklistID)),
	)
	if err != nil {
		log.Err(err).Str("source", source.Name).Msg("Failed to add source to cron")
		return err
	}
	log.Info().Str("source", source.Name).Str("job_id", job.ID().String()).Msg("Added source to cron")
	return nil
}

// Start starts the cron scheduler
func (u *StandardUpdater) Start() {
	u.scheduler.Start()
}

// Erase removes all cron entries
func (u *StandardUpdater) Erase() {
	log.Info().Msg("Erasing standard updater cron entries")
	for _, job := range u.scheduler.Jobs() {
		if err := u.scheduler.RemoveJob(job.ID()); err != nil {
			log.Err(err).Str("job", job.Name()).Msg("Failed to remove cron entry")
		}
	}
}

// Stop stops the cron scheduler, waiting for running jobs to finish
func (u *StandardUpdater) Stop() {
	log.Info().Msg("Stopping standard updater")
	if err := u.scheduler.Shutdown(); err != nil {
		log.Err(err).Msg("Failed to shut down scheduler")
	}
}
