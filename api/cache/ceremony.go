package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/ivpn/dns/api/internal/webauthn"
)

func ceremonyKey(token string) string {
	return fmt.Sprintf("webauthn_ceremony:%s", token)
}

// SaveCeremony stores WebAuthn ceremony state under its token for the given expiration.
func (c *RedisCache) SaveCeremony(ctx context.Context, token string, state []byte, expiresIn time.Duration) error {
	if err := c.client.Set(ctx, ceremonyKey(token), state, expiresIn).Err(); err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to save WebAuthn ceremony")
		return err
	}
	return nil
}

// ConsumeCeremony reads and deletes WebAuthn ceremony state in one step, so each
// challenge is used at most once (W3C WebAuthn Level 3 §13.4.3).
func (c *RedisCache) ConsumeCeremony(ctx context.Context, token string) ([]byte, error) {
	state, err := c.client.GetDel(ctx, ceremonyKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, webauthn.ErrCeremonyNotFound
	}
	return state, err
}
