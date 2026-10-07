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
	unconsentedPurgeBatch      = 1000
	unconsentedPurgeJobTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	unconsentedPurgeDeadline = 5 * time.Minute

	// RetentionMoveLockPrefix namespaces the per-profile retention-move locks in Redis.
	RetentionMoveLockPrefix = "statistics:retention-move:"
	// Longer than one move is allowed to run (unconsentedPurgeJobTimeout).
	retentionMoveLockTTL = 2 * time.Minute
)

// ProfileStatisticsReader loads the statistics settings of many profiles at once.
// A profile that does not exist is absent from the map; one without a statistics
// block maps to nil.
type ProfileStatisticsReader interface {
	GetProfilesStatisticsSettings(ctx context.Context, profileIds []string) (map[string]*model.StatisticsSettings, error)
}

type StatisticsService struct {
	StatisticsRepository repository.StatisticsRepository

	profiles     ProfileStatisticsReader
	cache        ReadCache
	moveLocker   *dislock.Locker
	now          func() time.Time
	purgeJobTime time.Duration
	purgeRunTime time.Duration
}

// Option configures optional StatisticsService dependencies.
type Option func(*StatisticsService)

// WithProfiles supplies the profile lookup PurgeUnconsentedStatistics needs.
func WithProfiles(profiles ProfileStatisticsReader) Option {
	return func(s *StatisticsService) { s.profiles = profiles }
}

// WithUnconsentedPurgeTimeouts overrides the per-profile delete bound and the whole-run
// bound of PurgeUnconsentedStatistics.
func WithUnconsentedPurgeTimeouts(perProfile, wholeRun time.Duration) Option {
	return func(s *StatisticsService) {
		s.purgeJobTime = perProfile
		s.purgeRunTime = wholeRun
	}
}

func NewStatisticsService(db repository.StatisticsRepository, opts ...Option) *StatisticsService {
	s := &StatisticsService{
		StatisticsRepository: db,
		cache:                noopReadCache{},
		now:                  time.Now,
		purgeJobTime:         unconsentedPurgeJobTimeout,
		purgeRunTime:         unconsentedPurgeDeadline,
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
		lock, err := s.moveLocker.TryLock(ctx, profileId, retentionMoveLockTTL)
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

// MoveBestEffort is the immediate move on lowering the retention; the next
// unconsented-statistics purge repeats it for anything left.
func (s *StatisticsService) MoveBestEffort(ctx context.Context, profileId string, retention model.StatisticsRetention) {
	if _, err := s.MoveToRetention(ctx, profileId, retention); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("statistics: retention move failed; the next unconsented-statistics purge retries it")
	}
}

// PurgeProfile deletes all of the profile's statistics.
func (s *StatisticsService) PurgeProfile(ctx context.Context, profileId string) error {
	err := s.StatisticsRepository.DeleteProfileStatistics(ctx, profileId, nil)
	s.invalidate(ctx, profileId)
	return err
}

// PurgeBestEffort is the immediate purge on statistics true->false and on profile
// delete. A failure is only logged: statistics reads are gated on the setting and
// the next unconsented-statistics purge removes whatever is left.
func (s *StatisticsService) PurgeBestEffort(ctx context.Context, profileId string) {
	if err := s.PurgeProfile(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("statistics: immediate purge failed; the next unconsented-statistics purge removes the leftovers")
	}
}

// UnconsentedPurgeBound returns what to delete for one profile id found in the
// statistics collections: nil means everything (profile missing or statistics
// off); otherwise enabled_at, which the repository floors to each tier's bucket
// width, so every tier keeps the bucket containing enabled_at and removes older
// ones (leftovers of an earlier on-period). Turning statistics on always sets
// enabled_at.
func UnconsentedPurgeBound(exists bool, settings *model.StatisticsSettings) *time.Time {
	if !exists || settings == nil || !settings.Enabled {
		return nil
	}
	return settings.EnabledAt
}

// UnconsentedPurgeResult counts what one run did; it carries no identifiers.
type UnconsentedPurgeResult struct {
	Checked, Purged, Failed, Moved int
}

// PurgeUnconsentedStatistics removes statistics that no longer have consent: for every
// profile id present in the statistics collections it applies UnconsentedPurgeBound
// against the profile as stored now. Everything is derived from Mongo, so a run
// needs no memory of earlier ones. The first per-profile timeout ends the run
// (the database is struggling) and so does the whole-run deadline; profiles not
// reached are picked up by the next run.
func (s *StatisticsService) PurgeUnconsentedStatistics(ctx context.Context) (UnconsentedPurgeResult, error) {
	var res UnconsentedPurgeResult
	if s.profiles == nil {
		return res, errors.New("unconsented-statistics purge: no profile reader configured")
	}

	runCtx, cancel := context.WithTimeout(ctx, s.purgeRunTime)
	defer cancel()

	ids, err := s.StatisticsRepository.ListStatisticsProfileIDs(runCtx)
	if err != nil {
		return res, err
	}

	for start := 0; start < len(ids); start += unconsentedPurgeBatch {
		batch := ids[start:min(start+unconsentedPurgeBatch, len(ids))]
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
			before := UnconsentedPurgeBound(exists, st)

			jobCtx, cancelJob := context.WithTimeout(runCtx, s.purgeJobTime)
			delErr := s.StatisticsRepository.DeleteProfileStatistics(jobCtx, id, before)
			if delErr == nil && exists && st != nil && st.Enabled && st.HistoryDeletedAt != nil {
				delErr = s.StatisticsRepository.DeleteProfileStatisticsThrough(jobCtx, id, *st.HistoryDeletedAt)
			}
			if delErr == nil && exists && st != nil && st.Enabled {
				var moved int
				moved, delErr = s.moveToRetention(jobCtx, id, st.Retention)
				res.Moved += moved
			}
			s.invalidate(runCtx, id)
			timedOut := errors.Is(jobCtx.Err(), context.DeadlineExceeded)
			cancelJob()
			if delErr != nil {
				res.Failed++
				if timedOut {
					return res, nil
				}
				continue
			}
			res.Purged++
		}
	}
	return res, nil
}
