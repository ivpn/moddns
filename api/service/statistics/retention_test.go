package statistics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/statistics"
)

// specRef: api-endpoint-behaviour.md J50 — the trim measures the retention from now and drops the read cache
// only when it deleted something.
func TestTrimToRetention(t *testing.T) {
	for _, deleted := range []int64{0, 3} {
		repo := mocks.NewStatisticsRepository(t)
		cache := newFakeReadCache()
		svc := readService(t, repo, cache)
		repo.On("TrimProfileDailyStatistics", mock.Anything, "p1", model.StatisticsRetention30d, readNow).Return(deleted, nil).Once()

		got, err := svc.TrimToRetention(context.Background(), "p1", model.StatisticsRetention30d)
		require.NoError(t, err)
		require.Equal(t, deleted, got)
		if deleted > 0 {
			require.Equal(t, []string{"p1"}, cache.invalidated)
		} else {
			require.Empty(t, cache.invalidated)
		}
	}
}

// specRef: api-endpoint-behaviour.md J50 — a failed trim on lowering is only logged.
func TestTrimBestEffort(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("TrimProfileDailyStatistics", mock.Anything, "p1", model.StatisticsRetention30d, mock.Anything).Return(int64(0), errors.New("boom")).Once()
	readService(t, repo, nil).TrimBestEffort(context.Background(), "p1", model.StatisticsRetention30d)
}

// specRef: api-endpoint-behaviour.md J8, J50 — the hourly purge trims enabled profiles to their retention;
// disabled and missing profiles are only deleted.
func TestPurgeStatistics_TrimsToRetention(t *testing.T) {
	h := newPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	on := stats(true, &enabledAt)
	on.Retention = model.StatisticsRetention90d
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"gone", "on"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, []string{"gone", "on"}).Return(map[string]*model.StatisticsSettings{"on": on}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "gone", (*time.Time)(nil)).Return(int64(5), nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on", mock.Anything).Return(int64(0), nil).Once()
	h.stats.On("TrimProfileDailyStatistics", mock.Anything, "on", model.StatisticsRetention90d, readNow).Return(int64(3), nil).Once()

	res, err := h.svc.PurgeStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.StatisticsPurgeResult{Checked: 2, Total: 2, Deleted: 8}, res)
	h.stats.AssertNotCalled(t, "TrimProfileDailyStatistics", mock.Anything, "gone", mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J9, J50 — a failed trim is counted and the run continues.
func TestPurgeStatistics_TrimFailureIsCounted(t *testing.T) {
	h := newPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{
		"a": stats(true, &enabledAt), "b": stats(true, &enabledAt),
	}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything).Return(int64(0), nil).Twice()
	h.stats.On("TrimProfileDailyStatistics", mock.Anything, "a", mock.Anything, mock.Anything).Return(int64(0), errors.New("boom")).Once()
	h.stats.On("TrimProfileDailyStatistics", mock.Anything, "b", mock.Anything, mock.Anything).Return(int64(1), nil).Once()

	res, err := h.svc.PurgeStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.StatisticsPurgeResult{Checked: 2, Total: 2, Deleted: 1, Failed: 1}, res)
}

// specRef: api-endpoint-behaviour.md J8, J53 — the hourly purge also removes buckets up to the
// one containing history_deleted_at.
func TestPurgeStatistics_HonoursHistoryDeletedAt(t *testing.T) {
	h := newPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	deletedAt := utc(2026, 10, 5, 11, 20, 0)
	on := stats(true, &enabledAt)
	on.HistoryDeletedAt = &deletedAt
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"on"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{"on": on}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on", mock.Anything).Return(int64(0), nil).Once()
	h.stats.On("DeleteProfileStatisticsThrough", mock.Anything, "on", deletedAt).Return(int64(2), nil).Once()
	h.stats.On("TrimProfileDailyStatistics", mock.Anything, "on", mock.Anything, mock.Anything).Return(int64(0), nil).Once()

	res, err := h.svc.PurgeStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.StatisticsPurgeResult{Checked: 1, Total: 1, Deleted: 2}, res)
}

// specRef: api-endpoint-behaviour.md J42, J53 — reads skip the buckets up to the one containing
// history_deleted_at and report the instant.
func TestGetProfileStatistics_HistoryDeletedAt(t *testing.T) {
	deletedAt := utc(2026, 10, 5, 11, 20, 0) // readNow is 12:07:31
	for _, tc := range []struct {
		timespan  string
		queryFrom time.Time
		points    int
	}{
		{model.StatisticsLast3Hours, utc(2026, 10, 5, 11, 30, 0), 13},
		{model.LAST_1_DAY, utc(2026, 10, 5, 12, 0, 0), 25},
	} {
		t.Run(tc.timespan, func(t *testing.T) {
			settings := enabledSettings()
			settings.HistoryDeletedAt = &deletedAt
			repo := mocks.NewStatisticsRepository(t)
			repo.On("GetProfileStatistics", mock.Anything, "p1", mock.Anything,
				mock.MatchedBy(func(from time.Time) bool { return from.Equal(tc.queryFrom) }),
				mock.Anything, mock.Anything).Return(&model.StatisticsAggregate{}, nil).Once()

			got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", tc.timespan, settings)
			require.NoError(t, err)
			require.Len(t, got.Series, tc.points, "the series grid is unchanged")
			require.NotNil(t, got.HistoryDeletedAt)
			require.True(t, got.HistoryDeletedAt.Equal(deletedAt))
		})
	}

	t.Run("bucket after now: nothing to query", func(t *testing.T) {
		settings := enabledSettings()
		recent := utc(2026, 10, 5, 12, 7, 0)
		settings.HistoryDeletedAt = &recent
		repo := mocks.NewStatisticsRepository(t) // any call fails the test
		got, err := readService(t, repo, nil).GetProfileStatistics(context.Background(), "p1", model.LAST_7_DAYS, settings)
		require.NoError(t, err)
		require.Len(t, got.Series, 169)
		require.Zero(t, got.Totals.Total)
	})
}
