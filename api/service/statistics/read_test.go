package statistics_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var readNow = utc(2026, 10, 5, 12, 7, 31)

// fakeReadCache is an in-memory statistics.ReadCache that records its traffic.
type fakeReadCache struct {
	mu                       sync.Mutex
	data                     map[string][]byte
	gets, sets               int
	invalidated              []string
	getErr, setErr, invalErr error
}

func newFakeReadCache() *fakeReadCache { return &fakeReadCache{data: map[string][]byte{}} }

func (f *fakeReadCache) GetStatistics(_ context.Context, profileID, timespan string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	b, ok := f.data[profileID+"|"+timespan]
	return b, ok, nil
}

func (f *fakeReadCache) SetStatistics(_ context.Context, profileID, timespan string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets++
	if f.setErr != nil {
		return f.setErr
	}
	f.data[profileID+"|"+timespan] = payload
	return nil
}

func (f *fakeReadCache) InvalidateStatistics(_ context.Context, profileID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, profileID)
	return f.invalErr
}

func readService(t *testing.T, repo *mocks.StatisticsRepository, cache statistics.ReadCache) *statistics.StatisticsService {
	t.Helper()
	opts := []statistics.Option{statistics.WithClock(func() time.Time { return readNow })}
	if cache != nil {
		opts = append(opts, statistics.WithReadCache(cache))
	}
	return statistics.NewStatisticsService(repo, opts...)
}

func enabledSettings() *model.StatisticsSettings {
	at := utc(2026, 9, 1, 8, 0, 0)
	return stats(true, &at)
}

// specRef: api-endpoint-behaviour.md J40, J41 — tier and point width per timespan, floored from, series covers [from, to); views longer than the 30d retention are clamped.
func TestGetProfileStatistics_TimespanBuckets(t *testing.T) {
	tests := []struct {
		timespan   string
		tier       model.StatisticsTier
		bucketSecs int
		from       time.Time
		points     int
	}{
		{model.StatisticsLast3Hours, model.StatisticsTier15Min, 900, utc(2026, 10, 5, 9, 0, 0), 13},
		{model.StatisticsLast6Hours, model.StatisticsTier15Min, 900, utc(2026, 10, 5, 6, 0, 0), 25},
		{model.LAST_1_DAY, model.StatisticsTier1H, 3600, utc(2026, 10, 4, 12, 0, 0), 25},
		{model.LAST_7_DAYS, model.StatisticsTier1H, 3600, utc(2026, 9, 28, 12, 0, 0), 169},
		{model.LAST_MONTH, model.StatisticsTier1D, 86400, utc(2026, 9, 5, 0, 0, 0), 31},
		// 3 months and 1 year are clamped to the 30-day retention until the retention setting exists.
		{model.StatisticsLast3Months, model.StatisticsTier1D, 86400, utc(2026, 9, 5, 0, 0, 0), 31},
		{model.StatisticsLastYear, model.StatisticsTier1D, 86400, utc(2026, 9, 5, 0, 0, 0), 31},
	}
	for _, tt := range tests {
		t.Run(tt.timespan, func(t *testing.T) {
			repo := mocks.NewStatisticsRepository(t)
			repo.On("GetProfileStatistics", mock.Anything, "p1", tt.tier,
				mock.MatchedBy(func(from time.Time) bool { return from.Equal(tt.from) }),
				mock.MatchedBy(func(to time.Time) bool { return to.Equal(readNow) }),
				time.Duration(tt.bucketSecs)*time.Second,
			).Return(&model.StatisticsAggregate{}, nil).Once()

			got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", tt.timespan, enabledSettings())
			require.NoError(t, err)
			require.True(t, got.Enabled)
			require.Equal(t, tt.timespan, got.Timespan)
			require.Equal(t, tt.bucketSecs, got.BucketSeconds)
			require.True(t, got.From.Equal(tt.from), "from=%s", got.From)
			require.True(t, got.To.Equal(readNow), "to=%s", got.To)
			require.Equal(t, "30d", got.Retention)
			require.Len(t, got.Series, tt.points)
			require.True(t, got.Series[0].Ts.Equal(tt.from))
			step := time.Duration(tt.bucketSecs) * time.Second
			for i := 1; i < len(got.Series); i++ {
				require.True(t, got.Series[i].Ts.Equal(got.Series[i-1].Ts.Add(step)))
			}
			require.True(t, got.Series[len(got.Series)-1].Ts.Before(readNow))
			require.False(t, got.Series[len(got.Series)-1].Ts.Add(step).Before(readNow))
		})
	}
}

