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

// specRef: api-endpoint-behaviour.md J45 — key shape, 60 s TTL, miss vs hit.
func TestStatisticsReadCache_SetGetTTL(t *testing.T) {
	c, mr := newStatsReadCache(t)
	ctx := context.Background()

	_, found, err := c.GetStatistics(ctx, "p1", "LAST_1_DAY")
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, c.SetStatistics(ctx, "p1", "LAST_1_DAY", []byte(`{"a":1}`)))
	require.True(t, mr.Exists("statistics:read:p1:LAST_1_DAY"))
	require.Equal(t, StatisticsReadTTL, mr.TTL("statistics:read:p1:LAST_1_DAY"))

	got, found, err := c.GetStatistics(ctx, "p1", "LAST_1_DAY")
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, `{"a":1}`, string(got))
}

// specRef: api-endpoint-behaviour.md J46 — invalidation removes every timespan of the profile and only that profile.
func TestStatisticsReadCache_InvalidateProfile(t *testing.T) {
	c, mr := newStatsReadCache(t)
	ctx := context.Background()
	for _, ts := range model.StatisticsTimespans() {
		require.NoError(t, c.SetStatistics(ctx, "p1", ts, []byte("x")))
		require.NoError(t, c.SetStatistics(ctx, "p2", ts, []byte("x")))
	}

	require.NoError(t, c.InvalidateStatistics(ctx, "p1"))
	for _, ts := range model.StatisticsTimespans() {
		require.False(t, mr.Exists(StatisticsReadKey("p1", ts)), ts)
		require.True(t, mr.Exists(StatisticsReadKey("p2", ts)), ts)
	}
}

type failingBase struct{ CacheBase }

func (failingBase) Get(context.Context, string) (string, error) { return "", errors.New("down") }

// specRef: api-endpoint-behaviour.md J45 — a real error is reported (not read as a miss) so the caller can log it.
func TestStatisticsReadCache_ErrorsSurface(t *testing.T) {
	_, found, err := NewStatisticsReadCache(failingBase{}).GetStatistics(context.Background(), "p1", "LAST_1_DAY")
	require.Error(t, err)
	require.False(t, found)
}
