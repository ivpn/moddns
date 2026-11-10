package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/internal/webauthn"
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
	require.ErrorIs(t, err, webauthn.ErrCeremonyNotFound)
}

// specRef: F9, F11
func TestConsumeCeremonyUnknownToken(t *testing.T) {
	c, _ := newTestCache(t)

	_, err := c.ConsumeCeremony(context.Background(), "unknown")
	require.ErrorIs(t, err, webauthn.ErrCeremonyNotFound)
}

// specRef: F11
func TestConsumeCeremonyStoreFailure(t *testing.T) {
	c, mr := newTestCache(t)
	mr.Close()

	_, err := c.ConsumeCeremony(context.Background(), "token")
	require.Error(t, err)
	require.NotErrorIs(t, err, webauthn.ErrCeremonyNotFound)
}
