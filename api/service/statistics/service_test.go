package statistics_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func utc(y int, mo time.Month, d, h, mi, s int) time.Time {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
}

func stats(enabled bool, at *time.Time) *model.StatisticsSettings {
	return &model.StatisticsSettings{Enabled: enabled, EnabledAt: at}
}

func atp(t time.Time) *time.Time { return &t }

// specRef: api-endpoint-behaviour.md J8
func TestUnconsentedPurgeBound(t *testing.T) {
	tests := []struct {
		name       string
		exists     bool
		settings   *model.StatisticsSettings
		wantBefore *time.Time
	}{
		{"missing profile deletes everything", false, nil, nil},
		{"statistics off deletes everything", true, stats(false, nil), nil},
		{"statistics off with a stale enabled_at deletes everything", true, stats(false, atp(utc(2026, 9, 29, 10, 0, 0))), nil},
		{"no statistics block counts as off", true, nil, nil},
		{"off-then-on: the bound is enabled_at itself (the repository floors it per tier)", true, stats(true, atp(utc(2026, 9, 29, 10, 7, 30))), atp(utc(2026, 9, 29, 10, 7, 30))},
		{"enabled_at exactly on a boundary", true, stats(true, atp(utc(2026, 9, 29, 10, 15, 0))), atp(utc(2026, 9, 29, 10, 15, 0))},
		{"non-UTC enabled_at is the same instant", true, stats(true, atp(time.Date(2026, 9, 29, 12, 7, 0, 0, time.FixedZone("x", 7200)))), atp(utc(2026, 9, 29, 10, 7, 0))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := statistics.UnconsentedPurgeBound(tt.exists, tt.settings)
			if tt.wantBefore == nil {
				require.Nil(t, before)
			} else {
				require.NotNil(t, before)
				require.True(t, before.Equal(*tt.wantBefore), "before=%s", before)
			}
		})
	}
}

// specRef: api-endpoint-behaviour.md J6 — the immediate purge on disable deletes everything and a failure does not surface.
func TestPurgeBestEffort(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(fmt.Sprintf("failing=%v", failing), func(t *testing.T) {
			repo := mocks.NewStatisticsRepository(t)
			var err error
			if failing {
				err = errors.New("boom")
			}
			repo.On("DeleteProfileStatistics", mock.Anything, "p1", (*time.Time)(nil)).Return(err).Once()
			statistics.NewStatisticsService(repo).PurgeBestEffort(context.Background(), "p1")
		})
	}
}

type unconsentedPurgeHarness struct {
	stats    *mocks.StatisticsRepository
	profiles *mocks.ProfileRepository
	svc      *statistics.StatisticsService
}

func newUnconsentedPurgeHarness(t *testing.T, opts ...statistics.Option) *unconsentedPurgeHarness {
	t.Helper()
	h := &unconsentedPurgeHarness{stats: mocks.NewStatisticsRepository(t), profiles: mocks.NewProfileRepository(t)}
	h.svc = statistics.NewStatisticsService(h.stats, append([]statistics.Option{statistics.WithProfiles(h.profiles)}, opts...)...)
	return h
}

// specRef: api-endpoint-behaviour.md J8, J9 — each profile id gets the rule's decision; counts are returned.
func TestPurgeUnconsentedStatistics_AppliesRulePerProfile(t *testing.T) {
	h := newUnconsentedPurgeHarness(t)
	enabledAt := utc(2026, 9, 29, 10, 7, 0)
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"gone", "off", "on-ts"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, []string{"gone", "off", "on-ts"}).Return(map[string]*model.StatisticsSettings{
		"off": stats(false, nil), "on-ts": stats(true, &enabledAt),
	}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "gone", (*time.Time)(nil)).Return(nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "off", (*time.Time)(nil)).Return(nil).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "on-ts", mock.MatchedBy(func(b *time.Time) bool {
		return b != nil && b.Equal(enabledAt)
	})).Return(nil).Once()
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, "on-ts", model.StatisticsRetention30d, mock.Anything).Return(0, nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.UnconsentedPurgeResult{Checked: 3, Purged: 3}, res)
}

