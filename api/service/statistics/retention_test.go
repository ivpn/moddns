package statistics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/libs/dislock"
)

func testLocker(t *testing.T) *dislock.Locker {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return dislock.New(client, statistics.RetentionMoveLockPrefix)
}

// specRef: api-endpoint-behaviour.md J50 — the move keeps documents since floor_day(now − retention)
// and invalidates the read cache.
func TestMoveToRetention(t *testing.T) {
	for _, tc := range []struct {
		retention model.StatisticsRetention
		since     time.Time
	}{
		{model.StatisticsRetention30d, utc(2026, 9, 5, 0, 0, 0)},
		{model.StatisticsRetention90d, utc(2026, 7, 7, 0, 0, 0)},
		{"", utc(2026, 9, 5, 0, 0, 0)},
	} {
		repo := mocks.NewStatisticsRepository(t)
		cache := newFakeReadCache()
		svc := readService(t, repo, cache)
		svc.SetRetentionMoveLocker(testLocker(t))
		repo.On("MoveProfileDailyStatistics", mock.Anything, "p1", tc.retention.OrDefault(),
			mock.MatchedBy(func(since time.Time) bool { return since.Equal(tc.since) })).Return(4, nil).Once()

		moved, err := svc.MoveToRetention(context.Background(), "p1", tc.retention)
		require.NoError(t, err)
		require.Equal(t, 4, moved)
		require.Equal(t, []string{"p1"}, cache.invalidated)
	}
}

// specRef: api-endpoint-behaviour.md J50 — a move already running elsewhere is skipped, not repeated.
func TestMoveToRetention_SkipsWhenLocked(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	svc := readService(t, repo, nil)
	locker := testLocker(t)
	svc.SetRetentionMoveLocker(locker)
	held, err := locker.TryLock(context.Background(), "p1", time.Minute)
	require.NoError(t, err)
	defer func() { _ = held.Unlock(context.Background()) }()

	moved, err := svc.MoveToRetention(context.Background(), "p1", model.StatisticsRetention30d)
	require.NoError(t, err)
	require.Zero(t, moved)
	repo.AssertNotCalled(t, "MoveProfileDailyStatistics", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J50 — the immediate move is best-effort: an error is only logged.
func TestMoveBestEffort(t *testing.T) {
	repo := mocks.NewStatisticsRepository(t)
	repo.On("MoveProfileDailyStatistics", mock.Anything, "p1", model.StatisticsRetention30d, mock.Anything).Return(0, errors.New("boom")).Once()
	svc := readService(t, repo, nil)
	svc.SetRetentionMoveLocker(testLocker(t))
	svc.MoveBestEffort(context.Background(), "p1", model.StatisticsRetention30d)
}

// specRef: api-endpoint-behaviour.md J8, J50 — the hourly purge moves daily documents of enabled
// profiles down to their retention; disabled and missing profiles are only deleted.
func TestPurgeUnconsentedStatistics_MovesToRetention(t *testing.T) {
	h := newUnconsentedPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	h.svc.SetRetentionMoveLocker(testLocker(t))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	on := stats(true, &enabledAt)
	on.Retention = model.StatisticsRetention90d
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"gone", "on"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, []string{"gone", "on"}).Return(map[string]*model.StatisticsSettings{"on": on}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "gone", (*time.Time)(nil)).Return(nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on", mock.Anything).Return(nil).Once()
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, "on", model.StatisticsRetention90d,
		mock.MatchedBy(func(since time.Time) bool { return since.Equal(utc(2026, 7, 7, 0, 0, 0)) })).Return(3, nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.UnconsentedPurgeResult{Checked: 2, Purged: 2, Moved: 3}, res)
	h.stats.AssertNotCalled(t, "MoveProfileDailyStatistics", mock.Anything, "gone", mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J9, J50 — a failed move is counted and the run continues.
func TestPurgeUnconsentedStatistics_MoveFailureIsCounted(t *testing.T) {
	h := newUnconsentedPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{
		"a": stats(true, &enabledAt), "b": stats(true, &enabledAt),
	}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, "a", mock.Anything, mock.Anything).Return(0, errors.New("boom")).Once()
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, "b", mock.Anything, mock.Anything).Return(1, nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.UnconsentedPurgeResult{Checked: 2, Purged: 1, Failed: 1, Moved: 1}, res)
}

// specRef: api-endpoint-behaviour.md J8, J53 — the hourly purge also removes buckets up to the
// one containing history_deleted_at.
func TestPurgeUnconsentedStatistics_HonoursHistoryDeletedAt(t *testing.T) {
	h := newUnconsentedPurgeHarness(t, statistics.WithClock(func() time.Time { return readNow }))
	enabledAt := utc(2026, 9, 1, 8, 0, 0)
	deletedAt := utc(2026, 10, 5, 11, 20, 0)
	on := stats(true, &enabledAt)
	on.HistoryDeletedAt = &deletedAt
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"on"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{"on": on}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on", mock.Anything).Return(nil).Once()
	h.stats.On("DeleteProfileStatisticsThrough", mock.Anything, "on", deletedAt).Return(nil).Once()
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, "on", mock.Anything, mock.Anything).Return(0, nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Purged)
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
