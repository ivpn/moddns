package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGetStatus_DecisionTable exercises every row L1-L4 of the lifecycle
// decision table in docs/specs/subscription-lifecycle-enforcement.md, plus
// the Tier 1 fresh-dates case that motivates PendingDelete()'s tier short-circuit.
//
// specRef: subscription-lifecycle-enforcement.md L1-L5
func TestGetStatus_DecisionTable(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name        string
		tier        string
		activeUntil time.Time
		updatedAt   time.Time
		want        SubscriptionStatus
		specRef     string
	}{
		{
			name:        "L1 active paid Tier 2",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusActive,
			specRef:     "L1",
		},
		{
			name:        "L1 active paid Tier 3 (sanity)",
			tier:        "IVPN Tier 3",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusActive,
			specRef:     "L1",
		},
		{
			name:        "L2 grace period (outage within 3-day windows)",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(-1 * 24 * time.Hour),
			updatedAt:   now.Add(-50 * time.Hour),
			want:        StatusGracePeriod,
			specRef:     "L2",
		},
		{
			name:        "L3 PD via stale active_until",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(-15 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L3 PD via stale updated_at",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now.Add(-15 * 24 * time.Hour),
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L3 PD via Tier 1 with fresh dates (IVPN Standard, legacy name)",
			tier:        "IVPN Tier 1",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L3 PD via Tier 1 with stale dates",
			tier:        "IVPN Tier 1",
			activeUntil: now.Add(-20 * 24 * time.Hour),
			updatedAt:   now.Add(-20 * 24 * time.Hour),
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L3 PD via IVPN Standard product name with fresh dates",
			tier:        "IVPN Standard",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L3 PD via standalone Standard string with fresh dates",
			tier:        "Standard",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusPendingDelete,
			specRef:     "L3",
		},
		{
			name:        "L4 limited access via active_until past within 14d",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(-1 * 24 * time.Hour),
			updatedAt:   now,
			want:        StatusLimitedAccess,
			specRef:     "L4",
		},
		{
			name:        "L4 limited access via outage path",
			tier:        "IVPN Tier 2",
			activeUntil: now.Add(30 * 24 * time.Hour),
			updatedAt:   now.Add(-9 * 24 * time.Hour),
			want:        StatusLimitedAccess,
			specRef:     "L4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Subscription{
				Tier:        tc.tier,
				ActiveUntil: tc.activeUntil,
				UpdatedAt:   tc.updatedAt,
			}
			assert.Equal(t, tc.want, s.GetStatus(), "specRef=%s", tc.specRef)
		})
	}
}

// TestPendingDelete_StandardPlan verifies the Standard-plan short-circuit
// in isolation. IVPN may emit the tier as "IVPN Tier 1", "IVPN Tier 1 Lite",
// or "IVPN Standard"; any of these substrings must trigger PendingDelete.
//
// specRef: subscription-lifecycle-enforcement.md L3 (Standard-plan trigger)
func TestPendingDelete_StandardPlan(t *testing.T) {
	now := time.Now()
	freshDates := func(s *Subscription) {
		s.ActiveUntil = now.Add(30 * 24 * time.Hour)
		s.UpdatedAt = now
	}

	cases := []struct {
		name string
		tier string
		want bool
	}{
		{"IVPN Tier 1", "IVPN Tier 1", true},
		{"IVPN Tier 1 Lite variant", "IVPN Tier 1 Lite", true},
		{"plain Tier 1", "Tier 1", true},
		{"IVPN Standard product name", "IVPN Standard", true},
		{"standalone Standard", "Standard", true},
		{"IVPN Tier 2 (not PD on fresh dates)", "IVPN Tier 2", false},
		{"IVPN Tier 3 (not PD on fresh dates)", "IVPN Tier 3", false},
		{"empty tier (not PD on fresh dates)", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Subscription{Tier: tc.tier}
			freshDates(&s)
			assert.Equal(t, tc.want, s.PendingDelete())
		})
	}
}

// TestActive_ExcludesStandardPlan confirms the Standard-plan carve-out in
// Active() — a sub on the Standard plan with fresh dates must never appear
// as `active` to any caller of GetStatus(), under either naming convention.
//
// specRef: subscription-lifecycle-enforcement.md L1 (Active definition)
func TestActive_ExcludesStandardPlan(t *testing.T) {
	now := time.Now()
	for _, tier := range []string{"IVPN Tier 1", "IVPN Standard"} {
		t.Run(tier, func(t *testing.T) {
			s := Subscription{
				Tier:        tier,
				ActiveUntil: now.Add(30 * 24 * time.Hour),
				UpdatedAt:   now,
			}
			assert.False(t, s.Active(), "Standard plan must never be Active even with fresh dates")
		})
	}
}
