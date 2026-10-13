package statistics

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// ReadCache is the cache-aside store of GetProfileStatistics answers. Entries are keyed by
// profile, generation and timespan and expire on their own (api/cache).
type ReadCache interface {
	// Generation returns the profile's current generation; a read uses the value it saw
	// before querying for both its lookup and its write.
	Generation(ctx context.Context, profileId string) (int64, error)
	GetStatistics(ctx context.Context, profileId, timespan string, gen int64) (payload []byte, found bool, err error)
	SetStatistics(ctx context.Context, profileId, timespan string, gen int64, payload []byte) error
	// InvalidateStatistics moves the profile to a new generation.
	InvalidateStatistics(ctx context.Context, profileId string) error
}

type noopReadCache struct{}

func (noopReadCache) Generation(context.Context, string) (int64, error) { return 0, nil }
func (noopReadCache) GetStatistics(context.Context, string, string, int64) ([]byte, bool, error) {
	return nil, false, nil
}
func (noopReadCache) SetStatistics(context.Context, string, string, int64, []byte) error { return nil }
func (noopReadCache) InvalidateStatistics(context.Context, string) error                 { return nil }

// WithReadCache enables the cache-aside layer of GetProfileStatistics.
func WithReadCache(c ReadCache) Option {
	return func(s *StatisticsService) { s.cache = c }
}

// WithClock overrides the time source of GetProfileStatistics.
func WithClock(now func() time.Time) Option {
	return func(s *StatisticsService) { s.now = now }
}

// ReadOption adjusts one GetProfileStatistics call.
type ReadOption func(*readOptions)

type readOptions struct {
	missGuard func() error
}

// WithMissGuard runs guard just before the database would be queried (never on a cache hit or
// for a disabled profile); its error is returned and nothing is queried.
func WithMissGuard(guard func() error) ReadOption {
	return func(o *readOptions) { o.missGuard = guard }
}

// statisticsRetention is the retention window the read is clamped to (J41, J48).
func statisticsRetention(settings *model.StatisticsSettings) (string, time.Duration) {
	if settings == nil {
		return model.StatisticsRetentionWindow("")
	}
	return model.StatisticsRetentionWindow(settings.Retention)
}

// ComputeWindow returns the read window [from, to): to is now in UTC, from is
// the earlier of now-window and now-retention clamped to the retention, floored
// to the series bucket. Bucket boundaries are epoch-aligned, hence DST-free.
func ComputeWindow(now time.Time, spec model.StatisticsTimespanSpec, retention time.Duration) (from, to time.Time) {
	to = now.UTC()
	reach := min(spec.Window, retention)
	return to.Add(-reach).Truncate(spec.Bucket), to
}

// GetProfileStatistics answers GET /profiles/{id}/statistics (spec rows J4, J40-J45).
// settings are the profile's current statistics settings; with statistics off
// nothing is queried or cached.
func (s *StatisticsService) GetProfileStatistics(ctx context.Context, profileId, timespan string, settings *model.StatisticsSettings, opts ...ReadOption) (*model.StatisticsResponse, error) {
	var o readOptions
	for _, opt := range opts {
		opt(&o)
	}
	spec, err := model.NewStatisticsTimespan(timespan)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC().Truncate(time.Second)
	retentionLabel, retention := statisticsRetention(settings)
	from, to := ComputeWindow(now, spec, retention)

	resp := &model.StatisticsResponse{
		Retention:     retentionLabel,
		Timespan:      timespan,
		From:          from,
		To:            to,
		BucketSeconds: int(spec.Bucket / time.Second),
		Series:        []model.StatisticsPoint{},
		Devices:       []model.StatisticsDevice{},
	}
	if settings == nil || !settings.Enabled {
		return resp, nil
	}
	resp.Enabled = true
	resp.EnabledAt = settings.EnabledAt
	resp.HistoryDeletedAt = settings.HistoryDeletedAt

	gen, cacheable := s.cacheGeneration(ctx, profileId)
	if cacheable {
		if cached := s.readCached(ctx, profileId, timespan, gen); cached != nil {
			return cached, nil
		}
	}

	queryFrom := keepSince(from, settings.HistoryDeletedAt, spec.Bucket)
	if !queryFrom.Before(to) {
		resp.Series = fillSeries(nil, from, to, spec.Bucket)
		return resp, nil
	}
	if o.missGuard != nil {
		if err := o.missGuard(); err != nil {
			return nil, err
		}
	}
	agg, err := s.StatisticsRepository.GetProfileStatistics(ctx, profileId, spec.Tier, queryFrom, to, spec.Bucket)
	if err != nil {
		return nil, err
	}
	resp.Totals = agg.Totals
	resp.Reasons = agg.Reasons
	resp.Protocols = agg.Protocols
	resp.Series = fillSeries(agg.Series, from, to, spec.Bucket)
	resp.Devices = FoldDevices(agg.Devices, agg.Totals)

	// An empty answer is not cached so a freshly enabled profile shows its first buckets at once.
	if cacheable && resp.Totals.Total > 0 {
		s.writeCached(ctx, profileId, timespan, gen, resp)
	}
	return resp, nil
}

