package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRedisCacheFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()})), mr
}

// specRef: F9
func TestCeremonyIsSingleUse(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t)

	state := []byte(`{"type":"reauth","account_id":"acc-1"}`)
	require.NoError(t, c.SaveCeremony(ctx, "token", state, 5*time.Minute))

	require.False(t, mr.Exists("token"), "ceremony state must not be stored under the bare token")
	require.Equal(t, 5*time.Minute, mr.TTL("webauthn_ceremony:token"))

	got, err := c.ConsumeCeremony(ctx, "token")
	require.NoError(t, err)
	require.Equal(t, state, got)

	_, err = c.ConsumeCeremony(ctx, "token")
	require.ErrorIs(t, err, redis.Nil)
}

// specRef: F9
func TestConsumeCeremonyUnknownToken(t *testing.T) {
	c, _ := newTestCache(t)

	_, err := c.ConsumeCeremony(context.Background(), "unknown")
	require.ErrorIs(t, err, redis.Nil)
}
