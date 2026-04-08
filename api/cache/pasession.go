package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// AddPASession stores a PASession in Redis with the given expiration.
// Key format: pasession:{sessionId}
func (c *RedisCache) AddPASession(ctx context.Context, session *model.PASession, expiresIn time.Duration) error {
	key := fmt.Sprintf("pasession:%s", session.ID)
	data, err := json.Marshal(session)
	if err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to marshal PA session")
		return err
	}
	if err := c.client.Set(ctx, key, string(data), expiresIn).Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to add PA session")
		return err
	}
	log.Info().Str("key", key).Dur("expires_in", expiresIn).Msg("Cache: added PA session")
	return nil
}

// GetPASession retrieves a PASession from Redis by session ID.
func (c *RedisCache) GetPASession(ctx context.Context, sessionID string) (*model.PASession, error) {
	key := fmt.Sprintf("pasession:%s", sessionID)
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to get PA session")
		return nil, err
	}
	var session model.PASession
	if err := json.Unmarshal([]byte(val), &session); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to unmarshal PA session")
		return nil, err
	}
	return &session, nil
}

// RemovePASession deletes a PASession from Redis.
func (c *RedisCache) RemovePASession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("pasession:%s", sessionID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		log.Err(err).Str("key", key).Msg("Cache: failed to remove PA session")
		return err
	}
	log.Info().Str("key", key).Msg("Cache: removed PA session")
	return nil
}
