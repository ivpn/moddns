package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// StatisticsReadTTL bounds how stale a cached statistics answer can be.
const StatisticsReadTTL = 60 * time.Second

// statisticsReadScope namespaces the generation counter of the statistics read cache.
const statisticsReadScope = "statistics:read"

// StatisticsReadKey is the key of a profile's cached answer for a timespan under a generation.
func StatisticsReadKey(profileId string, gen int64, timespan string) string {
	return statisticsReadScope + ":" + profileId + ":g" + strconv.FormatInt(gen, 10) + ":" + timespan
}

// StatisticsReadCache stores the JSON answers of the statistics read endpoint.
// Errors are returned to the caller, which treats the cache as best effort.
type StatisticsReadCache struct {
	base CacheBase
}

func NewStatisticsReadCache(base CacheBase) *StatisticsReadCache {
	return &StatisticsReadCache{base: base}
}

// Generation returns the profile's current generation; read it before querying and pass it to
// GetStatistics and SetStatistics.
func (c *StatisticsReadCache) Generation(ctx context.Context, profileId string) (int64, error) {
	return Generation(ctx, c.base, statisticsReadScope, profileId)
}

// GetStatistics returns the cached answer; found is false on a miss.
func (c *StatisticsReadCache) GetStatistics(ctx context.Context, profileId, timespan string, gen int64) ([]byte, bool, error) {
	val, err := c.base.Get(ctx, StatisticsReadKey(profileId, gen, timespan))
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(val), true, nil
}

func (c *StatisticsReadCache) SetStatistics(ctx context.Context, profileId, timespan string, gen int64, payload []byte) error {
	return c.base.Set(ctx, StatisticsReadKey(profileId, gen, timespan), payload, StatisticsReadTTL)
}

// InvalidateStatistics moves the profile to a new generation: every answer cached so far, and
// any still being computed, is never read again and expires on its TTL.
func (c *StatisticsReadCache) InvalidateStatistics(ctx context.Context, profileId string) error {
	return BumpGeneration(ctx, c.base, statisticsReadScope, profileId)
}