// specRef: api-endpoint-behaviour.md J41, J48 — the read is clamped to the profile's retention; an empty or unknown value reads as 30d.
func TestGetProfileStatistics_ClampsToProfileRetention(t *testing.T) {
	for _, tc := range []struct {
		retention model.StatisticsRetention
		label     string
		from      time.Time
		points    int
	}{
		{"", "30d", utc(2026, 9, 5, 0, 0, 0), 31},
		{"bogus", "30d", utc(2026, 9, 5, 0, 0, 0), 31},
		{model.StatisticsRetention90d, "90d", utc(2026, 7, 7, 0, 0, 0), 91},
		{model.StatisticsRetention1y, "1y", utc(2025, 10, 5, 0, 0, 0), 366},
	} {
		t.Run(tc.label+"/"+string(tc.retention), func(t *testing.T) {
			settings := enabledSettings()
			settings.Retention = tc.retention
			repo := mocks.NewStatisticsRepository(t)
			repo.On("GetProfileStatistics", mock.Anything, "p1", model.StatisticsTier1D,
				mock.MatchedBy(func(from time.Time) bool { return from.Equal(tc.from) }),
				mock.Anything, 24*time.Hour,
			).Return(&model.StatisticsAggregate{}, nil).Once()

			got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", model.StatisticsLastYear, settings)
			require.NoError(t, err)
			require.Equal(t, tc.label, got.Retention)
			require.True(t, got.From.Equal(tc.from), "from=%s", got.From)
			require.Len(t, got.Series, tc.points)
		})
	}
}

// specRef: api-endpoint-behaviour.md J40 — an unknown timespan is rejected before any query.
func TestGetProfileStatistics_RejectsUnknownTimespan(t *testing.T) {
	for _, ts := range []string{"", "LAST_1_HOUR", "LAST_12_HOURS", "LAST_3_YEARS", "bogus"} {
		repo := mocks.NewStatisticsRepository(t)
		_, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", ts, enabledSettings())
		require.Error(t, err, ts)
	}
}

// specRef: api-endpoint-behaviour.md J41 — from is clamped to the retention window; every instant is bucket-aligned.
func TestComputeWindow(t *testing.T) {
	spec := model.StatisticsTimespanSpec{Window: 30 * 24 * time.Hour, Bucket: 24 * time.Hour}
	tests := []struct {
		name      string
		now       time.Time
		retention time.Duration
		wantFrom  time.Time
	}{
		{"retention longer than the timespan", readNow, 90 * 24 * time.Hour, utc(2026, 9, 5, 0, 0, 0)},
		{"retention equal to the timespan", readNow, 30 * 24 * time.Hour, utc(2026, 9, 5, 0, 0, 0)},
		{"retention shorter than the timespan clamps", readNow, 7 * 24 * time.Hour, utc(2026, 9, 28, 0, 0, 0)},
		{"non-UTC now is floored in UTC", readNow.In(time.FixedZone("x", 5*3600+1800)), 30 * 24 * time.Hour, utc(2026, 9, 5, 0, 0, 0)},
		{"now exactly on a bucket", utc(2026, 10, 5, 12, 0, 0), 30 * 24 * time.Hour, utc(2026, 9, 5, 0, 0, 0)},
		{"across a DST change in Europe/Warsaw nothing shifts", utc(2026, 10, 26, 0, 30, 0), 30 * 24 * time.Hour, utc(2026, 9, 26, 0, 0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := statistics.ComputeWindow(tt.now, spec, tt.retention)
			require.True(t, from.Equal(tt.wantFrom), "from=%s", from)
			require.True(t, to.Equal(tt.now), "to=%s", to)
			require.Equal(t, time.UTC, from.Location())
			require.Equal(t, time.UTC, to.Location())
		})
	}
}

