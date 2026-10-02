package statistics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testProfile = "profile-a"

var farFuture = time.Unix(4_000_000_000, 0)

type purgeHarness struct {
	mr     *miniredis.Miniredis
	client *redis.Client
	queue  *cache.RedisCache
	repo   *mocks.StatisticsRepository
}

func newPurgeHarness(t *testing.T) *purgeHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &purgeHarness{mr: mr, client: client, queue: cache.NewRedisCacheFromClient(client), repo: mocks.NewStatisticsRepository(t)}
}

func (h *purgeHarness) sweeper() *statistics.PurgeSweeper {
	return statistics.NewPurgeSweeper(h.queue, h.repo)
}

// pending returns every queued member, due or not, with its due time.
func (h *purgeHarness) pending(t *testing.T) []cache.StatisticsPurgeEntry {
	t.Helper()
	got, err := h.queue.DueStatisticsPurges(context.Background(), farFuture, 100)
	require.NoError(t, err)
	return got
}

func (h *purgeHarness) enqueue(t *testing.T, job statistics.PurgeJob, due time.Time) {
	t.Helper()
	require.NoError(t, h.queue.EnqueueStatisticsPurge(context.Background(), job.Member(), due))
}

func at(t time.Time) *time.Time { return &t }

// specRef: api-endpoint-behaviour.md J9 — the member encodes kind, profile and cutoff.
func TestPurgeMember_RoundTrip(t *testing.T) {
	cutoff := time.Unix(1_800_000_900, 0).UTC()
	tests := []struct {
		name string
		job  statistics.PurgeJob
		want string
	}{
		{"disable carries its cutoff", statistics.PurgeJob{Kind: statistics.PurgeKindDisable, ProfileID: testProfile, Before: &cutoff}, "disable:profile-a:1800000900"},
		{"delete is unbounded", statistics.PurgeJob{Kind: statistics.PurgeKindDelete, ProfileID: testProfile}, "delete:profile-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.job.Member())
			got, ok := statistics.ParsePurgeMember(tt.want)
			require.True(t, ok)
			require.Equal(t, tt.job.Kind, got.Kind)
			require.Equal(t, tt.job.ProfileID, got.ProfileID)
			if tt.job.Before == nil {
				require.Nil(t, got.Before)
			} else {
				require.True(t, got.Before.Equal(*tt.job.Before))
			}
		})
	}
}

// specRef: api-endpoint-behaviour.md J10 — malformed members are rejected by the parser.
func TestParsePurgeMember_RejectsMalformed(t *testing.T) {
	for _, m := range []string{"", "garbage", "disable:", "disable:p", "disable:p:notanumber", "disable:p:-5", "disable::100", "delete:", "delete:p:100", "retry:p"} {
		_, ok := statistics.ParsePurgeMember(m)
		require.False(t, ok, "member %q", m)
	}
}

// specRef: api-endpoint-behaviour.md J10 — a due job runs with its own bound and is removed on success.
func TestSweeper_RunsDueJobAndRemovesIt(t *testing.T) {
	h := newPurgeHarness(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0).UTC()
	before := now.Add(15 * time.Minute)

	h.enqueue(t, statistics.PurgeJob{Kind: statistics.PurgeKindDisable, ProfileID: testProfile, Before: &before}, now)
	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, mock.MatchedBy(func(b *time.Time) bool {
		return b != nil && b.Equal(before)
	})).Return(nil).Once()

	res, err := h.sweeper().Sweep(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 1, res.Done)
	require.Empty(t, h.pending(t))
}

