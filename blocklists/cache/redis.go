package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivpn/dns/libs/cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// chunkSize is the number of members added per SADD command.
	chunkSize = 5000
	// flushEvery is the number of buffered commands sent per pipeline
	// round-trip. The full blocklist is never flushed as a single batch: large
	// NRD lists hold millions of entries, and writing the whole pipeline under
	// one socket write deadline overwhelms the master's drain rate and triggers
	// "i/o timeout". Flushing in bounded batches keeps each round-trip small and
	// gives each its own write deadline (~250k members per flush at chunkSize).
	flushEvery = 50
)

// RedisCache is a cache implementation using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache instance
func NewRedisCache(cfg *cache.Config) (*RedisCache, error) {
	rdb, err := cache.NewRedisClient(cfg)
	if err != nil {
		return nil, err
	}

	return &RedisCache{
		client: rdb,
	}, nil
}

// CreateOrUpdateBlocklist adds a blocklist to the cache, replacing the existing set if it exists
// Uses a temp set and atomic renames to ensure safe updates.
func (c *RedisCache) CreateOrUpdateBlocklist(ctx context.Context, blocklistId string, data []byte) error {
	blocklistName := fmt.Sprintf("blocklist:%s", blocklistId)
	tempBlocklistName := fmt.Sprintf("%s_temp", blocklistName)

	// Step 1: Populate the temp set with new data, flushing in bounded batches.
	pipe := c.client.Pipeline()

	// Step 0: Clear any stale temp set left by a previously crashed run, so the
	// new set is not silently merged with old data.
	pipe.Del(ctx, tempBlocklistName)
	buffered := 1 // the Del command above

	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunk := lines[i:end]
		// Skip empty chunk (can happen if data ends with newline)
		if len(chunk) == 0 {
			continue
		}
		pipe.SAdd(ctx, tempBlocklistName, chunk)

		if buffered++; buffered >= flushEvery {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
				return err
			}
			pipe = c.client.Pipeline()
			buffered = 0
		}
	}
	// Flush any commands still buffered (the initial Del plus trailing SADDs).
	if buffered > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
			return err
		}
	}

	// Step 2: Atomically swap the populated temp set into place.
	if err := c.swapBlocklist(ctx, tempBlocklistName, blocklistName); err != nil {
		return err
	}

	log.Debug().
		Str("component", "cache").
		Str("blocklist_key", blocklistName).
		Msg("Created/updated blocklist with atomic swap")
	return nil
}

// swapBlocklist promotes the populated temp set to the live blocklist key.
// A single RENAME atomically replaces the destination key, so no MULTI/EXEC
// is needed — and would not help anyway: Redis transactions do not abort on
// runtime errors, so a multi-step swap could still drop the live set if the
// temp key is missing (e.g. lost to a Sentinel failover between population
// and swap). If the temp key is gone the RENAME fails and the live set is
// left untouched.
func (c *RedisCache) swapBlocklist(ctx context.Context, tempName, liveName string) error {
	// Best-effort removal of an _old key orphaned by an interrupted swap of a
	// previous code version, which staged the live set under this name.
	if err := c.client.Unlink(ctx, fmt.Sprintf("%s_old", liveName)).Err(); err != nil {
		log.Debug().Err(err).Str("component", "cache").Msg("Cache: failed to unlink orphaned _old key")
	}

	if err := c.client.Rename(ctx, tempName, liveName).Err(); err != nil {
		log.Err(err).Str("component", "cache").Str("blocklist_key", liveName).Msg("Cache: blocklist swap failed")
		return err
	}
	return nil
}

// Ping reports whether the Redis backend is reachable.
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// DeleteBlocklist removes a blocklist set from the cache
func (c *RedisCache) DeleteBlocklist(ctx context.Context, blocklistId string) error {
	key := fmt.Sprintf("blocklist:%s", blocklistId)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	log.Debug().Str("component", "cache").Str("blocklist_key", key).Msg("Deleted blocklist from cache")
	return nil
}
