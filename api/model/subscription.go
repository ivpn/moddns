package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SubscriptionStatus represents the computed lifecycle state of a subscription.
type SubscriptionStatus string

const (
	StatusActive        SubscriptionStatus = "active"
	StatusGracePeriod   SubscriptionStatus = "grace_period"
	StatusLimitedAccess SubscriptionStatus = "limited_access"
	StatusPendingDelete SubscriptionStatus = "pending_delete"
)

const (
	// Tier1 is the legacy substring IVPN uses for the Standard plan,
	// e.g. "IVPN Tier 1".
	Tier1 = "Tier 1"
	// TierStandard is the product-name substring IVPN may also use
	// for the same Standard plan, e.g. "IVPN Standard".
	TierStandard = "Standard"
)

// RetiredAccountRetention is the grace window between an account being
// scheduled for deletion by the signup-reset flow (DeletionScheduledAt set)
// and its hard deletion by the DeleteRetiredAccounts cron. It gives the user
// time to export data.
const RetiredAccountRetention = 48 * time.Hour

// hasStandardTier reports whether the tier string identifies the IVPN
// Standard plan, which is not entitled to modDNS. IVPN may send the plan
// name as either "IVPN Tier 1" / "IVPN Tier 1 Lite" or as "IVPN Standard";
// either substring identifies the same (terminal PD) product. Centralised
// here so all callers (Active, PendingDelete) stay in sync; the Mongo
// pre-filter in FindPendingDeleteUnnotified mirrors this rule with
// regex `"Tier 1|Standard"`.
func hasStandardTier(tier string) bool {
	return strings.Contains(tier, Tier1) || strings.Contains(tier, TierStandard)
}

// Subscription represents a subscription with its properties
type Subscription struct {
	// ID is the primary key (UUIDv4) stored in Mongo _id
	ID          uuid.UUID          `json:"-" bson:"_id"`
	AccountID   primitive.ObjectID `json:"-" bson:"account_id"`
	ActiveUntil time.Time          `json:"active_until" bson:"active_until"`
	IsActive    bool               `json:"-" bson:"is_active"`
	// Type is a legacy pre-0.1.8 enum ("Free"/"Managed") retained so old documents
	// surface to clients (the beta-ending banner gates on Type == "Managed").
	// Cleared to "" by the resync flow once the user re-syncs with IVPN.
	Type                  string             `json:"type,omitempty" bson:"type,omitempty"`
	Tier                  string             `json:"tier,omitempty" bson:"tier,omitempty"`
	TokenHash             string             `json:"-" bson:"token_hash,omitempty"`
	UpdatedAt             time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Notified              bool               `json:"-" bson:"notified"`
	NotifiedPendingDelete bool               `json:"-" bson:"notified_pending_delete"`
	Limits                SubscriptionLimits `json:"-" bson:"limits"`
	// DeletionScheduledAt is set when the signup-reset flow schedules an account
	// for deletion. Exposed in the GET /sub JSON (omitted when nil) so the webapp
	// can detect the retired state directly — a non-null value means "retired".
	// It also forces GetStatus() to pending_delete (row L0) and drives the
	// DeleteRetiredAccounts cron. See docs/specs/signup-reset-behaviour.md.
	DeletionScheduledAt *time.Time `json:"deletion_scheduled_at,omitempty" bson:"deletion_scheduled_at,omitempty"`

	// Computed fields (not persisted)
	Status SubscriptionStatus `json:"status" bson:"-"`
	Outage bool               `json:"outage" bson:"-"`
}

func (s *Subscription) Active() bool {
	return s.ActiveUntil.After(time.Now()) && !hasStandardTier(s.Tier) && !s.IsOutage() && s.DeletionScheduledAt == nil
}

func (s *Subscription) GracePeriod() bool {
	if s.DeletionScheduledAt != nil {
		return false
	}
	return s.IsOutage() && s.GracePeriodDays(3) && s.OutageGracePeriodDays(3)
}

func (s *Subscription) LimitedAccess() bool {
	if s.DeletionScheduledAt != nil {
		return false
	}
	return s.GracePeriodDays(14) || (s.OutageGracePeriodDays(14) && s.IsOutage())
}

func (s *Subscription) PendingDelete() bool {
	if s.DeletionScheduledAt != nil {
		return true
	}

	if hasStandardTier(s.Tier) {
		return true
	}

	if s.UpdatedAt.AddDate(0, 0, 14).Before(time.Now()) {
		return true
	}

	if s.ActiveUntil.AddDate(0, 0, 14).Before(time.Now()) {
		return true
	}

	return false
}

func (s *Subscription) ActiveStatus() bool {
	return s.Active() || s.GracePeriod()
}

func (s *Subscription) IsOutage() bool {
	if s.UpdatedAt.IsZero() {
		return false
	}

	return s.UpdatedAt.Add(time.Duration(48) * time.Hour).Before(time.Now())
}

func (s *Subscription) GracePeriodDays(days int) bool {
	return s.ActiveUntil.AddDate(0, 0, days).After(time.Now()) && s.ActiveUntil.Before(time.Now())
}

func (s *Subscription) OutageGracePeriodDays(days int) bool {
	return s.UpdatedAt.AddDate(0, 0, days).After(time.Now()) && s.UpdatedAt.Before(time.Now())
}

func (s *Subscription) GetStatus() SubscriptionStatus {
	// L0: an account retired by the signup-reset flow is unconditionally
	// pending_delete, independent of dates/tier/outage. Highest precedence.
	if s.DeletionScheduledAt != nil {
		return StatusPendingDelete
	}
	if s.Active() {
		return StatusActive
	}
	if s.GracePeriod() {
		return StatusGracePeriod
	}
	if s.PendingDelete() {
		return StatusPendingDelete
	}
	if s.LimitedAccess() {
		return StatusLimitedAccess
	}
	return StatusPendingDelete
}

type SubscriptionLimits struct {
	MaxQueriesPerMonth int `json:"max_queries_per_month" bson:"max_queries_per_month"`
}

// DuplicateTokenHashGroup is an aggregation result: a single token_hash held by
// more than one NON-retired subscription. That violates the signup-reset
// invariant (≤1 active account per IVPN customer) and indicates either a
// pre-existing duplicate or a retirement that failed/raced. Surfaced read-only
// by the reconciliation report job; see docs/specs/signup-reset-behaviour.md.
type DuplicateTokenHashGroup struct {
	TokenHash  string               `bson:"_id"`
	Count      int                  `bson:"count"`
	AccountIDs []primitive.ObjectID `bson:"account_ids"`
}

// SubscriptionUpdate represents subscription update
// RFC6902 JSON Patch format is used
type SubscriptionUpdate struct {
	Operation string `json:"operation" validate:"required,oneof=remove add replace move copy"`
	Path      string `json:"path" validate:"required,oneof=/not_implemented"`
	Value     any    `json:"value" validate:"required"`
}
