// Package dislock provides a Redis-backed distributed try-lock so a unit of
// work runs on exactly one of several load-balanced service instances. It is
// the single-key SET NX EX pattern from the Redis documentation: a fresh UUID
// token per acquisition, and a Lua compare-and-delete release that can never
// remove a lock another instance has since acquired (e.g. after this holder's
// TTL expired mid-run). Failed releases fall back to TTL expiry.
package dislock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// ErrNotAcquired is returned by (*Locker).TryLock when the lock is already
// held by another instance. It is a sentinel value so callers can distinguish
// "another instance is doing this work" — the normal, expected outcome on
// losing instances — from real Redis failures.
var ErrNotAcquired = errors.New("dislock: lock not acquired")

// releaseScript performs a compare-and-delete: it deletes the lock key only
// when the value still matches the token supplied by the caller. Parsed once
// at package load time.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

// Locker hands out distributed locks in a key namespace. Instances that must
// coordinate use the same client target (one shared Redis) and prefix.
type Locker struct {
	client *redis.Client
	prefix string
}

// New returns a Locker whose lock keys are prefix+key. The prefix should end
// with a separator (e.g. "cron:lock:") so locks live in a clearly
// identifiable namespace.
func New(client *redis.Client, prefix string) *Locker {
	return &Locker{
		client: client,
		prefix: prefix,
	}
}

// TryLock attempts to acquire the named lock without blocking. It returns
// ErrNotAcquired when another instance currently holds the lock; any other
// error indicates a real Redis-level failure. The ttl is the lock's expiry:
// it should comfortably exceed the longest expected runtime of the guarded
// work so the lock survives for its duration, yet stay short enough that a
// crashed holder's lock is reclaimed quickly.
func (l *Locker) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token := uuid.NewString()
	fullKey := l.prefix + key

	acquired, err := l.client.SetNX(ctx, fullKey, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("dislock: setnx %q: %w", fullKey, err)
	}
	if !acquired {
		return nil, ErrNotAcquired
	}

	return &Lock{
		client: l.client,
		key:    fullKey,
		token:  token,
	}, nil
}

// Lock is a held distributed lock. The sync.Once guarantees the
// compare-and-delete script runs at most once even if Unlock is called
// repeatedly.
type Lock struct {
	client *redis.Client
	key    string
	token  string
	once   sync.Once
	err    error
}

// Unlock releases the lock if (and only if) it is still owned by this
// instance. Calling Unlock more than once is safe and returns the result of
// the first call. A failure to talk to Redis is logged at warn level — the
// lock will still be cleaned up by its TTL.
func (l *Lock) Unlock(ctx context.Context) error {
	l.once.Do(func() {
		if _, err := releaseScript.Run(ctx, l.client, []string{l.key}, l.token).Result(); err != nil {
			log.Warn().Err(err).Str("key", l.key).Msg("dislock: failed to release lock; relying on TTL")
			l.err = fmt.Errorf("dislock: release %q: %w", l.key, err)
		}
	})
	return l.err
}
