package dislock

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const (
	testPrefix  = "test:lock:"
	testLockKey = "test-job"
	testTTL     = 5 * time.Second
)

// newTestLocker spins up an in-memory Redis and returns a fresh locker plus
// the miniredis handle (so individual tests can fast-forward time or assert
// on raw key state).
func newTestLocker(t *testing.T) (*Locker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return New(client, testPrefix), mr
}

func TestTryLock_FirstAcquireSucceeds(t *testing.T) {
	locker, _ := newTestLocker(t)

	lock, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NotNil(t, lock)
}

func TestTryLock_SecondAcquireFailsWhileHeld(t *testing.T) {
	locker, _ := newTestLocker(t)

	first, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.Error(t, err)
	require.Nil(t, second)
	require.True(t, errors.Is(err, ErrNotAcquired), "expected ErrNotAcquired, got %v", err)
}

func TestTryLock_DistinctKeysAreIndependent(t *testing.T) {
	locker, _ := newTestLocker(t)

	first, err := locker.TryLock(t.Context(), "source:blp_a", testTTL)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := locker.TryLock(t.Context(), "source:blp_b", testTTL)
	require.NoError(t, err)
	require.NotNil(t, second, "a held lock must not block other keys")
}

func TestUnlock_DeletesKeyAndAllowsReacquire(t *testing.T) {
	locker, mr := newTestLocker(t)

	first, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NoError(t, first.Unlock(t.Context()))

	// Directly assert the key is gone after a successful Unlock, not just
	// that a subsequent TryLock succeeds (which would prove it only
	// transitively).
	require.False(t, mr.Exists(testPrefix+testLockKey), "Unlock must delete the lock key")

	second, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NotNil(t, second)
}

func TestUnlock_OnlyDeletesOwnedKey(t *testing.T) {
	locker, mr := newTestLocker(t)

	lock, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)

	// Simulate the TTL-expired-mid-run case: a peer instance has acquired
	// the same lock with a different token. Our Unlock must NOT delete it.
	fullKey := testPrefix + testLockKey
	require.NoError(t, mr.Set(fullKey, "different-token-from-another-instance"))

	require.NoError(t, lock.Unlock(t.Context()))

	value, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.Equal(t, "different-token-from-another-instance", value, "Unlock incorrectly deleted a peer's lock")
}

func TestUnlock_IsIdempotent(t *testing.T) {
	locker, _ := newTestLocker(t)

	lock, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)

	require.NoError(t, lock.Unlock(t.Context()))
	require.NoError(t, lock.Unlock(t.Context()), "second Unlock must be a safe no-op")
}

// TestUnlock_ReturnsErrWhenRedisDown covers the release-failure branch in
// (*Lock).Unlock: if the compare-and-delete script cannot be executed, the
// error is wrapped, captured under sync.Once, and returned to subsequent
// callers. The lock will still be cleaned up by its TTL on the real Redis,
// but the caller must learn that release did not succeed.
func TestUnlock_ReturnsErrWhenRedisDown(t *testing.T) {
	locker, mr := newTestLocker(t)

	lock, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)

	// Simulate Redis becoming unreachable mid-run by stopping the
	// in-memory server. The compare-and-delete script will fail to
	// execute, exercising the warn-and-wrap branch.
	mr.Close()

	firstUnlockErr := lock.Unlock(t.Context())
	require.Error(t, firstUnlockErr)
	require.ErrorContains(t, firstUnlockErr, "dislock: release")

	// sync.Once means the same error is returned on subsequent calls
	// without re-running the script.
	secondUnlockErr := lock.Unlock(t.Context())
	require.Equal(t, firstUnlockErr, secondUnlockErr, "Unlock must remain idempotent and return the captured error")
}

func TestTryLock_TTLExpires(t *testing.T) {
	locker, mr := newTestLocker(t)

	first, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NotNil(t, first)

	// Capture the first holder's token via the underlying key so we can
	// later verify the new acquisition got a fresh one.
	fullKey := testPrefix + testLockKey
	firstToken, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.NotEmpty(t, firstToken)

	// Without unlocking, advance miniredis past the TTL. The next TryLock
	// must succeed because the previous key has expired.
	mr.FastForward(testTTL + 1*time.Second)

	second, err := locker.TryLock(t.Context(), testLockKey, testTTL)
	require.NoError(t, err)
	require.NotNil(t, second)

	secondToken, err := mr.Get(fullKey)
	require.NoError(t, err)
	require.NotEmpty(t, secondToken)
	require.NotEqual(t, firstToken, secondToken, "TTL-expiry reacquire must mint a new token")
}
