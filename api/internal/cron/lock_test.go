package cron

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// The locking semantics (token ownership, compare-and-delete release, TTL
// expiry, idempotent unlock) are covered in libs/dislock. These tests cover
// only what the gocron adapter adds: the cron:lock: namespace and the
// not-acquired sentinel surfaced to the scheduler.

const testLockKey = "test-job"

// newTestLocker spins up an in-memory Redis and returns a fresh locker plus
// the miniredis handle so tests can assert on raw key state.
func newTestLocker(t *testing.T, ttl time.Duration) (*redisLocker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return NewRedisLocker(client, ttl).(*redisLocker), mr
}

func TestRedisLocker_AcquireUsesCronNamespace(t *testing.T) {
	locker, mr := newTestLocker(t, 5*time.Second)

	lock, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, lock)
	require.True(t, mr.Exists(lockKeyPrefix+testLockKey), "lock key must live under the cron:lock: prefix")
}

func TestRedisLocker_SecondAcquireReturnsSentinelWhileHeld(t *testing.T) {
	locker, _ := newTestLocker(t, 5*time.Second)

	first, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := locker.Lock(t.Context(), testLockKey)
	require.Error(t, err)
	require.Nil(t, second)
	require.True(t, errors.Is(err, errLockNotAcquired), "expected errLockNotAcquired, got %v", err)
}

func TestRedisLocker_UnlockAllowsReacquire(t *testing.T) {
	locker, mr := newTestLocker(t, 5*time.Second)

	first, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NoError(t, first.Unlock(t.Context()))
	require.False(t, mr.Exists(lockKeyPrefix+testLockKey), "Unlock must delete the lock key")

	second, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, second)
}
