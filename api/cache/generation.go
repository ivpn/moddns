package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// GenerationTTL bounds how long a profile's generation counter is kept after it is first set.
// It must outlive every cache entry versioned by it: when it expires the generation reads as 0
// again, which is safe only once all entries written under 0 have expired.
const GenerationTTL = 7 * 24 * time.Hour

// GenerationKey is the Redis key of a profile's generation counter within a cache scope.
//
// Cache entries derived from a profile's stored data carry the generation read before the data
// was queried. Purging bumps it, so an answer computed from data read before the purge is
// written under a generation no later read uses, however late that write lands.
func GenerationKey(scope, profileId string) string {
	return scope + "-gen:" + profileId
}

// Generation returns the profile's current generation in scope, 0 when none is stored.
func Generation(ctx context.Context, base CacheBase, scope, profileId string) (int64, error) {
	raw, err := base.Get(ctx, GenerationKey(scope, profileId))
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

// BumpGeneration makes every entry written under the profile's current generation unreachable.
func BumpGeneration(ctx context.Context, base CacheBase, scope, profileId string) error {
	_, err := base.Incr(ctx, GenerationKey(scope, profileId), GenerationTTL)
	return err
}
