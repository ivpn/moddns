package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const statisticsPurgeQueueKey = "statistics:purge:queue"

// StatisticsPurgeEntry is one queued purge: an opaque member and when it is due.
type StatisticsPurgeEntry struct {
	Member string
	Due    time.Time
}

// StatisticsPurgeQueue is the shared, restart-safe queue of delayed statistics
// purges: a Redis sorted set scored by due unix time. It is a dedicated narrow
// interface rather than part of Cache so the statistics service does not depend
// on (or mock) the whole profile-settings cache.
type StatisticsPurgeQueue interface {
	// EnqueueStatisticsPurge adds the member, or moves an existing one to a later
	// due time; it never pulls a member forward.
	EnqueueStatisticsPurge(ctx context.Context, member string, due time.Time) error
	// DueStatisticsPurges returns up to limit members due at or before now, earliest first.
	DueStatisticsPurges(ctx context.Context, now time.Time, limit int) ([]StatisticsPurgeEntry, error)
	// RemoveStatisticsPurge deletes exactly the given member; an absent member is not an error.
	RemoveStatisticsPurge(ctx context.Context, member string) error
}

var _ StatisticsPurgeQueue = (*RedisCache)(nil)

func (c *RedisCache) EnqueueStatisticsPurge(ctx context.Context, member string, due time.Time) error {
	return c.client.ZAddGT(ctx, statisticsPurgeQueueKey, redis.Z{Score: float64(due.Unix()), Member: member}).Err()
}

func (c *RedisCache) DueStatisticsPurges(ctx context.Context, now time.Time, limit int) ([]StatisticsPurgeEntry, error) {
	zs, err := c.client.ZRangeByScoreWithScores(ctx, statisticsPurgeQueueKey, &redis.ZRangeBy{
		Min: "-inf", Max: strconv.FormatInt(now.Unix(), 10), Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]StatisticsPurgeEntry, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		entries = append(entries, StatisticsPurgeEntry{Member: member, Due: time.Unix(int64(z.Score), 0).UTC()})
	}
	return entries, nil
}

func (c *RedisCache) RemoveStatisticsPurge(ctx context.Context, member string) error {
	return c.client.ZRem(ctx, statisticsPurgeQueueKey, member).Err()
}
