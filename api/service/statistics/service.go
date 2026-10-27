package statistics

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/rs/zerolog/log"
)

const (
	purgeBatch          = 1000
	purgeProfileTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	purgeDeadline = 5 * time.Minute

	// RetentionMoveLockPrefix namespaces the per-profile retention-move locks in Redis.
	RetentionMoveLockPrefix = "statistics:retention-move:"
	// RetentionMoveDeadline bounds the move a PATCH starts. Local measurement: ~25k documents/s;
	// a profile's daily documents are far fewer than one deadline's worth even at a tenth of that.
	RetentionMoveDeadline = 10 * time.Minute
	// RetentionMoveLockTTL outlives the longest move (RetentionMoveDeadline, purgeProfileTimeout)
	// plus the undo of its last batch, so no second mover can take the lock mid-batch.
	RetentionMoveLockTTL = 11 * time.Minute
)

// ProfileStatisticsStore loads the statistics settings of many profiles at once and drops
// stale turn-off stamps. A profile that does not exist is absent from the map; one without
// a statistics block maps to nil.
type ProfileStatisticsStore interface {
	GetProfilesStatisticsSettings(ctx context.Context, profileIds []string) (map[string]*model.StatisticsSettings, error)
	UnsetStatisticsDisabledAtBefore(ctx context.Context, before time.Time) (int64, error)
}

type StatisticsService struct {
	StatisticsRepository repository.StatisticsRepository

	profiles         ProfileStatisticsStore
	cache            ReadCache
	moveLocker       *dislock.Locker
	background       func(func())
	now              func() time.Time
	purgeProfileTime time.Duration
	purgeRunTime     time.Duration
}

// Option configures optional StatisticsService dependencies.
type Option func(*StatisticsService)

// WithProfiles supplies the profile lookup PurgeStatistics needs.
func WithProfiles(profiles ProfileStatisticsStore) Option {
	return func(s *StatisticsService) { s.profiles = profiles }
}

// WithBackground replaces how MoveBestEffort starts its background move (a goroutine by default).
func WithBackground(run func(func())) Option {
	return func(s *StatisticsService) { s.background = run }
}

// WithPurgeTimeouts overrides the per-profile bound and the whole-run bound of
// PurgeStatistics.
func WithPurgeTimeouts(perProfile, wholeRun time.Duration) Option {
	return func(s *StatisticsService) {
		s.purgeProfileTime = perProfile
		s.purgeRunTime = wholeRun
	}
}

