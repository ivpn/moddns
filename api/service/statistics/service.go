package statistics

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// DefaultProxySettingsTTL mirrors the proxy's PROFILE_SETTINGS_CACHE_TTL default.
const DefaultProxySettingsTTL = 30 * time.Second

// PurgeTiming feeds PurgeSchedule. ProxySettingsTTL must be >= the proxy's
// PROFILE_SETTINGS_CACHE_TTL. A non-zero DelayOverride replaces the derived due
// time (now + DelayOverride) and exists for end-to-end tests only.
type PurgeTiming struct {
	ProxySettingsTTL time.Duration
	DelayOverride    time.Duration
}

type StatisticsService struct {
	StatisticsRepository repository.StatisticsRepository

	queue  cache.StatisticsPurgeQueue
	timing PurgeTiming
}

// Option configures optional StatisticsService dependencies.
type Option func(*StatisticsService)

// WithPurgeQueue enables the delayed purge pass and retry jobs.
func WithPurgeQueue(queue cache.StatisticsPurgeQueue, timing PurgeTiming) Option {
	return func(s *StatisticsService) {
		s.queue = queue
		s.timing = timing
		if s.timing.ProxySettingsTTL <= 0 {
			s.timing.ProxySettingsTTL = DefaultProxySettingsTTL
		}
	}
}

func NewStatisticsService(db repository.StatisticsRepository, opts ...Option) *StatisticsService {
	s := &StatisticsService{
		StatisticsRepository: db,
		timing:               PurgeTiming{ProxySettingsTTL: DefaultProxySettingsTTL},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *StatisticsService) GetProfileStatistics(ctx context.Context, profileId string, timespan string) ([]model.StatisticsAggregated, error) {
	timespanHours, err := model.NewTimespan(timespan)
	if err != nil {
		return nil, err
	}

	stats, err := s.StatisticsRepository.GetProfileStatistics(ctx, profileId, timespanHours)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// PurgeProfile deletes all of the profile's statistics.
func (s *StatisticsService) PurgeProfile(ctx context.Context, profileId string) error {
	return s.StatisticsRepository.DeleteProfileStatistics(ctx, profileId, nil)
}

func (s *StatisticsService) schedule(now time.Time) (before, due time.Time) {
	before, due = PurgeSchedule(now, s.timing.ProxySettingsTTL)
	if s.timing.DelayOverride > 0 {
		due = now.Add(s.timing.DelayOverride)
	}
	return before, due
}

// enqueue queues the delayed pass that catches buckets the proxy flushes after
// the immediate purge. Its due time comes from the cutoff window (or the test
// override), so a repeat in the same window is the same member at the same score.
func (s *StatisticsService) enqueue(ctx context.Context, job PurgeJob, due time.Time) error {
	if s.queue == nil {
		return nil
	}
	return s.queue.EnqueueStatisticsPurge(ctx, job.Member(), due)
}

// PurgeOnDisable handles statistics true->false: an immediate purge, then the
// queued pass bounded by the cutoff. A failed immediate purge is covered by that
// pass; statistics reads are already gated on the setting, so nothing is
// surfaced to the caller.
func (s *StatisticsService) PurgeOnDisable(ctx context.Context, profileId string, now time.Time) {
	if err := s.PurgeProfile(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("statistics: immediate purge failed; the queued pass will retry")
	}
	before, due := s.schedule(now)
	job := PurgeJob{Kind: PurgeKindDisable, ProfileID: profileId, Before: &before}
	if err := s.enqueue(ctx, job, due); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("statistics: failed to queue delayed purge")
	}
}

// SchedulePurgeForDelete queues the unbounded pass for a profile about to be
// deleted. Call it before deleting anything so a failed deletion cannot leave
// statistics without a purge job.
func (s *StatisticsService) SchedulePurgeForDelete(ctx context.Context, profileId string, now time.Time) error {
	_, due := s.schedule(now)
	return s.enqueue(ctx, PurgeJob{Kind: PurgeKindDelete, ProfileID: profileId}, due)
}

// PurgeForDelete deletes the profile's statistics now. A failure is covered by
// the pass queued by SchedulePurgeForDelete; it is returned only when no queue
// is configured.
func (s *StatisticsService) PurgeForDelete(ctx context.Context, profileId string) error {
	err := s.PurgeProfile(ctx, profileId)
	if err == nil || s.queue == nil {
		return err
	}
	log.Ctx(ctx).Error().Err(err).Msg("statistics: purge on profile delete failed; the queued pass will retry")
	return nil
}