// specRef: api-endpoint-behaviour.md J41, J43 — sparse repository buckets are zero-filled into the series; sums pass through.
func TestGetProfileStatistics_ZeroFillsSeries(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	agg := &model.StatisticsAggregate{
		Totals:    model.StatisticsTotals{Total: 30, Blocked: 4, Dnssec: 2},
		Reasons:   model.StatisticsReasons{Blocklist: 3, CustomRule: 1},
		Protocols: model.StatisticsProtocols{DoH: 20, DoT: 10},
		Series: []model.StatisticsPoint{
			{Ts: utc(2026, 10, 5, 6, 15, 0), Total: 10, Blocked: 1, Dnssec: 1},
			{Ts: utc(2026, 10, 5, 12, 0, 0), Total: 20, Blocked: 3, Dnssec: 1},
			{Ts: utc(2026, 10, 4, 1, 0, 0), Total: 99}, // outside [from, to): dropped
		},
	}
	repo.On("GetProfileStatistics", mock.Anything, "p1", model.StatisticsTier15Min, mock.Anything, mock.Anything, 15*time.Minute).Return(agg, nil)

	got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", model.StatisticsLast6Hours, enabledSettings())
	require.NoError(t, err)
	require.Len(t, got.Series, 25)
	var sum int64
	for i, p := range got.Series {
		sum += p.Total
		switch i {
		case 1:
			require.Equal(t, model.StatisticsPoint{Ts: utc(2026, 10, 5, 6, 15, 0), Total: 10, Blocked: 1, Dnssec: 1}, p)
		case 24:
			require.Equal(t, int64(20), p.Total)
		default:
			require.Zero(t, p.Total+p.Blocked+p.Dnssec, "bucket %d", i)
		}
	}
	require.Equal(t, int64(30), sum)
	require.Equal(t, agg.Totals, got.Totals)
	require.Equal(t, agg.Reasons, got.Reasons)
	require.Equal(t, agg.Protocols, got.Protocols)
	require.NotNil(t, got.Devices)
	require.Empty(t, got.Devices)
}

// specRef: api-endpoint-behaviour.md J42 — statistics off: 200-shaped answer, no query, no cache traffic.
func TestGetProfileStatistics_DisabledAnswersWithoutQuerying(t *testing.T) {
	at := utc(2026, 9, 1, 8, 0, 0)
	for name, settings := range map[string]*model.StatisticsSettings{
		"nil block":              nil,
		"off":                    stats(false, nil),
		"off with stale enabled": stats(false, &at),
	} {
		t.Run(name, func(t *testing.T) {
			repo := mocks.NewStatisticsRepository(t) // any call fails the test
			cache := newFakeReadCache()
			got, err := readService(t, repo, cache).GetProfileStatistics(context.Background(), "p1", model.LAST_7_DAYS, settings)
			require.NoError(t, err)
			require.False(t, got.Enabled)
			require.Nil(t, got.EnabledAt)
			require.Equal(t, "LAST_7_DAYS", got.Timespan)
			require.Equal(t, 3600, got.BucketSeconds)
			require.Equal(t, "30d", got.Retention)
			require.True(t, got.From.Equal(utc(2026, 9, 28, 12, 0, 0)))
			require.True(t, got.To.Equal(readNow))
			require.NotNil(t, got.Series)
			require.Empty(t, got.Series)
			require.NotNil(t, got.Devices)
			require.Empty(t, got.Devices)
			require.Equal(t, model.StatisticsTotals{}, got.Totals)
			require.Equal(t, model.StatisticsReasons{}, got.Reasons)
			require.Equal(t, model.StatisticsProtocols{}, got.Protocols)
			require.Zero(t, cache.gets+cache.sets)

			b, err := json.Marshal(got)
			require.NoError(t, err)
			require.Contains(t, string(b), `"enabled_at":null`)
			require.Contains(t, string(b), `"series":[]`)
			require.Contains(t, string(b), `"devices":[]`)
		})
	}
}

// specRef: api-endpoint-behaviour.md J42 — an enabled profile reports enabled_at (null when it predates the field).
func TestGetProfileStatistics_EnabledAt(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&model.StatisticsAggregate{}, nil).Twice()
	svc := readService(t, repo, nil)

	got, err := svc.GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.True(t, got.EnabledAt.Equal(utc(2026, 9, 1, 8, 0, 0)))

	got, err = svc.GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, stats(true, nil))
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Nil(t, got.EnabledAt)
}

