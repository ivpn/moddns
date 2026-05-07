// Package cron provides scheduled job execution and the distributed
// locking primitives required to run a single scheduler across multiple
// load-balanced API instances.
package cron

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// lockKeyPrefix is prepended to every job-lock key written to Redis so
// distributed locks live in a clearly identifiable namespace.
const lockKeyPrefix = "cron:lock:"

// errLockNotAcquired is returned by (*redisLocker).Lock when the lock is
// already held by another scheduler instance. It is a sentinel value so
// callers (and the gocron scheduler) can distinguish "another instance is
// running this tick" from real Redis failures and silently skip the run.
var errLockNotAcquired = errors.New("cron locker: lock not acquired")

// lockReleaseScript performs a compare-and-delete: it deletes the lock key
// only when the value still matches the token supplied by the caller. This
// prevents an instance from accidentally deleting a lock another instance
// has since acquired (e.g. when the original holder's TTL expired
// mid-run). It is parsed once at package load time.
var lockReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

// redisLocker is a gocron.Locker implementation backed by Redis SET NX EX.
// A fresh UUID-v4 token is generated on every successful acquisition so
// the unlock path can guarantee it only releases its own key.
type redisLocker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisLocker returns a gocron.Locker that coordinates job execution
// across multiple scheduler instances using Redis. The provided ttl is
// the lock's expiry; it should comfortably exceed the longest expected
// job runtime so the lock survives for the duration of one tick but is
// short enough that a crashed holder's lock is reclaimed quickly.
func NewRedisLocker(client *redis.Client, ttl time.Duration) gocron.Locker {
	return &redisLocker{
		client: client,
		ttl:    ttl,
	}
}

// Lock attempts to acquire the named job lock. It returns errLockNotAcquired
// when another instance currently holds the lock — that is the normal,
// expected outcome on losing instances and is intentionally not logged.
// Any other error indicates a real Redis-level failure.
func (l *redisLocker) Lock(ctx context.Context, key string) (gocron.Lock, error) {
	token := uuid.NewString()
	fullKey := lockKeyPrefix + key

	acquired, err := l.client.SetNX(ctx, fullKey, token, l.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("cron locker: setnx %q: %w", fullKey, err)
	}
	if !acquired {
		return nil, errLockNotAcquired
	}

	return &redisLock{
		client: l.client,
		key:    fullKey,
		token:  token,
	}, nil
}

// redisLock is a gocron.Lock backed by Redis. The sync.Once guarantees that
// the compare-and-delete script runs at most once even if Unlock is called
// repeatedly (gocron's contract leaves the call count up to the scheduler).
type redisLock struct {
	client *redis.Client
	key    string
	token  string
	once   sync.Once
	err    error
}

// Unlock releases the Redis lock if (and only if) it is still owned by this
// instance. Calling Unlock more than once is safe and returns the result of
// the first call. A failure to talk to Redis is logged at warn level — the
// lock will still be cleaned up by its TTL.
func (l *redisLock) Unlock(ctx context.Context) error {
	l.once.Do(func() {
		if _, err := lockReleaseScript.Run(ctx, l.client, []string{l.key}, l.token).Result(); err != nil {
			log.Warn().Err(err).Str("key", l.key).Msg("cron locker: failed to release lock; relying on TTL")
			l.err = fmt.Errorf("cron locker: release %q: %w", l.key, err)
		}
	})
	return l.err
}
