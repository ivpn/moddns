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
	c, err := New(ttl, 8)
	require.NoError(t, err)
	now := time.Unix(1_700_000_000, 0)
	c.SetClock(func() time.Time { return now })
	return c, &now
}

func settings(rule string) *model.ProfileSettings {
	return &model.ProfileSettings{Privacy: map[string]string{"default_rule": rule}}
}

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

func TestGet_ZeroTTLNeverExpires(t *testing.T) {
	c, now := newTestCache(t, 0)
	c.Put("p1", settings("block"))
	*now = now.Add(365 * 24 * time.Hour)
	_, state := c.Get("p1")
	assert.Equal(t, Fresh, state)
}

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

func TestEvict(t *testing.T) {
	c, _ := newTestCache(t, 30*time.Second)
	c.Put("p1", settings("allow"))
	c.Evict("p1")
	_, state := c.Get("p1")
	assert.Equal(t, Miss, state)
	assert.Equal(t, 0, c.Len())
}

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

func TestBreaker_OneProbePerInterval(t *testing.T) {
	c, now := newTestCache(t, 30*time.Second)
	assert.True(t, c.FetchAllowed(), "healthy store: every fetch allowed")
	assert.True(t, c.FetchAllowed())

	c.StoreFailed()
	assert.False(t, c.FetchAllowed(), "just failed: no fetch until the probe interval passes")

	*now = now.Add(DefaultProbeInterval - time.Millisecond)
	assert.False(t, c.FetchAllowed())

	*now = now.Add(time.Millisecond)
	assert.True(t, c.FetchAllowed(), "first caller after the interval probes")
	assert.False(t, c.FetchAllowed(), "second caller in the same interval does not")

	*now = now.Add(DefaultProbeInterval)
	assert.True(t, c.FetchAllowed(), "probe re-arms once per interval while failing")

	c.StoreRecovered()
	assert.True(t, c.FetchAllowed())
	assert.True(t, c.FetchAllowed(), "recovered: no gating")
}
