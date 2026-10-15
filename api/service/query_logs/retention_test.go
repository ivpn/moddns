package querylogs_test

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
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/libs/dislock"
)

var now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func testLocker(t *testing.T) *dislock.Locker {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return dislock.New(client, querylogs.RetentionMoveLockPrefix)
}

// specRef: api-endpoint-behaviour.md J54 — the move conforms the profile to the retention as of now.
func TestConformToRetention(t *testing.T) {
	repo := mocks.NewQueryLogsRepository(t)
	svc := querylogs.NewQueryLogsService(repo, querylogs.WithClock(func() time.Time { return now }))
	svc.SetRetentionMoveLocker(testLocker(t))
	repo.On("ConformQueryLogs", mock.Anything, "p1", model.RetentionOneDay, now).Return(4, int64(2), nil).Once()

	moved, deleted, err := svc.ConformToRetention(context.Background(), "p1", model.RetentionOneDay)
	require.NoError(t, err)
	require.Equal(t, 4, moved)
	require.EqualValues(t, 2, deleted)
}

// specRef: api-endpoint-behaviour.md J54 — a move already running elsewhere is skipped, not repeated.
func TestConformToRetention_SkipsWhenLocked(t *testing.T) {
	repo := mocks.NewQueryLogsRepository(t)
	svc := querylogs.NewQueryLogsService(repo)
	locker := testLocker(t)
	svc.SetRetentionMoveLocker(locker)
	held, err := locker.TryLock(context.Background(), "p1", time.Minute)
	require.NoError(t, err)
	defer func() { _ = held.Unlock(context.Background()) }()

	moved, deleted, err := svc.ConformToRetention(context.Background(), "p1", model.RetentionOneDay)
	require.NoError(t, err)
	require.Zero(t, moved)
	require.Zero(t, deleted)
	repo.AssertNotCalled(t, "ConformQueryLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J54 - the move on lowering runs in the background: ConformBestEffort
// returns before it starts, the move outlives the caller's context under its own deadline, an error is
// only logged, and done runs afterwards either way.
func TestConformBestEffort(t *testing.T) {
	repo := mocks.NewQueryLogsRepository(t)
	var moveErr error
	var deadline time.Time
	var hasDeadline bool
	repo.On("ConformQueryLogs", mock.Anything, "p1", model.RetentionOneHour, mock.Anything).
		Run(func(args mock.Arguments) {
			moveCtx := args.Get(0).(context.Context)
			moveErr = moveCtx.Err()
			deadline, hasDeadline = moveCtx.Deadline()
		}).
		Return(0, int64(0), errors.New("boom")).Once()
	var pending []func()
	svc := querylogs.NewQueryLogsService(repo, querylogs.WithBackground(func(f func()) { pending = append(pending, f) }))
	svc.SetRetentionMoveLocker(testLocker(t))

	callerCtx, cancel := context.WithCancel(context.Background())
	done := 0
	svc.ConformBestEffort(callerCtx, "p1", model.RetentionOneHour, func(context.Context) { done++ })
	cancel()
	repo.AssertNotCalled(t, "ConformQueryLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	require.Len(t, pending, 1)

	start := time.Now()
	pending[0]()
	require.NoError(t, moveErr, "the caller's cancellation does not reach the move")
	require.True(t, hasDeadline, "the move has a deadline")
	require.WithinDuration(t, start.Add(querylogs.RetentionMoveDeadline), deadline, 5*time.Second)
	require.Equal(t, 1, done)
}

// specRef: api-endpoint-behaviour.md J54 - the move lock outlives the longest move (and the undo of
// its last batch), so the reconcile cannot start the same move while one is still running.
func TestRetentionMoveLockOutlivesTheMove(t *testing.T) {
	require.GreaterOrEqual(t, querylogs.RetentionMoveLockTTL, querylogs.RetentionMoveDeadline+time.Minute)
}