// specRef: api-endpoint-behaviour.md J10 — a delete job is unbounded.
func TestSweeper_DeleteJobIsUnbounded(t *testing.T) {
	h := newPurgeHarness(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	h.enqueue(t, statistics.PurgeJob{Kind: statistics.PurgeKindDelete, ProfileID: testProfile}, now)
	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(nil).Once()

	res, err := h.sweeper().Sweep(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 1, res.Done)
}

// specRef: api-endpoint-behaviour.md J10 — only due jobs are taken.
func TestSweeper_SkipsFutureJobs(t *testing.T) {
	h := newPurgeHarness(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	h.enqueue(t, statistics.PurgeJob{Kind: statistics.PurgeKindDelete, ProfileID: testProfile}, now.Add(time.Minute))

	res, err := h.sweeper().Sweep(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, statistics.SweepResult{}, res)
	require.Len(t, h.pending(t), 1)
}

// specRef: api-endpoint-behaviour.md J10 — a failed delete leaves the job queued for the next tick.
func TestSweeper_FailedDeleteKeepsJob(t *testing.T) {
	h := newPurgeHarness(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	h.enqueue(t, statistics.PurgeJob{Kind: statistics.PurgeKindDelete, ProfileID: testProfile}, now)
	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(errors.New("boom")).Once()

	res, err := h.sweeper().Sweep(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 1, res.Failed)
	require.Len(t, h.pending(t), 1)
}

// specRef: api-endpoint-behaviour.md J10 — malformed members are dropped.
func TestSweeper_DropsMalformedMember(t *testing.T) {
	h := newPurgeHarness(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	require.NoError(t, h.queue.EnqueueStatisticsPurge(context.Background(), "garbage", now))

	res, err := h.sweeper().Sweep(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 1, res.Dropped)
	require.Empty(t, h.pending(t))
}

// specRef: api-endpoint-behaviour.md J10 — a hung delete is cut off and the job stays queued.
func TestSweeper_JobTimeoutKeepsJob(t *testing.T) {
	h := newPurgeHarness(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	h.enqueue(t, statistics.PurgeJob{Kind: statistics.PurgeKindDelete, ProfileID: testProfile}, now)
	h.repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return(context.DeadlineExceeded).Once()

	sw := h.sweeper()
	sw.SetJobTimeout(50 * time.Millisecond)
	start := time.Now()
	res, err := sw.Sweep(context.Background(), now)
	require.NoError(t, err)
	require.Less(t, time.Since(start), 5*time.Second)
	require.Equal(t, 1, res.Failed)
	require.Len(t, h.pending(t), 1)
}

// specRef: api-endpoint-behaviour.md J8 — PurgeCutoff is the end of the 15-minute UTC bucket containing its argument.
func TestPurgeCutoff(t *testing.T) {
	tests := []struct {
		now  time.Time
		want time.Time
	}{
		{time.Date(2026, 9, 29, 10, 7, 30, 0, time.UTC), time.Date(2026, 9, 29, 10, 15, 0, 0, time.UTC)},
		{time.Date(2026, 9, 29, 10, 15, 0, 0, time.UTC), time.Date(2026, 9, 29, 10, 30, 0, 0, time.UTC)},
		{time.Date(2026, 9, 29, 23, 59, 59, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)},
		{time.Date(2026, 9, 29, 12, 3, 0, 0, time.FixedZone("x", 3600)), time.Date(2026, 9, 29, 11, 15, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		require.True(t, statistics.PurgeCutoff(tt.now).Equal(tt.want), "now=%s", tt.now)
	}
}

// specRef: api-endpoint-behaviour.md J10 — a queue read failure is returned and runs nothing.
func TestSweeper_QueueReadFailureIsReturned(t *testing.T) {
	queue := mocks.NewStatisticsPurgeQueuecache(t)
	repo := mocks.NewStatisticsRepository(t)
	queue.On("DueStatisticsPurges", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("redis down")).Once()

	_, err := statistics.NewPurgeSweeper(queue, repo).Sweep(context.Background(), time.Now())
	require.Error(t, err)
}

// specRef: api-endpoint-behaviour.md J10 — a job whose removal fails is counted as failed and will simply re-run (deletes are idempotent).
func TestSweeper_RemoveFailureCountsAsFailed(t *testing.T) {
	queue := mocks.NewStatisticsPurgeQueuecache(t)
	repo := mocks.NewStatisticsRepository(t)
	queue.On("DueStatisticsPurges", mock.Anything, mock.Anything, mock.Anything).
		Return([]cache.StatisticsPurgeEntry{{Member: "delete:" + testProfile}}, nil).Once()
	repo.On("DeleteProfileStatistics", mock.Anything, testProfile, (*time.Time)(nil)).Return(nil).Once()
	queue.On("RemoveStatisticsPurge", mock.Anything, "delete:"+testProfile).Return(errors.New("redis down")).Once()

	res, err := statistics.NewPurgeSweeper(queue, repo).Sweep(context.Background(), time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, res.Failed)
}
