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
	oldBlocklistName := fmt.Sprintf("%s_old", blocklistName)

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

	// Step 2: Atomically swap the populated temp set into place. A pipeline is
	// not a transaction, so this is kept separate from population; the only
	// atomic unit that matters is each individual RENAME.
	swap := c.client.Pipeline()
	// If the original blocklist exists, rename it to _old
	renameNXCmd := swap.RenameNX(ctx, blocklistName, oldBlocklistName)
	// Rename temp set to the main blocklist name
	swap.Rename(ctx, tempBlocklistName, blocklistName)
	// Step 3: Delete the old set
	swap.Del(ctx, oldBlocklistName)

	// Commit the swap commands in the pipeline
	cmds, err := swap.Exec(ctx)
	if err != nil {
		// Check if the only error is "ERR no such key" from RENAME or RENAME_NX
		ignore := false
		for _, cmd := range cmds {
			if cmd.Err() != nil {
				// Only ignore "ERR no such key" from RENAME/RENAME_NX
				if strings.Contains(cmd.Err().Error(), "no such key") {
					// If this is the RENAME_NX command, we can ignore it
					if cmd == renameNXCmd {
						ignore = true
						continue
					}
				}
				// If it's any other error, or from another command, do not ignore
				log.Err(cmd.Err()).Str("component", "cache").Msg("Cache: pipeline command error")
				return cmd.Err()
			}
		}
		// If all errors were ignorable, treat as success
		if ignore {
			log.Debug().
				Str("component", "cache").
				Str("blocklist_key", blocklistName).
				Msg("Created/updated blocklist with atomic swap (ignored 'no such key' error)")
			return nil
		}
		// Otherwise, return the pipeline error
		log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
		return err
	}

	log.Debug().
		Str("component", "cache").
		Str("blocklist_key", blocklistName).
		Msgf("Created/updated blocklist with atomic swap using temp and old sets")
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
