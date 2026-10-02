package statistics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ivpn/dns/api/service/statistics"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testTiming = statistics.PurgeTiming{ProxySettingsTTL: 30 * time.Second}

func utc(y int, mo time.Month, d, h, mi, s int) time.Time {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
}

// specRef: api-endpoint-behaviour.md J8 — the cutoff covers the proxy's cached-settings window and lag margin.
func TestPurgeSchedule(t *testing.T) {
	tests := []struct {
		name       string
		now        time.Time
		proxyTTL   time.Duration
		wantBefore time.Time
		wantDue    time.Time
	}{
		{"mid bucket", utc(2026, 9, 29, 10, 7, 0), 30 * time.Second, utc(2026, 9, 29, 10, 15, 0), utc(2026, 9, 29, 10, 17, 30)},
		{"settings cache still counting past the bucket end", utc(2026, 9, 29, 10, 14, 50), 30 * time.Second, utc(2026, 9, 29, 10, 30, 0), utc(2026, 9, 29, 10, 32, 30)},
		{"exactly on the boundary rolls forward", utc(2026, 9, 29, 10, 12, 30), 30 * time.Second, utc(2026, 9, 29, 10, 30, 0), utc(2026, 9, 29, 10, 32, 30)},
		{"longer proxy TTL", utc(2026, 9, 29, 10, 9, 0), 5 * time.Minute, utc(2026, 9, 29, 10, 30, 0), utc(2026, 9, 29, 10, 32, 30)},
		{"non-UTC input", time.Date(2026, 9, 29, 12, 7, 0, 0, time.FixedZone("x", 7200)), 30 * time.Second, utc(2026, 9, 29, 10, 15, 0), utc(2026, 9, 29, 10, 17, 30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, due := statistics.PurgeSchedule(tt.now, tt.proxyTTL)
			require.True(t, before.Equal(tt.wantBefore), "before=%s", before)
			require.True(t, due.Equal(tt.wantDue), "due=%s", due)
			require.True(t, due.After(before), "due must follow the last closed bucket")
		})
	}
}

// specRef: api-endpoint-behaviour.md J6, J8, J9 — immediate purge, then a queued member carrying the cutoff, due derived from it.
func TestPurgeOnDisable_ImmediateThenDelayedPass(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, testTiming))
	now := utc(2026, 9, 29, 10, 14, 50)

	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(nil).Once()
	svc.PurgeOnDisable(context.Background(), testProfile, now)

	before, due := statistics.PurgeSchedule(now, testTiming.ProxySettingsTTL)
	pending := h.pending(t)
	require.Len(t, pending, 1)
	require.Equal(t, statistics.PurgeJob{Kind: statistics.PurgeKindDisable, ProfileID: testProfile, Before: &before}.Member(), pending[0].Member)
	require.True(t, pending[0].Due.Equal(due))
}

// specRef: api-endpoint-behaviour.md J9 — a second disable in the same 15-minute window hits the same member; a later window adds one.
func TestPurgeOnDisable_SameWindowSameMemberLaterWindowNewMember(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, testTiming))
	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(nil)
	ctx := context.Background()

	svc.PurgeOnDisable(ctx, testProfile, utc(2026, 9, 29, 10, 1, 0))
	svc.PurgeOnDisable(ctx, testProfile, utc(2026, 9, 29, 10, 3, 0))
	require.Len(t, h.pending(t), 1)

	svc.PurgeOnDisable(ctx, testProfile, utc(2026, 9, 29, 10, 40, 0))
	require.Len(t, h.pending(t), 2)
}

// specRef: api-endpoint-behaviour.md J8 — the optional delay override replaces only the due time.
func TestPurgeOnDisable_DelayOverrideChangesDueOnly(t *testing.T) {
	h := newPurgeHarness(t)
	timing := statistics.PurgeTiming{ProxySettingsTTL: 30 * time.Second, DelayOverride: 5 * time.Second}
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, timing))
	now := utc(2026, 9, 29, 10, 7, 0)

	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(nil).Once()
	svc.PurgeOnDisable(context.Background(), testProfile, now)

	before, _ := statistics.PurgeSchedule(now, timing.ProxySettingsTTL)
	pending := h.pending(t)
	require.Len(t, pending, 1)
	require.Equal(t, statistics.PurgeJob{Kind: statistics.PurgeKindDisable, ProfileID: testProfile, Before: &before}.Member(), pending[0].Member)
	require.True(t, pending[0].Due.Equal(now.Add(5*time.Second)))
}

// specRef: api-endpoint-behaviour.md J6 — a failed immediate purge is covered by the queued pass alone and does not fail the caller.
func TestPurgeOnDisable_FailureIsCoveredByDelayedPass(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, testTiming))
	now := utc(2026, 9, 29, 10, 7, 0)

	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(errors.New("boom")).Once()
	svc.PurgeOnDisable(context.Background(), testProfile, now)

	pending := h.pending(t)
	require.Len(t, pending, 1)
	job, ok := statistics.ParsePurgeMember(pending[0].Member)
	require.True(t, ok)
	require.Equal(t, statistics.PurgeKindDisable, job.Kind)
}

// specRef: api-endpoint-behaviour.md J7, J9 — the pass for a deleted profile is an unbounded delete member with derived due.
func TestSchedulePurgeForDelete_UnboundedMember(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, testTiming))
	now := utc(2026, 9, 29, 10, 7, 0)

	require.NoError(t, svc.SchedulePurgeForDelete(context.Background(), testProfile, now))

	_, due := statistics.PurgeSchedule(now, testTiming.ProxySettingsTTL)
	pending := h.pending(t)
	require.Len(t, pending, 1)
	require.Equal(t, "delete:"+testProfile, pending[0].Member)
	require.True(t, pending[0].Due.Equal(due))
}

// specRef: api-endpoint-behaviour.md J7 — a failed purge on delete is covered by the queued pass and does not fail the deletion.
func TestPurgeForDelete_FailureCoveredByQueuedPass(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo, statistics.WithPurgeQueue(h.queue, testTiming))

	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(errors.New("boom")).Once()
	require.NoError(t, svc.PurgeForDelete(context.Background(), testProfile))
}

// specRef: api-endpoint-behaviour.md J7 — without a queue a failed purge on delete is surfaced.
func TestPurgeForDelete_FailureWithoutQueueReturnsError(t *testing.T) {
	h := newPurgeHarness(t)
	svc := statistics.NewStatisticsService(h.repo)

	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(errors.New("boom")).Once()
	require.Error(t, svc.PurgeForDelete(context.Background(), testProfile))
}
