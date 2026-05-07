package cron

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const testLockKey = "test-job"

// newTestLocker spins up an in-memory Redis and returns a fresh locker plus
// the miniredis handle (so individual tests can fast-forward time or assert
// on raw key state) and the underlying client.
func newTestLocker(t *testing.T, ttl time.Duration) (*redisLocker, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return &redisLocker{client: client, ttl: ttl}, mr, client
}

func TestRedisLocker_FirstAcquireSucceeds(t *testing.T) {
	locker, _, _ := newTestLocker(t, 5*time.Second)

	lock, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, lock)
}

func TestRedisLocker_SecondAcquireFailsWhileHeld(t *testing.T) {
	locker, _, _ := newTestLocker(t, 5*time.Second)

	first, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := locker.Lock(t.Context(), testLockKey)
	require.Error(t, err)
	require.Nil(t, second)
	require.True(t, errors.Is(err, errLockNotAcquired), "expected errLockNotAcquired, got %v", err)
}

func TestRedisLocker_AfterUnlockReacquireSucceeds(t *testing.T) {
	locker, mr, _ := newTestLocker(t, 5*time.Second)

	first, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NoError(t, first.Unlock(t.Context()))

	// L5: directly assert the key is gone after a successful Unlock,
	// not just that a subsequent Lock succeeds (which would prove it
	// only transitively).
	require.False(t, mr.Exists(lockKeyPrefix+testLockKey), "Unlock must delete the lock key")

	second, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, second)
}

func TestRedisLocker_UnlockOnlyDeletesOwnedKey(t *testing.T) {
	locker, mr, _ := newTestLocker(t, 5*time.Second)

	lock, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)

	// Simulate the TTL-expired-mid-run case: a peer instance has acquired
	// the same lock with a different token. Our Unlock must NOT delete it.
	fullKey := lockKeyPrefix + testLockKey
	require.NoError(t, mr.Set(fullKey, "different-token-from-another-instance"))

	require.NoError(t, lock.Unlock(t.Context()))

	value, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.Equal(t, "different-token-from-another-instance", value, "Unlock incorrectly deleted a peer's lock")
}

func TestRedisLocker_UnlockIsIdempotent(t *testing.T) {
	locker, _, _ := newTestLocker(t, 5*time.Second)

	lock, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)

	require.NoError(t, lock.Unlock(t.Context()))
	require.NoError(t, lock.Unlock(t.Context()), "second Unlock must be a safe no-op")
}

// TestRedisLocker_UnlockReturnsErrWhenRedisDown covers the release-failure
// branch in (*redisLock).Unlock: if the CAS script cannot be executed, the
// error is wrapped, captured under sync.Once, and returned to subsequent
// callers. The lock will still be cleaned up by its TTL on the real Redis,
// but the caller must learn that release did not succeed (L9).
func TestRedisLocker_UnlockReturnsErrWhenRedisDown(t *testing.T) {
	locker, mr, _ := newTestLocker(t, 5*time.Second)

	lock, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)

	// Simulate Redis becoming unreachable mid-run by stopping the
	// in-memory server. The compare-and-delete script will fail to
	// execute, exercising the warn-and-wrap branch.
	mr.Close()

	firstUnlockErr := lock.Unlock(t.Context())
	require.Error(t, firstUnlockErr)
	require.ErrorContains(t, firstUnlockErr, "cron locker: release")

	// sync.Once means the same error is returned on subsequent calls
	// without re-running the script.
	secondUnlockErr := lock.Unlock(t.Context())
	require.Equal(t, firstUnlockErr, secondUnlockErr, "Unlock must remain idempotent and return the captured error")
}

func TestRedisLocker_TTLExpires(t *testing.T) {
	ttl := 5 * time.Second
	locker, mr, _ := newTestLocker(t, ttl)

	first, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, first)

	// Capture the first holder's token via the underlying key so we can
	// later verify the new acquisition got a fresh one (L3).
	fullKey := lockKeyPrefix + testLockKey
	firstToken, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.NotEmpty(t, firstToken)

	// Without unlocking, advance miniredis past the TTL. The next Lock
	// must succeed because the previous key has expired.
	mr.FastForward(ttl + 1*time.Second)

	second, err := locker.Lock(t.Context(), testLockKey)
	require.NoError(t, err)
	require.NotNil(t, second)

	secondToken, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.NotEmpty(t, secondToken)
	require.NotEqual(t, firstToken, secondToken, "TTL-expiry reacquire must mint a new token")
}
