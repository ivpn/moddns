// Package settingscache keeps the last successfully fetched settings of each
// profile in process. An entry is fresh for the configured TTL and is kept
// beyond that, as last-known-good, until the LRU evicts it; serving a stale
// entry is how the proxy keeps filtering correctly while the settings store is
// unreachable (spec: proxy-request-admission-behaviour.md Q13).
package settingscache

import (
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ivpn/dns/proxy/model"
)

// State describes what Get found for a profile.
type State int

const (
	// Miss means no entry exists; the store must be consulted.
	Miss State = iota
	// Fresh means the entry is within its TTL and can be used as is.
	Fresh
	// Stale means the entry is past its TTL: refresh if the store is
	// reachable, otherwise serve it as last-known-good.
	Stale
)

// DefaultProbeInterval bounds how often a failing store is retried.
const DefaultProbeInterval = time.Second

type entry struct {
	settings  *model.ProfileSettings
	fetchedAt time.Time
}

// Cache is safe for concurrent use.
type Cache struct {
	ttl           time.Duration
	probeInterval time.Duration
	entries       *lru.Cache[string, *entry]
	now           func() time.Time
	// retryAt is the unix-nano instant before which store fetches are
	// refused; zero means the store is believed healthy.
	retryAt atomic.Int64
}

// New creates a cache holding at most size entries. ttl <= 0 disables
// expiry (every entry stays Fresh), matching PROFILE_SETTINGS_CACHE_TTL=0.
func New(ttl time.Duration, size int) (*Cache, error) {
	entries, err := lru.New[string, *entry](size)
	if err != nil {
		return nil, err
	}
	return &Cache{
		ttl:           ttl,
		probeInterval: DefaultProbeInterval,
		entries:       entries,
		now:           time.Now,
	}, nil
}

// SetClock replaces the time source; for tests that need to age entries.
func (c *Cache) SetClock(now func() time.Time) {
	c.now = now
}

// Get returns the cached settings for id and how current they are. The
// settings are nil only when the state is Miss.
func (c *Cache) Get(id string) (*model.ProfileSettings, State) {
	e, ok := c.entries.Get(id)
	if !ok {
		return nil, Miss
	}
	if c.ttl > 0 && c.now().Sub(e.fetchedAt) >= c.ttl {
		return e.settings, Stale
	}
	return e.settings, Fresh
}

// Put stores a successful fetch.
func (c *Cache) Put(id string, settings *model.ProfileSettings) {
	c.entries.Add(id, &entry{settings: settings, fetchedAt: c.now()})
}

// Evict forgets a profile, e.g. when the store reports it no longer exists.
func (c *Cache) Evict(id string) {
	c.entries.Remove(id)
}

// Len returns the number of cached profiles.
func (c *Cache) Len() int {
	return c.entries.Len()
}

// FetchAllowed reports whether the store may be queried now. While the store
// is marked failed, exactly one caller per probe interval is let through, so
// an outage costs one probe per interval instead of one timeout per query.
func (c *Cache) FetchAllowed() bool {
	retryAt := c.retryAt.Load()
	if retryAt == 0 {
		return true
	}
	now := c.now().UnixNano()
	if now < retryAt {
		return false
	}
	return c.retryAt.CompareAndSwap(retryAt, now+int64(c.probeInterval))
}

// StoreFailed marks the store unreachable after a connection-level error.
func (c *Cache) StoreFailed() {
	c.retryAt.Store(c.now().UnixNano() + int64(c.probeInterval))
}

// StoreRecovered clears the failure mark after a successful fetch.
func (c *Cache) StoreRecovered() {
	c.retryAt.Store(0)
}
