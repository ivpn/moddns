package cache

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/redis/go-redis/v9"
)

// StatisticsReadTTL bounds how stale a cached statistics answer can be.
const StatisticsReadTTL = 60 * time.Second

// StatisticsReadKey is the key of a profile's cached answer for a timespan.
func StatisticsReadKey(profileId, timespan string) string {
	return "statistics:read:" + profileId + ":" + timespan
}

// StatisticsReadCache stores the JSON answers of the statistics read endpoint.
// Errors are returned to the caller, which treats the cache as best effort.
type StatisticsReadCache struct {
	base CacheBase
}

func NewStatisticsReadCache(base CacheBase) *StatisticsReadCache {
	return &StatisticsReadCache{base: base}
}

// GetStatistics returns the cached answer; found is false on a miss.
func (c *StatisticsReadCache) GetStatistics(ctx context.Context, profileId, timespan string) ([]byte, bool, error) {
	val, err := c.base.Get(ctx, StatisticsReadKey(profileId, timespan))
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(val), true, nil
}

func (c *StatisticsReadCache) SetStatistics(ctx context.Context, profileId, timespan string, payload []byte) error {
	return c.base.Set(ctx, StatisticsReadKey(profileId, timespan), payload, StatisticsReadTTL)
}

// InvalidateStatistics deletes the answer of every timespan; the key space is
// the fixed timespan list, so no key scan is needed. Every key is attempted.
func (c *StatisticsReadCache) InvalidateStatistics(ctx context.Context, profileId string) error {
	var errs []error
	for _, timespan := range model.StatisticsTimespans() {
		if err := c.base.Del(ctx, StatisticsReadKey(profileId, timespan)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
