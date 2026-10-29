package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
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
	return c.client.GetDel(ctx, ceremonyKey(token)).Bytes()
}
