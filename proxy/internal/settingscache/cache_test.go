package settingscache

import (
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T, ttl time.Duration) (*Cache, *time.Time) {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	c, err := New(ttl, 8, WithClock(func() time.Time { return now }))
	require.NoError(t, err)
	return c, &now
}

func settings(rule string) *model.ProfileSettings {
	return &model.ProfileSettings{Privacy: map[string]string{"default_rule": rule}}
}

// specRef: proxy-request-admission-behaviour.md #S1 #S2 #S3 #S6
func TestGet_MissFreshStale(t *testing.T) {
	c, now := newTestCache(t, 30*time.Second)

	got, state := c.Get("p1")
	assert.Nil(t, got)
	assert.Equal(t, Miss, state)

	c.Put("p1", settings("allow"))
	got, state = c.Get("p1")
	assert.Equal(t, Fresh, state)
	assert.Equal(t, "allow", got.Privacy["default_rule"])

	*now = now.Add(29 * time.Second)
	_, state = c.Get("p1")
	assert.Equal(t, Fresh, state)

	// Past the TTL the entry is kept and reported as stale, not dropped.
	*now = now.Add(time.Second)
	got, state = c.Get("p1")
	assert.Equal(t, Stale, state)
	assert.Equal(t, "allow", got.Privacy["default_rule"])

	*now = now.Add(48 * time.Hour)
	got, state = c.Get("p1")
	assert.Equal(t, Stale, state)
	assert.NotNil(t, got)
}

// specRef: proxy-request-admission-behaviour.md #S5
func TestGet_ZeroTTLNeverExpires(t *testing.T) {
	c, now := newTestCache(t, 0)
	c.Put("p1", settings("block"))
	*now = now.Add(365 * 24 * time.Hour)
	_, state := c.Get("p1")
	assert.Equal(t, Fresh, state)
}

// specRef: proxy-request-admission-behaviour.md #S2
func TestPut_RefreshResetsAge(t *testing.T) {
	c, now := newTestCache(t, 30*time.Second)
	c.Put("p1", settings("allow"))
	*now = now.Add(time.Minute)
	_, state := c.Get("p1")
	assert.Equal(t, Stale, state)

	c.Put("p1", settings("block"))
	got, state := c.Get("p1")
	assert.Equal(t, Fresh, state)
	assert.Equal(t, "block", got.Privacy["default_rule"])
}

// specRef: proxy-request-admission-behaviour.md #S8
func TestEvict(t *testing.T) {
	c, _ := newTestCache(t, 30*time.Second)
	c.Put("p1", settings("allow"))
	c.Evict("p1")
	_, state := c.Get("p1")
	assert.Equal(t, Miss, state)
	assert.Equal(t, 0, c.Len())
}

// specRef: proxy-request-admission-behaviour.md #S7
func TestSizeBound(t *testing.T) {
	c, err := New(time.Minute, 2)
	require.NoError(t, err)
	c.Put("a", settings("allow"))
	c.Put("b", settings("allow"))
	c.Put("c", settings("allow"))
	assert.Equal(t, 2, c.Len())
	_, state := c.Get("a")
	assert.Equal(t, Miss, state, "least recently used entry is evicted")
}

// specRef: proxy-request-admission-behaviour.md #S9
func TestBreaker_OneProbePerInterval(t *testing.T) {
	c, now := newTestCache(t, 30*time.Second)
	assert.True(t, c.FetchAllowed(), "healthy store: every fetch allowed")
	assert.True(t, c.FetchAllowed())

	assert.True(t, c.StoreFailed(), "first failure is the healthy→failed transition")
	assert.False(t, c.StoreFailed(), "repeated failures are not transitions")
	assert.False(t, c.FetchAllowed(), "just failed: no fetch until the probe interval passes")

	*now = now.Add(DefaultProbeInterval - time.Millisecond)
	assert.False(t, c.FetchAllowed())

	*now = now.Add(time.Millisecond)
	assert.True(t, c.FetchAllowed(), "first caller after the interval probes")
	assert.False(t, c.FetchAllowed(), "second caller in the same interval does not")

	*now = now.Add(DefaultProbeInterval)
	assert.True(t, c.FetchAllowed(), "probe re-arms once per interval while failing")

	assert.True(t, c.StoreRecovered(), "recovery after a failure is a transition")
	assert.False(t, c.StoreRecovered(), "recovery while healthy is not")
	assert.True(t, c.FetchAllowed())
	assert.True(t, c.FetchAllowed(), "recovered: no gating")
}

// specRef: proxy-request-admission-behaviour.md #S7 #S8 #S11
func TestBytesAccounting(t *testing.T) {
	var reasons []string
	c, err := New(time.Minute, 2, WithEvictionHook(func(r string) { reasons = append(reasons, r) }))
	require.NoError(t, err)
	assert.Zero(t, c.Bytes())

	small := settings("allow")
	c.Put("a", small)
	sizeA := c.Bytes()
	assert.Greater(t, sizeA, int64(entryOverhead), "an entry costs more than its fixed overhead")

	heavy := &model.ProfileSettings{Privacy: map[string]string{"default_rule": "block"}}
	for i := 0; i < 100; i++ {
		heavy.CustomRules = append(heavy.CustomRules, map[string]string{"value": "*.tracker.example", "action": "block", "syntax": "domain"})
	}
	c.Put("b", heavy)
	sizeB := c.Bytes() - sizeA
	assert.Greater(t, sizeB, 100*int64(mapOverhead), "rules dominate a heavy entry")

	// Replacing in place swaps the old size for the new one.
	c.Put("a", heavy)
	assert.Equal(t, 2*sizeB, c.Bytes())
	assert.Empty(t, reasons, "in-place replacement is not an eviction")

	// Capacity eviction removes the least recently used entry and reports it.
	c.Put("c", small)
	assert.Equal(t, 2, c.Len())
	assert.Equal(t, sizeB+sizeA, c.Bytes())
	assert.Equal(t, []string{EvictionReasonSize}, reasons)

	// Explicit eviction reports "deleted"; evicting an unknown key is a no-op.
	c.Evict("c")
	c.Evict("missing")
	assert.Equal(t, sizeB, c.Bytes())
	assert.Equal(t, []string{EvictionReasonSize, EvictionReasonDeleted}, reasons)

	c.Evict("a")
	assert.Zero(t, c.Bytes())
	assert.Zero(t, c.Len())
}

// specRef: proxy-request-admission-behaviour.md #S9
func TestStoreAvailable(t *testing.T) {
	c, now := newTestCache(t, time.Minute)
	assert.True(t, c.StoreAvailable())
	c.StoreFailed()
	assert.False(t, c.StoreAvailable())
	*now = now.Add(2 * DefaultProbeInterval)
	assert.False(t, c.StoreAvailable(), "a due probe does not mean the store is back")
	c.StoreRecovered()
	assert.True(t, c.StoreAvailable())
}

// specRef: proxy-request-admission-behaviour.md #S11
func TestEstimateBytes_Shapes(t *testing.T) {
	assert.Equal(t, int64(entryOverhead), estimateBytes(nil))
	assert.Equal(t, int64(entryOverhead), estimateBytes(&model.ProfileSettings{}))
	one := estimateBytes(&model.ProfileSettings{Privacy: map[string]string{"k": "vv"}})
	assert.Equal(t, int64(entryOverhead+mapOverhead+kvOverhead+3), one)
}
