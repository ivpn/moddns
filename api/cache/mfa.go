package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// SetTOTPSecret sets the TOTP secret for the given account
func (c *RedisCache) SetTOTPSecret(ctx context.Context, accountId, secret string, expiresIn time.Duration) error {
	totpKey := fmt.Sprintf("totp:%s", accountId)

	intCmd := c.client.Set(ctx, totpKey, secret, expiresIn)
	if err := intCmd.Err(); err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to create TOTP secret")
		return err
	}
	log.Ctx(ctx).Trace().Msg("Created totp secret")
	return nil
}

// GetTOTPSecret gets the TOTP secret for the given account
func (c *RedisCache) GetTOTPSecret(ctx context.Context, accountId string) (string, error) {
	totpKey := fmt.Sprintf("totp:%s", accountId)

	strCmd := c.client.Get(ctx, totpKey)
	if err := strCmd.Err(); err != nil {
		log.Ctx(ctx).Err(err).Msg("Cache: failed to get TOTP secret")
		return "", err
	}
	secret := strCmd.Val()
	log.Ctx(ctx).Trace().Msg("Got totp secret")
	return secret, nil
}