// keepSince moves the query start past the bucket containing a history delete: that
// bucket also holds counts from before it (J53).
func keepSince(from time.Time, deletedAt *time.Time, bucket time.Duration) time.Time {
	if deletedAt == nil {
		return from
	}
	cutoff := deletedAt.UTC().Truncate(bucket)
	if cutoff.Before(deletedAt.UTC()) {
		cutoff = cutoff.Add(bucket)
	}
	if cutoff.After(from) {
		return cutoff
	}
	return from
}

// cacheGeneration returns the generation the read uses; false (cache skipped both ways) when it
// cannot be read, since a guessed generation could serve an answer from before a purge.
func (s *StatisticsService) cacheGeneration(ctx context.Context, profileId string) (int64, bool) {
	gen, err := s.cache.Generation(ctx, profileId)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: cache generation unreadable; reading from the database")
		return 0, false
	}
	return gen, true
}

func (s *StatisticsService) readCached(ctx context.Context, profileId, timespan string, gen int64) *model.StatisticsResponse {
	payload, found, err := s.cache.GetStatistics(ctx, profileId, timespan, gen)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: cache read failed; reading from the database")
		return nil
	}
	if !found {
		return nil
	}
	var resp model.StatisticsResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: undecodable cache entry ignored")
		return nil
	}
	return &resp
}

func (s *StatisticsService) writeCached(ctx context.Context, profileId, timespan string, gen int64, resp *model.StatisticsResponse) {
	payload, err := json.Marshal(resp)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: cache entry not encoded")
		return
	}
	if err := s.cache.SetStatistics(ctx, profileId, timespan, gen, payload); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: cache write failed")
	}
}

// invalidate drops the profile's cached answers; a failure is only logged
// because every entry expires within its TTL anyway.
func (s *StatisticsService) invalidate(ctx context.Context, profileId string) {
	if err := s.cache.InvalidateStatistics(ctx, profileId); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("statistics: cache invalidation failed")
	}
}

// fillSeries lays the sparse repository buckets onto the full [from, to) grid,
// oldest first; buckets outside the grid are dropped.
func fillSeries(sparse []model.StatisticsPoint, from, to time.Time, bucket time.Duration) []model.StatisticsPoint {
	byStart := make(map[int64]model.StatisticsPoint, len(sparse))
	for _, p := range sparse {
		byStart[p.Ts.UTC().Unix()] = p
	}
	series := make([]model.StatisticsPoint, 0, int(to.Sub(from)/bucket)+1)
	for ts := from; ts.Before(to); ts = ts.Add(bucket) {
		p := byStart[ts.Unix()]
		p.Ts = ts
		series = append(series, p)
	}
	return series
}

// FoldDevices orders devices by total (desc) then id and, past
// model.StatisticsMaxDevices entries, keeps the largest ones other than
// model.StatisticsOtherDeviceID and folds everything else into that entry. The
// list may be cut short by the repository, so the folded entry is all minus what
// is kept, all being the totals over every device.
func FoldDevices(devices []model.StatisticsDevice, all model.StatisticsTotals) []model.StatisticsDevice {
	out := make([]model.StatisticsDevice, len(devices))
	copy(out, devices)
	sortDevices(out)
	if len(out) <= model.StatisticsMaxDevices {
		return out
	}

	other := model.StatisticsDevice{DeviceID: model.StatisticsOtherDeviceID, Total: all.Total, Blocked: all.Blocked}
	kept := make([]model.StatisticsDevice, 0, model.StatisticsMaxDevices)
	for _, d := range out {
		if d.DeviceID != model.StatisticsOtherDeviceID && len(kept) < model.StatisticsMaxDevices-1 {
			kept = append(kept, d)
			other.Total -= d.Total
			other.Blocked -= d.Blocked
		}
	}
	kept = append(kept, other)
	sortDevices(kept)
	return kept
}

func sortDevices(ds []model.StatisticsDevice) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].Total != ds[j].Total {
			return ds[i].Total > ds[j].Total
		}
		return ds[i].DeviceID < ds[j].DeviceID
	})
}
