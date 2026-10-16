package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

func ceremonyKey(token string) string {
	return fmt.Sprintf("webauthn_ceremony:%s", token)
}

// SaveCeremony stores WebAuthn ceremony state under its token for the given expiration.
func (c *RedisCache) SaveCeremony(ctx context.Context, token string, ceremony *model.Ceremony, expiresIn time.Duration) error {
	data, err := json.Marshal(ceremony)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to marshal WebAuthn ceremony")
		return err
	}
	if err := c.client.Set(ctx, ceremonyKey(token), string(data), expiresIn).Err(); err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to save WebAuthn ceremony")
		return err
	}
	return nil
}

// ConsumeCeremony reads and deletes WebAuthn ceremony state in one step, so each
// challenge is used at most once (W3C WebAuthn Level 3 §13.4.3).
func (c *RedisCache) ConsumeCeremony(ctx context.Context, token string) (*model.Ceremony, error) {
	val, err := c.client.GetDel(ctx, ceremonyKey(token)).Result()
	if err != nil {
		return nil, err
	}
	var ceremony model.Ceremony
	if err := json.Unmarshal([]byte(val), &ceremony); err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to unmarshal WebAuthn ceremony")
		return nil, err
	}
	return &ceremony, nil
}
