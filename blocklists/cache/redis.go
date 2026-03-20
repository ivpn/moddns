package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivpn/dns/libs/cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const chunkSize = 5000

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

	pipe := c.client.Pipeline()

	// Step 1: Create the temp set with new data
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
	}

	// Step 2: Atomically swap sets using RENAME
	// If the original blocklist exists, rename it to _old
	renameNXCmd := pipe.RenameNX(ctx, blocklistName, oldBlocklistName)
	// Rename temp set to the main blocklist name
	pipe.Rename(ctx, tempBlocklistName, blocklistName)
	// Step 3: Delete the old set
	pipe.Del(ctx, oldBlocklistName)

	// Commit all commands in the pipeline
	cmds, err := pipe.Exec(ctx)
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
			log.Info().
				Str("component", "cache").
				Str("blocklist_key", blocklistName).
				Msg("Created/updated blocklist with atomic swap (ignored 'no such key' error)")
			return nil
		}
		// Otherwise, return the pipeline error
		log.Err(err).Str("component", "cache").Msg("Cache: pipeline execution failed")
		return err
	}

	log.Info().
		Str("component", "cache").
		Str("blocklist_key", blocklistName).
		Msgf("Created/updated blocklist with atomic swap using temp and old sets")
	return nil
}

// DeleteBlocklist removes a blocklist set from the cache
func (c *RedisCache) DeleteBlocklist(ctx context.Context, blocklistId string) error {
	key := fmt.Sprintf("blocklist:%s", blocklistId)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	log.Info().Str("component", "cache").Str("blocklist_key", key).Msg("Deleted blocklist from cache")
	return nil
}
