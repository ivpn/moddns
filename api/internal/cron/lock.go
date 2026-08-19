package cron

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/redis/go-redis/v9"
)

// lockKeyPrefix is prepended to every job-lock key written to Redis so
// distributed locks live in a clearly identifiable namespace.
const lockKeyPrefix = "cron:lock:"

// errLockNotAcquired is returned by (*redisLocker).Lock when the lock is
// already held by another scheduler instance, so callers (and the gocron
// scheduler) can distinguish "another instance is running this tick" from
// real Redis failures and silently skip the run.
var errLockNotAcquired = dislock.ErrNotAcquired

// redisLocker adapts libs/dislock to the gocron.Locker interface. The
// per-acquisition token/compare-and-delete mechanics live in dislock; this
// adapter only fixes the key namespace and TTL for cron jobs.
type redisLocker struct {
	locker *dislock.Locker
	ttl    time.Duration
}

// NewRedisLocker returns a gocron.Locker that coordinates job execution
// across multiple scheduler instances using Redis. The provided ttl is
// the lock's expiry; it should comfortably exceed the longest expected
// job runtime so the lock survives for the duration of one tick but is
// short enough that a crashed holder's lock is reclaimed quickly.
func NewRedisLocker(client *redis.Client, ttl time.Duration) gocron.Locker {
	return &redisLocker{
		locker: dislock.New(client, lockKeyPrefix),
		ttl:    ttl,
	}
}

// Lock attempts to acquire the named job lock. It returns errLockNotAcquired
// when another instance currently holds the lock — that is the normal,
// expected outcome on losing instances and is intentionally not logged.
// Any other error indicates a real Redis-level failure.
func (l *redisLocker) Lock(ctx context.Context, key string) (gocron.Lock, error) {
	lock, err := l.locker.TryLock(ctx, key, l.ttl)
	if err != nil {
		return nil, err
	}
	return lock, nil
}