// specRef: api-endpoint-behaviour.md J44 — at most 100 devices, tail folded into _other, ordered by total desc then id.
func TestFoldDevices(t *testing.T) {
	many := func(n int) []model.StatisticsDevice {
		out := make([]model.StatisticsDevice, n)
		for i := range out {
			out[i] = model.StatisticsDevice{DeviceID: fmt.Sprintf("d%03d", i), Total: int64(1000 - i), Blocked: 1}
		}
		return out
	}
	sumTotals := func(ds []model.StatisticsDevice) (total, blocked int64) {
		for _, d := range ds {
			total += d.Total
			blocked += d.Blocked
		}
		return
	}

	t.Run("under the cap is only sorted", func(t *testing.T) {
		got := statistics.FoldDevices([]model.StatisticsDevice{
			{DeviceID: "b", Total: 5}, {DeviceID: "", Total: 9}, {DeviceID: "a", Total: 5}, {DeviceID: "c", Total: 1},
		})
		require.Equal(t, []string{"", "a", "b", "c"}, deviceIDs(got))
	})
	t.Run("empty input is an empty non-nil list", func(t *testing.T) {
		got := statistics.FoldDevices(nil)
		require.NotNil(t, got)
		require.Empty(t, got)
	})
	t.Run("exactly 100 is untouched", func(t *testing.T) {
		in := many(100)
		got := statistics.FoldDevices(in)
		require.Len(t, got, 100)
		require.NotContains(t, deviceIDs(got), "_other")
	})
	t.Run("101 devices fold the two smallest into _other", func(t *testing.T) {
		in := many(101)
		got := statistics.FoldDevices(in)
		require.Len(t, got, 100)
		require.Contains(t, deviceIDs(got), "_other")
		require.NotContains(t, deviceIDs(got), "d099")
		require.NotContains(t, deviceIDs(got), "d100")
		var other model.StatisticsDevice
		for _, d := range got {
			if d.DeviceID == "_other" {
				other = d
			}
		}
		require.Equal(t, in[99].Total+in[100].Total, other.Total)
		require.Equal(t, int64(2), other.Blocked)
		wantT, wantB := sumTotals(in)
		gotT, gotB := sumTotals(got)
		require.Equal(t, wantT, gotT)
		require.Equal(t, wantB, gotB)
	})
	t.Run("an existing _other is merged, not duplicated", func(t *testing.T) {
		in := append(many(150), model.StatisticsDevice{DeviceID: "_other", Total: 7, Blocked: 3})
		got := statistics.FoldDevices(in)
		require.Len(t, got, 100)
		n := 0
		for _, id := range deviceIDs(got) {
			if id == "_other" {
				n++
			}
		}
		require.Equal(t, 1, n)
		wantT, wantB := sumTotals(in)
		gotT, gotB := sumTotals(got)
		require.Equal(t, wantT, gotT)
		require.Equal(t, wantB, gotB)
	})
	t.Run("a lone _other under the cap is an ordinary entry", func(t *testing.T) {
		got := statistics.FoldDevices([]model.StatisticsDevice{{DeviceID: "_other", Total: 1}, {DeviceID: "x", Total: 2}})
		require.Equal(t, []string{"x", "_other"}, deviceIDs(got))
	})
	t.Run("the empty id survives the fold when it ranks high enough", func(t *testing.T) {
		in := append(many(120), model.StatisticsDevice{DeviceID: "", Total: 5000})
		got := statistics.FoldDevices(in)
		require.Len(t, got, 100)
		require.Contains(t, deviceIDs(got), "")
	})
	t.Run("result order is total desc then id asc", func(t *testing.T) {
		got := statistics.FoldDevices(many(130))
		for i := 1; i < len(got); i++ {
			require.False(t, got[i].Total > got[i-1].Total, "row %d out of order", i)
			if got[i].Total == got[i-1].Total {
				require.Less(t, got[i-1].DeviceID, got[i].DeviceID)
			}
		}
	})
}

func deviceIDs(ds []model.StatisticsDevice) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.DeviceID
	}
	return out
}

// specRef: api-endpoint-behaviour.md J44 — the service folds the repository's device list.
func TestGetProfileStatistics_FoldsDevices(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	var devs []model.StatisticsDevice
	for i := 0; i < 130; i++ {
		devs = append(devs, model.StatisticsDevice{DeviceID: fmt.Sprintf("d%03d", i), Total: 1})
	}
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&model.StatisticsAggregate{Devices: devs}, nil)
	got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.Len(t, got.Devices, 100)
}

