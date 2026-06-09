package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SubscriptionRepository represents a Subscription repository.
//
// Find* methods that take no filter beyond a flag (FindExpiredUnnotified,
// FindInactiveUnnotified, FindWithLANotified, FindWithInactiveNotified)
// are coarse pre-filters. Predicate logic lives in model.Subscription;
// callers MUST post-filter results via sub.GetStatus(). See
// docs/specs/subscription-lifecycle-enforcement.md.
type SubscriptionRepository interface {
	GetSubscriptionByAccountId(ctx context.Context, accountId string) (*model.Subscription, error)
	Upsert(ctx context.Context, subscription model.Subscription) error
	Create(ctx context.Context, subscription model.Subscription) error
	ClearLegacyType(ctx context.Context, accountId string) error
	DeleteSubscriptionByAccountId(ctx context.Context, accountId string) error

	// Reset account methods for the signup-reset flow. See docs/specs/signup-reset-behaviour.md.
	FindActiveByTokenHash(ctx context.Context, tokenHash string, excludeAccountID primitive.ObjectID) ([]model.Subscription, error)
	MarkSubscriptionRetired(ctx context.Context, subscriptionID uuid.UUID, when time.Time) error
	FindScheduledForDeletion(ctx context.Context, before time.Time) ([]model.Subscription, error)
	FindDuplicateTokenHashGroups(ctx context.Context) ([]model.DuplicateTokenHashGroup, error)

	// FindExpiredUnnotified returns LA-candidate subs with notified=false.
	// Coarse pre-filter: cron must check sub.GetStatus() == StatusLimitedAccess.
	FindExpiredUnnotified(ctx context.Context) ([]model.Subscription, error)

	// FindInactiveUnnotified returns Inactive-candidate subs with notified_inactive=false.
	// Coarse pre-filter: cron must check sub.GetStatus() == StatusInactive.
	FindInactiveUnnotified(ctx context.Context) ([]model.Subscription, error)

	// FindWithLANotified returns all subs with notified=true.
	// The LA cron iterates and clears the flag for any whose GetStatus() no
	// longer returns StatusLimitedAccess (i.e. transitioned back to active).
	FindWithLANotified(ctx context.Context) ([]model.Subscription, error)

	// FindWithInactiveNotified returns all subs with notified_inactive=true.
	// The inactive cron iterates and clears the flag for any whose GetStatus() no
	// longer returns StatusInactive.
	FindWithInactiveNotified(ctx context.Context) ([]model.Subscription, error)

	// SetNotified sets the `notified` field to `value` for the given IDs.
	SetNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error

	// SetInactiveNotified sets the `notified_inactive` field to `value` for the given IDs.
	SetInactiveNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error
}
