package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newGenerationBase(t *testing.T) (CacheBase, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCacheFromClient(client), mr
}

// specRef: api-endpoint-behaviour.md J23, J46 - a profile's generation starts at 0, each bump
// raises it by one, and profiles and scopes are independent.
func TestGeneration_StartsAtZeroAndBumps(t *testing.T) {
	base, mr := newGenerationBase(t)
	ctx := context.Background()

	gen, err := Generation(ctx, base, "logs", "p1")
	require.NoError(t, err)
	require.Zero(t, gen)

	require.NoError(t, BumpGeneration(ctx, base, "logs", "p1"))
	require.NoError(t, BumpGeneration(ctx, base, "logs", "p1"))
	gen, err = Generation(ctx, base, "logs", "p1")
	require.NoError(t, err)
	require.EqualValues(t, 2, gen)
	require.Equal(t, GenerationTTL, mr.TTL(GenerationKey("logs", "p1")))

	for _, other := range [][2]string{{"logs", "p2"}, {"statistics:read", "p1"}} {
		gen, err = Generation(ctx, base, other[0], other[1])
		require.NoError(t, err)
		require.Zero(t, gen, other)
	}
}

// specRef: api-endpoint-behaviour.md J23 - an unreadable generation is an error, never 0, so a
// caller cannot mistake a Redis failure for "never purged".
func TestGeneration_ErrorsSurface(t *testing.T) {
	_, err := Generation(context.Background(), failingBase{}, "logs", "p1")
	require.Error(t, err)

	base, mr := newGenerationBase(t)
	mr.Set(GenerationKey("logs", "p1"), "not-a-number")
	_, err = Generation(context.Background(), base, "logs", "p1")
	require.Error(t, err)
}

// The generation outlives every entry versioned by it, so an expiry can only reset it after
// those entries are gone.
func TestGenerationTTL_OutlivesEntries(t *testing.T) {
	require.Greater(t, GenerationTTL, StatisticsReadTTL+time.Hour)
	require.Greater(t, GenerationTTL, 5*time.Minute+time.Hour)
}