// specRef: api-endpoint-behaviour.md J45 — a non-empty answer is cached and then served without a query.
func TestGetProfileStatistics_CachesNonEmptyAnswers(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&model.StatisticsAggregate{Totals: model.StatisticsTotals{Total: 5}}, nil).Once()
	cache := newFakeReadCache()
	svc := readService(t, repo, cache)

	first, err := svc.GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.Equal(t, 1, cache.sets)
	require.Contains(t, cache.data, "p1|LAST_1_DAY")

	second, err := svc.GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.Equal(t, int64(5), second.Totals.Total)
	require.True(t, second.From.Equal(first.From))
	require.Len(t, second.Series, len(first.Series))

}

// specRef: api-endpoint-behaviour.md J45 — an empty answer is never cached.
func TestGetProfileStatistics_NeverCachesEmpty(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&model.StatisticsAggregate{}, nil).Twice()
	cache := newFakeReadCache()
	svc := readService(t, repo, cache)
	for i := 0; i < 2; i++ {
		_, err := svc.GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
		require.NoError(t, err)
	}
	require.Zero(t, cache.sets)
}

// specRef: api-endpoint-behaviour.md J45 — cache failures never fail the request.
func TestGetProfileStatistics_CacheErrorsFallThrough(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&model.StatisticsAggregate{Totals: model.StatisticsTotals{Total: 5}}, nil).Once()
	cache := newFakeReadCache()
	cache.getErr = errors.New("redis down")
	cache.setErr = errors.New("redis down")

	got, err := readService(t, repo, cache).GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.Equal(t, int64(5), got.Totals.Total)
}

// specRef: api-endpoint-behaviour.md J45 — an undecodable cache entry is ignored and recomputed.
func TestGetProfileStatistics_CorruptCacheEntryIsIgnored(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&model.StatisticsAggregate{Totals: model.StatisticsTotals{Total: 5}}, nil).Once()
	cache := newFakeReadCache()
	cache.data["p1|LAST_1_DAY"] = []byte("{not json")

	got, err := readService(t, repo, cache).GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.NoError(t, err)
	require.Equal(t, int64(5), got.Totals.Total)
}

// specRef: api-endpoint-behaviour.md J4 — a repository failure is returned, nothing is cached.
func TestGetProfileStatistics_RepositoryError(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	cache := newFakeReadCache()
	_, err := readService(t, repo, cache).GetProfileStatistics(context.Background(), "p1", model.LAST_1_DAY, enabledSettings())
	require.Error(t, err)
	require.Zero(t, cache.sets)
}

// specRef: api-endpoint-behaviour.md J46 — the immediate purge (disable, profile delete) invalidates the profile's cache keys, even when the delete fails.
func TestPurge_InvalidatesReadCache(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(fmt.Sprintf("failing=%v", failing), func(t *testing.T) {
			repo := mocks.NewStatisticsRepository(t)
			var err error
			if failing {
				err = errors.New("boom")
			}
			repo.On("DeleteProfileStatistics", mock.Anything, "p1", (*time.Time)(nil)).Return(err).Once()
			cache := newFakeReadCache()
			cache.invalErr = errors.New("redis down") // must not surface

			readService(t, repo, cache).PurgeBestEffort(context.Background(), "p1")
			require.Equal(t, []string{"p1"}, cache.invalidated)
		})
	}
}

// specRef: api-endpoint-behaviour.md J46 — every delete attempt of the unconsented-statistics purge invalidates that profile's keys, whether the delete succeeds or fails.
func TestUnconsentedPurge_InvalidatesReadCache(t *testing.T) {
	cache := newFakeReadCache()
	h := newUnconsentedPurgeHarness(t, statistics.WithReadCache(cache))
	enabledAt := time.Date(2026, 9, 29, 10, 7, 0, 0, time.UTC)
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"gone", "on", "failing"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{
		"on": stats(true, &enabledAt),
	}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "gone", (*time.Time)(nil)).Return(nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on", mock.MatchedBy(func(b *time.Time) bool {
		return b != nil && b.Equal(enabledAt)
	})).Return(nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "failing", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	_, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"gone", "on", "failing"}, cache.invalidated)
}
