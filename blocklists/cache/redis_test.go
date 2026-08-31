package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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
// not yet exist (RENAME creates the live key).
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

// TestCreateOrUpdateBlocklist_ClearsStaleTemp ensures a fixed-name _temp set
// left behind by a crashed run of a previous code version is discarded rather
// than merged into the new set (current staging keys are per-run and
// TTL-bound, so the legacy name can only be an orphan).
func TestCreateOrUpdateBlocklist_ClearsStaleTemp(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	// Simulate a crashed prior run of the legacy code that left a partial temp set.
	require.NoError(t, rc.client.SAdd(ctx, "blocklist:stale_temp", "garbage.com").Err())

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "stale", []byte("real.com")))

	members, err := rc.client.SMembers(ctx, "blocklist:stale").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"real.com"}, members)
	assert.NotContains(t, members, "garbage.com")
	assert.False(t, mr.Exists("blocklist:stale_temp"))
}

// specRef: #H2 — DeleteBlocklist removes exactly the targeted blocklist:{id}
// key, leaves other blocklists untouched, and is a no-op for an absent key.
func TestDeleteBlocklist(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "gone", []byte("a.com\nb.com")))
	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "kept", []byte("c.com")))

	require.NoError(t, rc.DeleteBlocklist(ctx, "gone"))

	assert.False(t, mr.Exists("blocklist:gone"), "deleted blocklist key must be gone")
	assert.True(t, mr.Exists("blocklist:kept"), "unrelated blocklist must survive")

	// DEL on a missing key succeeds; a repeated purge must not error.
	assert.NoError(t, rc.DeleteBlocklist(ctx, "gone"))
}

// specRef: #F2 — a failed promote must leave the live set untouched.
// The temp set can vanish between population and swap (Sentinel failover,
// Redis restart); the swap must then fail without destroying the live list.
func TestSwapBlocklist_MissingTemp_PreservesLiveList(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.client.SAdd(ctx, "blocklist:live", "keep1.com", "keep2.com").Err())
	require.False(t, mr.Exists("blocklist:live:tmp:gone"))

	err := rc.swapBlocklist(ctx, "blocklist:live:tmp:gone", "blocklist:live")
	assert.Error(t, err, "swap with a missing temp set must fail")

	members, merr := rc.client.SMembers(ctx, "blocklist:live").Result()
	require.NoError(t, merr)
	assert.ElementsMatch(t, []string{"keep1.com", "keep2.com"}, members,
		"live set must survive a failed swap")
	assert.False(t, mr.Exists("blocklist:live_old"))
}

// specRef: #F2 — promote atomically overwrites an existing live set and leaves
// no temp/old residue.
func TestSwapBlocklist_OverwritesExistingLive(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.client.SAdd(ctx, "blocklist:x", "old.com").Err())
	require.NoError(t, rc.client.SAdd(ctx, "blocklist:x:tmp:run1", "new1.com", "new2.com").Err())

	require.NoError(t, rc.swapBlocklist(ctx, "blocklist:x:tmp:run1", "blocklist:x"))

	members, err := rc.client.SMembers(ctx, "blocklist:x").Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"new1.com", "new2.com"}, members)
	assert.False(t, mr.Exists("blocklist:x:tmp:run1"))
	assert.False(t, mr.Exists("blocklist:x_old"))
}

// specRef: #F2 — an _old key orphaned by an interrupted legacy swap is cleaned
// up by the next successful swap.
func TestSwapBlocklist_CleansOrphanedOldKey(t *testing.T) {
	rc, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.client.SAdd(ctx, "blocklist:y_old", "orphan.com").Err())
	require.NoError(t, rc.client.SAdd(ctx, "blocklist:y_temp", "legacy-orphan.com").Err())
	require.NoError(t, rc.client.SAdd(ctx, "blocklist:y:tmp:run1", "new.com").Err())

	require.NoError(t, rc.swapBlocklist(ctx, "blocklist:y:tmp:run1", "blocklist:y"))

	assert.False(t, mr.Exists("blocklist:y_old"), "orphaned _old key must be removed")
	assert.False(t, mr.Exists("blocklist:y_temp"), "orphaned legacy _temp key must be removed")
}

// makePrefixedDomains builds n newline-separated domains with a distinguishing
// prefix so concurrent-writer tests can tell whose set won the swap.
func makePrefixedDomains(prefix string, n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s%d.example.com", prefix, i)
	}
	return []byte(b.String())
}

// specRef: #F3 — two writers racing on the same blocklist must each stage into
// their own per-run key: whichever swap lands last wins with a COMPLETE set,
// never an interleaved or truncated one. Both inputs span multiple pipeline
// flushes so the two populations genuinely interleave in time.
func TestCreateOrUpdateBlocklist_ConcurrentWritersDoNotCorrupt(t *testing.T) {
	rc, _ := newTestCache(t)
	ctx := context.Background()

	// 260k entries => 52 SADD commands => more than one pipeline flush each.
	const n = 260_000
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for idx, prefix := range []string{"a", "b"} {
		wg.Add(1)
		go func(idx int, prefix string) {
			defer wg.Done()
			errs[idx] = rc.CreateOrUpdateBlocklist(ctx, "race", makePrefixedDomains(prefix, n))
		}(idx, prefix)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	card, err := rc.client.SCard(ctx, "blocklist:race").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(n), card, "live set must be exactly one writer's complete set")

	// The set must belong entirely to one writer: probe first and last member
	// of each candidate set.
	aFirst := rc.client.SIsMember(ctx, "blocklist:race", "a0.example.com").Val()
	aLast := rc.client.SIsMember(ctx, "blocklist:race", fmt.Sprintf("a%d.example.com", n-1)).Val()
	bFirst := rc.client.SIsMember(ctx, "blocklist:race", "b0.example.com").Val()
	bLast := rc.client.SIsMember(ctx, "blocklist:race", fmt.Sprintf("b%d.example.com", n-1)).Val()
	assert.True(t, (aFirst && aLast && !bFirst && !bLast) || (bFirst && bLast && !aFirst && !aLast),
		"live set mixes writers: aFirst=%v aLast=%v bFirst=%v bLast=%v", aFirst, aLast, bFirst, bLast)
}

// specRef: #F1 — the staging key is created with a bounded lifetime so an
// interrupted run cannot orphan it forever.
func TestPopulateStaging_SetsBoundedTTL(t *testing.T) {
	rc, _ := newTestCache(t)
	ctx := context.Background()

	staging := "blocklist:ttltest:tmp:fixed"
	require.NoError(t, rc.populateStaging(ctx, staging, []byte("a.com\nb.com")))

	ttl, err := rc.client.TTL(ctx, staging).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0), "staging key must carry a TTL")
	assert.LessOrEqual(t, ttl, stagingTTL)
}

// specRef: #F2 — the promoted live key must never expire: the swap script
// clears the staging TTL that RENAME would otherwise carry over.
func TestCreateOrUpdateBlocklist_LiveKeyHasNoTTL(t *testing.T) {
	rc, _ := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, rc.CreateOrUpdateBlocklist(ctx, "persist", []byte("a.com\nb.com")))

	ttl, err := rc.client.TTL(ctx, "blocklist:persist").Result()
	require.NoError(t, err)
	assert.Equal(t, time.Duration(-1), ttl, "live key must be persistent (TTL -1)")
}
