package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/model"
)

// SubscriptionRepository represents a Subscription repository
type SubscriptionRepository interface {
	GetSubscriptionByAccountId(ctx context.Context, accountId string) (*model.Subscription, error)
	Upsert(ctx context.Context, subscription model.Subscription) error
	Create(ctx context.Context, subscription model.Subscription) error
	ClearLegacyType(ctx context.Context, accountId string) error
	ResetNotifiedForActive(ctx context.Context) error
	FindExpiredUnnotified(ctx context.Context) ([]model.Subscription, error)
	MarkNotified(ctx context.Context, subscriptionIDs []uuid.UUID) error
	FindPendingDeleteUnnotified(ctx context.Context) ([]model.Subscription, error)
	MarkPendingDeleteNotified(ctx context.Context, subscriptionIDs []uuid.UUID) error
	ResetPendingDeleteNotifiedForActive(ctx context.Context) error
}
