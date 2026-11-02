package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/api/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newStatsReadCache(t *testing.T) (*StatisticsReadCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewStatisticsReadCache(NewRedisCacheFromClient(client)), mr
}

// specRef: api-endpoint-behaviour.md J45 - key shape (profile, generation, timespan), 60 s TTL, miss vs hit.
func TestStatisticsReadCache_SetGetTTL(t *testing.T) {
	c, mr := newStatsReadCache(t)
	ctx := context.Background()

	_, found, err := c.GetStatistics(ctx, "p1", "LAST_1_DAY", 0)
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, c.SetStatistics(ctx, "p1", "LAST_1_DAY", 0, []byte(`{"a":1}`)))
	require.True(t, mr.Exists("statistics:read:p1:g0:LAST_1_DAY"))
	require.Equal(t, StatisticsReadTTL, mr.TTL("statistics:read:p1:g0:LAST_1_DAY"))

	got, found, err := c.GetStatistics(ctx, "p1", "LAST_1_DAY", 0)
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, `{"a":1}`, string(got))
}

// specRef: api-endpoint-behaviour.md J46 - invalidation moves the profile to a new generation, so
// none of its entries is read again; other profiles keep theirs.
func TestStatisticsReadCache_InvalidateProfile(t *testing.T) {
	c, _ := newStatsReadCache(t)
	ctx := context.Background()
	for _, ts := range model.StatisticsTimespans() {
		require.NoError(t, c.SetStatistics(ctx, "p1", ts, 0, []byte("x")))
		require.NoError(t, c.SetStatistics(ctx, "p2", ts, 0, []byte("x")))
	}

	require.NoError(t, c.InvalidateStatistics(ctx, "p1"))
	gen1, err := c.Generation(ctx, "p1")
	require.NoError(t, err)
	require.EqualValues(t, 1, gen1)
	gen2, err := c.Generation(ctx, "p2")
	require.NoError(t, err)
	for _, ts := range model.StatisticsTimespans() {
		_, found, err := c.GetStatistics(ctx, "p1", ts, gen1)
		require.NoError(t, err)
		require.False(t, found, ts)
		_, found, err = c.GetStatistics(ctx, "p2", ts, gen2)
		require.NoError(t, err)
		require.True(t, found, ts)
	}
}

// specRef: api-endpoint-behaviour.md J46 - an answer computed before an invalidation and written
// after it is never served: it lands under the generation read before the query.
func TestStatisticsReadCache_LateWriteAfterInvalidationIsNeverRead(t *testing.T) {
	c, _ := newStatsReadCache(t)
	ctx := context.Background()

	before, err := c.Generation(ctx, "p1")
	require.NoError(t, err)
	require.NoError(t, c.InvalidateStatistics(ctx, "p1"))
	require.NoError(t, c.SetStatistics(ctx, "p1", "LAST_1_DAY", before, []byte(`{"stale":true}`)))

	now, err := c.Generation(ctx, "p1")
	require.NoError(t, err)
	_, found, err := c.GetStatistics(ctx, "p1", "LAST_1_DAY", now)
	require.NoError(t, err)
	require.False(t, found)
}

type failingBase struct{ CacheBase }

func (failingBase) Get(context.Context, string) (string, error) { return "", errors.New("down") }

// specRef: api-endpoint-behaviour.md J45 — a real error is reported (not read as a miss) so the caller can log it.
func TestStatisticsReadCache_ErrorsSurface(t *testing.T) {
	c := NewStatisticsReadCache(failingBase{})
	_, found, err := c.GetStatistics(context.Background(), "p1", "LAST_1_DAY", 0)
	require.Error(t, err)
	require.False(t, found)
	_, err = c.Generation(context.Background(), "p1")
	require.Error(t, err)
}
