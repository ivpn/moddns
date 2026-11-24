package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIncrSetsExpiryOnFirstIncrement(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t)

	n, err := c.Incr(ctx, "totp_fails:a", time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Equal(t, time.Hour, mr.TTL("totp_fails:a"))

	mr.FastForward(10 * time.Minute)
	n, err = c.Incr(ctx, "totp_fails:a", time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	require.Equal(t, 50*time.Minute, mr.TTL("totp_fails:a"), "later increments keep the window")
}

// A counter left without an expiry must not live forever.
func TestIncrRepairsMissingExpiry(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t)
	require.NoError(t, mr.Set("totp_fails:a", "7"))

	n, err := c.Incr(ctx, "totp_fails:a", time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(8), n)
	require.Equal(t, time.Hour, mr.TTL("totp_fails:a"))
}

func TestIncrWithoutExpiration(t *testing.T) {
	ctx := context.Background()
	c, mr := newTestCache(t)

	_, err := c.Incr(ctx, "counter", 0)
	require.NoError(t, err)
	require.Zero(t, mr.TTL("counter"))
}
