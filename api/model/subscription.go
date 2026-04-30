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

const Tier1 = "Tier 1"

// Subscription represents a subscription with its properties
type Subscription struct {
	// ID is the primary key (UUIDv4) stored in Mongo _id
	ID          uuid.UUID          `json:"-" bson:"_id"`
	AccountID   primitive.ObjectID `json:"-" bson:"account_id"`
	ActiveUntil time.Time          `json:"active_until" bson:"active_until"`
	IsActive    bool               `json:"-" bson:"is_active"`
	Tier        string             `json:"tier,omitempty" bson:"tier,omitempty"`
	TokenHash   string             `json:"-" bson:"token_hash,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Notified              bool               `json:"-" bson:"notified"`
	NotifiedPendingDelete bool               `json:"-" bson:"notified_pending_delete"`
	Limits                SubscriptionLimits `json:"-" bson:"limits"`

	// Computed fields (not persisted)
	Status SubscriptionStatus `json:"status" bson:"-"`
	Outage bool               `json:"outage" bson:"-"`
}

// Active returns true when the subscription is valid: not expired, not Tier1, and no outage.
func (s *Subscription) Active() bool {
	return s.ActiveUntil.After(time.Now()) && !strings.Contains(s.Tier, Tier1) && !s.IsOutage()
}

// GracePeriod returns true during a sync outage when both 3-day grace windows still hold.
func (s *Subscription) GracePeriod() bool {
	return s.IsOutage() && s.GracePeriodDays(3) && s.OutageGracePeriodDays(3)
}

// LimitedAccess returns true when at least one 14-day grace period is still active.
func (s *Subscription) LimitedAccess() bool {
	return s.GracePeriodDays(14) || s.OutageGracePeriodDays(14)
}

// PendingDelete returns true when both 14-day grace periods have been exceeded.
func (s *Subscription) PendingDelete() bool {
	return !s.GracePeriodDays(14) || !s.OutageGracePeriodDays(14)
}

// ActiveStatus returns true when the subscription permits normal operations (Active or GracePeriod).
func (s *Subscription) ActiveStatus() bool {
	return s.Active() || s.GracePeriod()
}

// IsOutage returns true when the subscription hasn't been updated in over 48 hours.
// Returns false for zero UpdatedAt (never-synced pre-ZLA accounts) to avoid
// incorrectly degrading paid subscriptions that haven't gone through ZLA sync yet.
func (s *Subscription) IsOutage() bool {
	if s.UpdatedAt.IsZero() {
		return false
	}
	return s.UpdatedAt.Add(48 * time.Hour).Before(time.Now())
}

// GracePeriodDays returns true when ActiveUntil + days is still in the future.
func (s *Subscription) GracePeriodDays(days int) bool {
	return s.ActiveUntil.AddDate(0, 0, days).After(time.Now())
}

// OutageGracePeriodDays returns true when UpdatedAt + days is still in the future.
func (s *Subscription) OutageGracePeriodDays(days int) bool {
	return s.UpdatedAt.AddDate(0, 0, days).After(time.Now())
}

// GetStatus computes the current lifecycle status.
func (s *Subscription) GetStatus() SubscriptionStatus {
	if s.Active() {
		return StatusActive
	}
	if s.GracePeriod() {
		return StatusGracePeriod
	}
	if s.LimitedAccess() {
		return StatusLimitedAccess
	}
	return StatusPendingDelete
}

type SubscriptionLimits struct {
	MaxQueriesPerMonth int `json:"max_queries_per_month" bson:"max_queries_per_month"`
}

// SubscriptionUpdate represents subscription update
// RFC6902 JSON Patch format is used
type SubscriptionUpdate struct {
	Operation string `json:"operation" validate:"required,oneof=remove add replace move copy"`
	Path      string `json:"path" validate:"required,oneof=/not_implemented"`
	Value     any    `json:"value" validate:"required"`
}
