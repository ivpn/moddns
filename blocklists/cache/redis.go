package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/dislock"
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
	// stagingTTL is set on the per-run staging key so a run that dies between
	// populate and swap cannot leave an orphaned set behind forever. It must
	// exceed the longest population time; the swap clears it from the live key.
	stagingTTL = 30 * time.Minute
)

// swapScript promotes a populated staging set to the live key in one atomic
// step: RENAME replaces the destination, and PERSIST drops the staging TTL the
// key carries over — RENAME preserves the source key's TTL, and the live set
// must never expire. Running both in one script means no interleaving client
// can observe (or a crash leave behind) a live set with a TTL.
var swapScript = redis.NewScript(`
redis.call("RENAME", KEYS[1], KEYS[2])
redis.call("PERSIST", KEYS[2])
return 1
`)

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

// Locker returns a distributed locker backed by this cache's Redis client,
// namespaced under prefix. Instances coordinating through it must share the
// same Redis (they do: all blocklists instances write the Sentinel master).
func (c *RedisCache) Locker(prefix string) *dislock.Locker {
	return dislock.New(c.client, prefix)
}

// CreateOrUpdateBlocklist adds a blocklist to the cache, replacing the existing
// set if it exists. Data is staged into a key unique to this run — concurrent
// writers (peer instances racing on the same source) can therefore never
// interleave on one staging set — and promoted with an atomic swap.
func (c *RedisCache) CreateOrUpdateBlocklist(ctx context.Context, blocklistId string, data []byte) error {
	blocklistName := fmt.Sprintf("blocklist:%s", blocklistId)
	stagingName := fmt.Sprintf("%s:tmp:%s", blocklistName, uuid.NewString())

	// Step 1: Populate the staging set with new data.
	if err := c.populateStaging(ctx, stagingName, data); err != nil {
		c.discardStaging(ctx, stagingName)
		return err
	}

	// Step 2: Atomically swap the populated staging set into place.
	if err := c.swapBlocklist(ctx, stagingName, blocklistName); err != nil {
		c.discardStaging(ctx, stagingName)
		return err
	}

	log.Debug().
		Str("component", "cache").
		Str("blocklist_key", blocklistName).
		Msg("Created/updated blocklist with atomic swap")
	return nil
}

// populateStaging fills the staging set, flushing in bounded batches, and
// bounds the key's lifetime with stagingTTL so an interrupted run self-cleans.
func (c *RedisCache) populateStaging(ctx context.Context, stagingName string, data []byte) error {
	pipe := c.client.Pipeline()
	buffered := 0

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
		pipe.SAdd(ctx, stagingName, chunk)
		buffered++
		if i == 0 {
			// The first SAdd creates the key; expire it right after so a run
			// that dies mid-population self-cleans.
			pipe.Expire(ctx, stagingName, stagingTTL)
			buffered++
		}

		if buffered >= flushEvery {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
				return err
			}
			pipe = c.client.Pipeline()
			buffered = 0
		}
	}
	// Flush any commands still buffered.
	if buffered > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
			return err
		}
	}
	return nil
}

// swapBlocklist promotes the populated staging set to the live blocklist key.
// The single RENAME+PERSIST script atomically replaces the destination, so no
// MULTI/EXEC is needed — and would not help anyway: Redis transactions do not
// abort on runtime errors, so a multi-step swap could still drop the live set
// if the staging key is missing (e.g. expired, or lost to a Sentinel failover
// between population and swap). If the staging key is gone the RENAME fails
// and the live set is left untouched.
func (c *RedisCache) swapBlocklist(ctx context.Context, stagingName, liveName string) error {
	// Best-effort removal of _old and _temp keys orphaned by interrupted runs
	// of previous code versions, which staged under these fixed names. Current
	// staging keys are per-run and TTL-bound, so neither name is ever live.
	if err := c.client.Unlink(ctx, fmt.Sprintf("%s_old", liveName), fmt.Sprintf("%s_temp", liveName)).Err(); err != nil {
		log.Debug().Err(err).Str("component", "cache").Msg("Cache: failed to unlink orphaned legacy keys")
	}

	if err := swapScript.Run(ctx, c.client, []string{stagingName, liveName}).Err(); err != nil {
		log.Err(err).Str("component", "cache").Str("blocklist_key", liveName).Msg("Cache: blocklist swap failed")
		return err
	}
	return nil
}

// discardStaging best-effort deletes a staging set after a failed run; the
// stagingTTL remains the backstop when Redis is unreachable.
func (c *RedisCache) discardStaging(ctx context.Context, stagingName string) {
	if err := c.client.Unlink(ctx, stagingName).Err(); err != nil {
		log.Debug().Err(err).Str("component", "cache").Str("staging_key", stagingName).Msg("Cache: failed to discard staging key")
	}
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
