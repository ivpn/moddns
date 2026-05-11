package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/model"
)

// SubscriptionRepository represents a Subscription repository.
//
// Find* methods that take no filter beyond a flag (FindExpiredUnnotified,
// FindPendingDeleteUnnotified, FindWithLANotified, FindWithPendingDeleteNotified)
// are coarse pre-filters. Predicate logic lives in model.Subscription;
// callers MUST post-filter results via sub.GetStatus(). See
// docs/specs/subscription-lifecycle-enforcement.md.
type SubscriptionRepository interface {
	GetSubscriptionByAccountId(ctx context.Context, accountId string) (*model.Subscription, error)
	Upsert(ctx context.Context, subscription model.Subscription) error
	Create(ctx context.Context, subscription model.Subscription) error
	ClearLegacyType(ctx context.Context, accountId string) error
	DeleteSubscriptionByAccountId(ctx context.Context, accountId string) error

	// FindExpiredUnnotified returns LA-candidate subs with notified=false.
	// Coarse pre-filter: cron must check sub.GetStatus() == StatusLimitedAccess.
	FindExpiredUnnotified(ctx context.Context) ([]model.Subscription, error)

	// FindPendingDeleteUnnotified returns PD-candidate subs with notified_pending_delete=false.
	// Coarse pre-filter: cron must check sub.GetStatus() == StatusPendingDelete.
	FindPendingDeleteUnnotified(ctx context.Context) ([]model.Subscription, error)

	// FindWithLANotified returns all subs with notified=true.
	// The LA cron iterates and clears the flag for any whose GetStatus() no
	// longer returns StatusLimitedAccess (i.e. transitioned back to active).
	FindWithLANotified(ctx context.Context) ([]model.Subscription, error)

	// FindWithPendingDeleteNotified returns all subs with notified_pending_delete=true.
	// The PD cron iterates and clears the flag for any whose GetStatus() no
	// longer returns StatusPendingDelete.
	FindWithPendingDeleteNotified(ctx context.Context) ([]model.Subscription, error)

	// SetNotified sets the `notified` field to `value` for the given IDs.
	SetNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error

	// SetPendingDeleteNotified sets the `notified_pending_delete` field to `value` for the given IDs.
	SetPendingDeleteNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error
}
