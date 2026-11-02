package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J54, J56 — each logs retention keeps rows for its collection's TTL; unknown reads as 1h.
func TestRetentionWindow(t *testing.T) {
	cases := map[Retention]time.Duration{
		RetentionOneHour:  time.Hour,
		RetentionSixHours: 6 * time.Hour,
		RetentionOneDay:   24 * time.Hour,
		RetentionOneWeek:  7 * 24 * time.Hour,
		RetentionOneMonth: 30 * 24 * time.Hour,
		"":                time.Hour,
		"2y":              time.Hour,
	}
	for r, want := range cases {
		require.Equal(t, want, r.Window(), "retention %q", r)
	}
}

// specRef: api-endpoint-behaviour.md J54 — the retentions are listed shortest first.
func TestRetentions_ShortestFirst(t *testing.T) {
	rs := Retentions()
	require.Equal(t, []Retention{RetentionOneHour, RetentionSixHours, RetentionOneDay, RetentionOneWeek, RetentionOneMonth}, rs)
	for i := 1; i < len(rs); i++ {
		require.Less(t, rs[i-1].Window(), rs[i].Window())
	}
}
