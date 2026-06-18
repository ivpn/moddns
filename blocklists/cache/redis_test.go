package cache

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return &RedisCache{client: rdb}, mr
}

func makeDomains(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "d%d.example.com", i)
	}
	return []byte(b.String())
}

// TestCreateOrUpdateBlocklist_LargeInputMultipleFlushes verifies that an input
// large enough to span more than one pipeline flush batch is stored completely
// and leaves no temp/old residue. Guards the batched-flush refactor.
func TestCreateOrUpdateBlocklist_LargeInputMultipleFlushes(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	// 260k entries => 52 SADD commands => crosses the flushEvery boundary,
	// forcing at least two pipeline Exec round-trips.
	const n = 260_000
	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "big", makeDomains(n)))

	card, err := rc.client.SCard(ctx, "blocklist:big").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(n), card)

	ok, err := rc.client.SIsMember(ctx, "blocklist:big", "d259999.example.com").Result()
	require.NoError(t, err)
	assert.True(t, ok, "last member should be present after multiple flushes")

	assert.False(t, mr.Exists("blocklist:big_temp"), "temp set must not survive")
	assert.False(t, mr.Exists("blocklist:big_old"), "old set must not survive")
}

// TestCreateOrUpdateBlocklist_FirstRun covers the path where the target set does
// not yet exist (RENAMENX hits "no such key", which must be ignored).
func TestCreateOrUpdateBlocklist_FirstRun(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "fresh", []byte("a.com\nb.com\nc.com")))

	members, err := rc.client.SMembers(ctx, "blocklist:fresh").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"a.com", "b.com", "c.com"}, members)
	assert.False(t, mr.Exists("blocklist:fresh_temp"))
	assert.False(t, mr.Exists("blocklist:fresh_old"))
}

// TestCreateOrUpdateBlocklist_ReplacesExisting verifies the atomic swap fully
// replaces a previously stored set (no stale members from the old version).
func TestCreateOrUpdateBlocklist_ReplacesExisting(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "swap", []byte("old1.com\nold2.com")))
	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "swap", []byte("new1.com\nnew2.com\nnew3.com")))

	members, err := rc.client.SMembers(ctx, "blocklist:swap").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"new1.com", "new2.com", "new3.com"}, members)
	assert.False(t, mr.Exists("blocklist:swap_temp"))
	assert.False(t, mr.Exists("blocklist:swap_old"))
}

// TestCreateOrUpdateBlocklist_ClearsStaleTemp ensures a temp set left behind by a
// previously crashed run is discarded rather than merged into the new set.
func TestCreateOrUpdateBlocklist_ClearsStaleTemp(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	// Simulate a crashed prior run that left a partial temp set.
	require.NoError(t, rc.client.SAdd(ctx, "blocklist:stale_temp", "garbage.com").Err())

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "stale", []byte("real.com")))

	members, err := rc.client.SMembers(ctx, "blocklist:stale").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"real.com"}, members)
	assert.NotContains(t, members, "garbage.com")
	assert.False(t, mr.Exists("blocklist:stale_temp"))
}
