package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newQueueCache(t *testing.T) (*RedisCache, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCacheFromClient(client), client
}

var queueEpoch = time.Unix(1_800_000_000, 0).UTC()

// specRef: api-endpoint-behaviour.md J9 — only due members are returned, earliest first, up to the limit.
func TestStatisticsPurgeQueue_DueReturnsOnlyDueMembersInOrder(t *testing.T) {
	c, _ := newQueueCache(t)
	ctx := context.Background()

	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "later", queueEpoch.Add(2*time.Minute)))
	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "first", queueEpoch))
	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "second", queueEpoch.Add(time.Minute)))
	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "future", queueEpoch.Add(time.Hour)))

	got, err := c.DueStatisticsPurges(ctx, queueEpoch.Add(2*time.Minute), 10)
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second", "later"}, members(got))
	require.True(t, got[0].Due.Equal(queueEpoch))

	limited, err := c.DueStatisticsPurges(ctx, queueEpoch.Add(2*time.Minute), 2)
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, members(limited))
}

// specRef: api-endpoint-behaviour.md J9 — re-enqueueing keeps one member and only ever pushes its due time later.
func TestStatisticsPurgeQueue_EnqueueKeepsLaterDue(t *testing.T) {
	c, client := newQueueCache(t)
	ctx := context.Background()

	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "m", queueEpoch.Add(10*time.Minute)))
	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "m", queueEpoch.Add(5*time.Minute)))
	score, err := client.ZScore(ctx, statisticsPurgeQueueKey, "m").Result()
	require.NoError(t, err)
	require.EqualValues(t, queueEpoch.Add(10*time.Minute).Unix(), score, "an earlier due time must not pull the member forward")

	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "m", queueEpoch.Add(20*time.Minute)))
	score, err = client.ZScore(ctx, statisticsPurgeQueueKey, "m").Result()
	require.NoError(t, err)
	require.EqualValues(t, queueEpoch.Add(20*time.Minute).Unix(), score)

	n, err := client.ZCard(ctx, statisticsPurgeQueueKey).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}

// specRef: api-endpoint-behaviour.md J10 — removal deletes exactly the given member and tolerates absence.
func TestStatisticsPurgeQueue_RemoveDeletesExactMember(t *testing.T) {
	c, _ := newQueueCache(t)
	ctx := context.Background()

	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "a", queueEpoch))
	require.NoError(t, c.EnqueueStatisticsPurge(ctx, "b", queueEpoch))
	require.NoError(t, c.RemoveStatisticsPurge(ctx, "a"))
	require.NoError(t, c.RemoveStatisticsPurge(ctx, "never-queued"))

	got, err := c.DueStatisticsPurges(ctx, queueEpoch, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"b"}, members(got))
}

func members(entries []StatisticsPurgeEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Member)
	}
	return out
}
