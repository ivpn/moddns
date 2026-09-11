// Package settingscache keeps the last successfully fetched settings of each
// profile in process. An entry is fresh for the configured TTL and is kept
// beyond that, as last-known-good, until the LRU evicts it; serving a stale
// entry is how the proxy keeps filtering correctly while the settings store is
// unreachable (spec: proxy-request-admission-behaviour.md Q13).
package settingscache

import (
	"sync"
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

// Eviction reasons reported to the eviction hook.
const (
	EvictionReasonSize    = "size"    // LRU capacity reached
	EvictionReasonDeleted = "deleted" // store reported the profile gone
)

type entry struct {
	settings  *model.ProfileSettings
	fetchedAt time.Time
	// size is the retained-bytes estimate computed once at Put.
	size int64
}

// Option configures a Cache at construction time.
type Option func(*Cache)

// WithClock replaces the time source; for tests that age entries.
func WithClock(now func() time.Time) Option {
	return func(c *Cache) { c.now = now }
}

// WithEvictionHook receives one call per evicted entry with its reason.
func WithEvictionHook(hook func(reason string)) Option {
	return func(c *Cache) { c.onEvict = hook }
}

// Cache is safe for concurrent use.
type Cache struct {
	ttl           time.Duration
	probeInterval time.Duration
	entries       *lru.Cache[string, *entry]
	now           func() time.Time
	onEvict       func(reason string)
	// mu serialises writers so the bytes total tracks the LRU exactly; Get
	// only takes the LRU's own lock. evictReason is set under mu around an
	// explicit removal so the LRU callback reports it instead of "size".
	mu          sync.Mutex
	evictReason string
	bytes       atomic.Int64
	// retryAt is the unix-nano instant before which store fetches are
	// refused; zero means the store is believed healthy.
	retryAt atomic.Int64
}

// New creates a cache holding at most size entries. ttl <= 0 disables
// expiry (every entry stays Fresh), matching PROFILE_SETTINGS_CACHE_TTL=0.
func New(ttl time.Duration, size int, opts ...Option) (*Cache, error) {
	c := &Cache{
		ttl:           ttl,
		probeInterval: DefaultProbeInterval,
		now:           time.Now,
		onEvict:       func(string) {},
	}
	for _, opt := range opts {
		opt(c)
	}
	entries, err := lru.NewWithEvict[string, *entry](size, func(_ string, e *entry) {
		// Runs synchronously inside Add (capacity) and Remove (explicit), both
		// under mu, so it is the single place the total is decremented.
		c.bytes.Add(-e.size)
		reason := c.evictReason
		if reason == "" {
			reason = EvictionReasonSize
		}
		c.onEvict(reason)
	})
	if err != nil {
		return nil, err
	}
	c.entries = entries
	return c, nil
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

// Put stores a successful fetch. The size estimate is taken here, once per
// fill, so the per-query Get path carries no accounting.
func (c *Cache) Put(id string, settings *model.ProfileSettings) {
	e := &entry{settings: settings, fetchedAt: c.now(), size: estimateBytes(settings)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries.Peek(id); ok {
		// Replacing in place does not fire the LRU's eviction callback.
		c.bytes.Add(-old.size)
	}
	c.entries.Add(id, e)
	c.bytes.Add(e.size)
}

// Evict forgets a profile, e.g. when the store reports it no longer exists.
func (c *Cache) Evict(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictReason = EvictionReasonDeleted
	c.entries.Remove(id) // accounting happens in the LRU callback
	c.evictReason = ""
}

// Len returns the number of cached profiles.
func (c *Cache) Len() int {
	return c.entries.Len()
}

// Bytes returns the estimated bytes retained by all cached entries.
func (c *Cache) Bytes() int64 {
	return c.bytes.Load()
}

// StoreAvailable reports whether the settings store is currently believed
// reachable (no failure mark or probe pending).
func (c *Cache) StoreAvailable() bool {
	return c.retryAt.Load() == 0
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

// StoreFailed marks the store unreachable after a connection-level error. It
// reports true on the transition from healthy to failed, so callers can log
// the outage once instead of once per query.
func (c *Cache) StoreFailed() (transition bool) {
	return c.retryAt.Swap(c.now().UnixNano()+int64(c.probeInterval)) == 0
}

// StoreRecovered clears the failure mark after a successful fetch and reports
// true when the store had been marked failed.
func (c *Cache) StoreRecovered() (transition bool) {
	return c.retryAt.Swap(0) != 0
}
