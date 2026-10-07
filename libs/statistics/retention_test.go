package statistics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J48
func TestRetention(t *testing.T) {
	require.Equal(t, []Retention{Retention30d, Retention90d, Retention1y}, Retentions())
	for _, tc := range []struct {
		in     Retention
		valid  bool
		window time.Duration
		effect Retention
	}{
		{Retention30d, true, 30 * 24 * time.Hour, Retention30d},
		{Retention90d, true, 90 * 24 * time.Hour, Retention90d},
		{Retention1y, true, 365 * 24 * time.Hour, Retention1y},
		{"", false, 30 * 24 * time.Hour, Retention30d},
		{"1m", false, 30 * 24 * time.Hour, Retention30d},
	} {
		require.Equal(t, tc.valid, tc.in.Valid(), tc.in)
		require.Equal(t, tc.effect, tc.in.OrDefault(), tc.in)
		require.Equal(t, tc.window, tc.in.Window(), tc.in)
	}
}

// specRef: api-endpoint-behaviour.md J48 — go-redis encodes the value through MarshalBinary.
func TestRetentionMarshalBinary(t *testing.T) {
	b, err := Retention90d.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, "90d", string(b))
}