// specRef: api-endpoint-behaviour.md J9 — profile lookups are batched.
func TestPurgeUnconsentedStatistics_BatchesProfileLookups(t *testing.T) {
	h := newUnconsentedPurgeHarness(t)
	enabledAt := utc(2026, 9, 29, 10, 7, 0)
	ids := make([]string, 2500)
	settings := map[string]*model.StatisticsSettings{}
	for i := range ids {
		ids[i] = fmt.Sprintf("p%d", i)
		settings[ids[i]] = stats(true, &enabledAt)
	}
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return(ids, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.MatchedBy(func(b []string) bool { return len(b) <= 1000 })).
		Return(settings, nil).Times(3)
	h.stats.On("DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2500)
	h.stats.On("MoveProfileDailyStatistics", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Times(2500)

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2500, res.Checked)
	require.Equal(t, 2500, res.Purged)
}

// specRef: api-endpoint-behaviour.md J9 — a failed delete is counted and the run continues; the profile is retried next run.
func TestPurgeUnconsentedStatistics_DeleteFailureDoesNotStopTheRun(t *testing.T) {
	h := newUnconsentedPurgeHarness(t)
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "a", (*time.Time)(nil)).Return(errors.New("boom")).Once()
	h.stats.On("DeleteProfileStatistics", mock.Anything, "b", (*time.Time)(nil)).Return(nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.UnconsentedPurgeResult{Checked: 2, Purged: 1, Failed: 1}, res)
}

// specRef: api-endpoint-behaviour.md J9 — the first per-profile timeout ends the run.
func TestPurgeUnconsentedStatistics_JobTimeoutStopsTheRun(t *testing.T) {
	h := newUnconsentedPurgeHarness(t, statistics.WithUnconsentedPurgeTimeouts(20*time.Millisecond, time.Minute))
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "a", (*time.Time)(nil)).Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return(context.DeadlineExceeded).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, statistics.UnconsentedPurgeResult{Checked: 1, Failed: 1}, res)
	h.stats.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, "b", mock.Anything)
}

// specRef: api-endpoint-behaviour.md J9 — past the whole-run deadline no further profile is processed.
func TestPurgeUnconsentedStatistics_RunDeadlineStopsTheRun(t *testing.T) {
	h := newUnconsentedPurgeHarness(t, statistics.WithUnconsentedPurgeTimeouts(time.Minute, 50*time.Millisecond))
	h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(map[string]*model.StatisticsSettings{}, nil)
	h.stats.On("DeleteProfileStatistics", mock.Anything, "a", (*time.Time)(nil)).Run(func(mock.Arguments) {
		time.Sleep(120 * time.Millisecond)
	}).Return(nil).Once()

	res, err := h.svc.PurgeUnconsentedStatistics(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Purged)
	h.stats.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, "b", mock.Anything)
}

// specRef: api-endpoint-behaviour.md J9 — read failures abort the run with an error.
func TestPurgeUnconsentedStatistics_ReadFailuresAbort(t *testing.T) {
	t.Run("listing ids", func(t *testing.T) {
		h := newUnconsentedPurgeHarness(t)
		h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeUnconsentedStatistics(context.Background())
		require.Error(t, err)
	})
	t.Run("profile lookup", func(t *testing.T) {
		h := newUnconsentedPurgeHarness(t)
		h.stats.On("ListStatisticsProfileIDs", mock.Anything).Return([]string{"a"}, nil)
		h.profiles.On("GetProfilesStatisticsSettings", mock.Anything, mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeUnconsentedStatistics(context.Background())
		require.Error(t, err)
		h.stats.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
	})
}

// specRef: api-endpoint-behaviour.md J9 — a lookup failure must never be read as "profile missing".
func TestPurgeUnconsentedStatistics_NoProfileReaderDeletesNothing(t *testing.T) {
	stats := mocks.NewStatisticsRepository(t)
	_, err := statistics.NewStatisticsService(stats).PurgeUnconsentedStatistics(context.Background())
	require.Error(t, err)
}