func NewStatisticsService(db repository.StatisticsRepository, opts ...Option) *StatisticsService {
	s := &StatisticsService{
		StatisticsRepository: db,
		cache:                noopReadCache{},
		background:           func(f func()) { go f() },
		now:                  time.Now,
		purgeProfileTime:     purgeProfileTimeout,
		purgeRunTime:         purgeDeadline,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SetRetentionMoveLocker serialises retention moves of one profile across instances
// (api-endpoint-behaviour.md J50). Without it moves run unlocked.
func (s *StatisticsService) SetRetentionMoveLocker(l *dislock.Locker) { s.moveLocker = l }

// MoveToRetention leaves the profile's daily statistics only in collections no longer than
// retention, keeping the days the retention still covers (J50). A move already running on
// another instance is skipped. It returns the number of documents copied.
func (s *StatisticsService) MoveToRetention(ctx context.Context, profileId string, retention model.StatisticsRetention) (int, error) {
	moved, err := s.moveToRetention(ctx, profileId, retention)
	s.invalidate(ctx, profileId)
	return moved, err
}

func (s *StatisticsService) moveToRetention(ctx context.Context, profileId string, retention model.StatisticsRetention) (int, error) {
	if s.moveLocker != nil {
		lock, err := s.moveLocker.TryLock(ctx, profileId, RetentionMoveLockTTL)
		if errors.Is(err, dislock.ErrNotAcquired) {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		defer func() { _ = lock.Unlock(context.WithoutCancel(ctx)) }()
	}
	since := s.now().UTC().Add(-retention.Window()).Truncate(24 * time.Hour)
	return s.StatisticsRepository.MoveProfileDailyStatistics(ctx, profileId, retention.OrDefault(), since)
}

// MoveBestEffort starts the move on lowering the retention in the background, so the PATCH does
// not wait for it; the next statistics purge repeats it for anything left.
func (s *StatisticsService) MoveBestEffort(ctx context.Context, profileId string, retention model.StatisticsRetention) {
	moveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), RetentionMoveDeadline)
	s.background(func() {
		defer cancel()
		if _, err := s.MoveToRetention(moveCtx, profileId, retention); err != nil {
			log.Ctx(moveCtx).Error().Err(err).Msg("statistics: retention move failed; the next statistics purge retries it")
		}
	})
}

// InvalidateReadCache drops the profile's cached statistics answers; a failure is only logged.
func (s *StatisticsService) InvalidateReadCache(ctx context.Context, profileId string) {
	s.invalidate(ctx, profileId)
}

// PurgeProfile deletes all of the profile's statistics.
func (s *StatisticsService) PurgeProfile(ctx context.Context, profileId string) error {
	_, err := s.StatisticsRepository.DeleteProfileStatistics(ctx, profileId, nil)
	s.invalidate(ctx, profileId)
	return err
}

// PurgeBestEffort is the immediate purge on statistics true->false and on profile
// delete. A failure is only logged: statistics reads are gated on the setting and
// the next statistics purge removes whatever is left.
func (s *StatisticsService) PurgeBestEffort(ctx context.Context, profileId string) {
	if err := s.PurgeProfile(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("statistics: immediate purge failed; the next statistics purge removes the leftovers")
	}
}

// PurgeBound returns what to delete for one profile id found in the statistics
// collections: nil means everything (profile missing or statistics off); otherwise
// enabled_at, which the repository floors to each tier's bucket width, so every tier
// keeps the bucket containing enabled_at and removes older ones (leftovers of an
// earlier on-period). Turning statistics on always sets enabled_at.
func PurgeBound(exists bool, settings *model.StatisticsSettings) *time.Time {
	if !exists || settings == nil || !settings.Enabled {
		return nil
	}
	return settings.EnabledAt
}

// StatisticsPurgeResult counts what one run did; it carries no identifiers.
// Deleted counts documents; Moved counts documents copied to a shorter retention.
type StatisticsPurgeResult struct {
	Checked, Deleted, Moved, Failed int
}

// PurgeStatistics makes the stored statistics conform to each profile's current
// settings (api-endpoint-behaviour.md J8, J9): for every profile id present in the
// collections it deletes what the profile no longer keeps (PurgeBound, then
// history_deleted_at) and moves daily documents down to the current retention. Everything
// is derived from Mongo, so a run needs no memory of earlier ones. A profile's read cache
// is dropped only when its stored documents changed. The first per-profile timeout ends
// the run (the database is struggling) and so does the whole-run deadline; profiles not
// reached are picked up by the next run.
func (s *StatisticsService) PurgeStatistics(ctx context.Context) (StatisticsPurgeResult, error) {
	var res StatisticsPurgeResult
	if s.profiles == nil {
		return res, errors.New("statistics purge: no profile reader configured")
	}

	runCtx, cancel := context.WithTimeout(ctx, s.purgeRunTime)
	defer cancel()

	// G22: disabled_at only matters until the end of its UTC day; after that its ceiling cannot
	// reach a bucket of a later turn-on, so it is not kept.
	if n, err := s.profiles.UnsetStatisticsDisabledAtBefore(runCtx, s.now().UTC().Truncate(24*time.Hour)); err != nil {
		log.Ctx(ctx).Err(err).Msg("Statistics purge: failed to drop stale disabled_at")
	} else if n > 0 {
		log.Ctx(ctx).Debug().Int64("profiles", n).Msg("Statistics purge: dropped stale disabled_at")
	}

	ids, err := s.StatisticsRepository.ListStatisticsProfileIDs(runCtx)
	if err != nil {
		return res, err
	}

	for start := 0; start < len(ids); start += purgeBatch {
		batch := ids[start:min(start+purgeBatch, len(ids))]
		settings, err := s.profiles.GetProfilesStatisticsSettings(runCtx, batch)
		if err != nil {
			return res, err
		}

		for _, id := range batch {
			if runCtx.Err() != nil {
				return res, nil
			}
			res.Checked++
			st, exists := settings[id]

			jobCtx, cancelJob := context.WithTimeout(runCtx, s.purgeProfileTime)
			deleted, moved, jobErr := s.purgeProfile(jobCtx, id, exists, st)
			timedOut := errors.Is(jobCtx.Err(), context.DeadlineExceeded)
			cancelJob()
			res.Deleted += int(deleted)
			res.Moved += moved
			if deleted > 0 || moved > 0 {
				s.invalidate(runCtx, id)
			}
			if jobErr != nil {
				res.Failed++
				if timedOut {
					return res, nil
				}
			}
		}
	}
	return res, nil
}

func (s *StatisticsService) purgeProfile(ctx context.Context, profileId string, exists bool, st *model.StatisticsSettings) (int64, int, error) {
	deleted, err := s.StatisticsRepository.DeleteProfileStatistics(ctx, profileId, PurgeBound(exists, st))
	if err != nil || !exists || st == nil || !st.Enabled {
		return deleted, 0, err
	}
	if st.HistoryDeletedAt != nil {
		n, err := s.StatisticsRepository.DeleteProfileStatisticsThrough(ctx, profileId, *st.HistoryDeletedAt)
		deleted += n
		if err != nil {
			return deleted, 0, err
		}
	}
	moved, err := s.moveToRetention(ctx, profileId, st.Retention)
	return deleted, moved, err
}
