package statistics

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

const (
	unconsentedPurgeBatch      = 1000
	unconsentedPurgeJobTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	unconsentedPurgeDeadline = 5 * time.Minute
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

// UnconsentedPurgeBound decides what to delete for one profile id found in the
// statistics collections: nothing, everything (nil before), or the buckets
// starting before the returned instant.
//   - profile missing or statistics off: everything.
//   - statistics on with enabled_at: enabled_at itself is the bound; the
//     repository floors it to each tier's bucket width, so every tier keeps the
//     bucket containing enabled_at and removes older ones (leftovers of an
//     earlier on-period).
//   - statistics on without enabled_at (enabled before the field existed): nothing.
func UnconsentedPurgeBound(exists bool, settings *model.StatisticsSettings) (purge bool, before *time.Time) {
	if !exists || settings == nil || !settings.Enabled {
		return true, nil
	}
	if settings.EnabledAt == nil {
		return false, nil
	}
	b := settings.EnabledAt.UTC()
	return true, &b
}

// UnconsentedPurgeResult counts what one run did; it carries no identifiers.
type UnconsentedPurgeResult struct {
	Checked, Purged, Failed int
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
			purge, before := UnconsentedPurgeBound(exists, st)
			if !purge {
				continue
			}

			jobCtx, cancelJob := context.WithTimeout(runCtx, s.purgeJobTime)
			delErr := s.StatisticsRepository.DeleteProfileStatistics(jobCtx, id, before)
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
