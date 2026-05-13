package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SubscriptionRepository is a MongoDB repository for subscription collection
type SubscriptionRepository struct {
	DbName                  string
	CollectionName          string
	subscriptionsCollection *mongo.Collection
}

// NewSubscriptionRepository creates a new SubscriptionRepository instance
func NewSubscriptionRepository(client *mongo.Client, dbName, collectionName string) SubscriptionRepository {
	collection := client.Database(dbName).Collection(collectionName)

	return SubscriptionRepository{
		DbName:                  dbName,
		CollectionName:          collectionName,
		subscriptionsCollection: collection,
	}
}

func (r *SubscriptionRepository) GetSubscriptionByAccountId(ctx context.Context, accountId string) (*model.Subscription, error) {
	// account_id is stored as a MongoDB ObjectID; convert incoming hex string
	objID, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		// Treat invalid ObjectID as not found to avoid leaking validation details
		return nil, errors.ErrSubscriptionNotFound
	}
	filter := bson.D{{Key: "account_id", Value: objID}}
	var subscription model.Subscription
	if err := r.subscriptionsCollection.FindOne(ctx, filter).Decode(&subscription); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrSubscriptionNotFound
		}
		return nil, err
	}
	return &subscription, nil
}

// Upsert creates or updates a subscription in the subscriptions collection
func (r *SubscriptionRepository) Upsert(ctx context.Context, subscription model.Subscription) error {
	filter := bson.M{"account_id": subscription.AccountID}
	update := bson.M{"$set": subscription}
	opts := options.Update().SetUpsert(true)
	res, err := r.subscriptionsCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	log.Debug().Str("component", "mongoDB").Interface("result", res).Msg("Upserted subscription")
	return nil
}

// ClearLegacyType writes type="" on the account's subscription document so the
// pre-0.1.8 banner stops rendering after a successful resync. Set semantics
// (vs $unset) preserve a deterministic, queryable empty-string sentinel.
func (r *SubscriptionRepository) ClearLegacyType(ctx context.Context, accountId string) error {
	objID, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return errors.ErrSubscriptionNotFound
	}
	filter := bson.M{"account_id": objID}
	update := bson.M{"$set": bson.M{"type": ""}}
	_, err = r.subscriptionsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Str("account_id", accountId).Msg("Failed to clear legacy subscription type")
	}
	return err
}

// Create inserts a new subscription; fails if already exists
func (r *SubscriptionRepository) Create(ctx context.Context, sub model.Subscription) error {
	_, err := r.subscriptionsCollection.InsertOne(ctx, sub)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return errors.ErrSubscriptionAlreadyExists
		}
		return err
	}
	return nil
}

// ResetNotifiedForActive sets notified=false for subscriptions that are genuinely
// Active per model.Subscription.Active(): active_until in the future AND updated_at
// recent enough that IsOutage() returns false (within the last 48h). Tier1 is a
// string check the cron filters in Go via GetStatus().
func (r *SubscriptionRepository) ResetNotifiedForActive(ctx context.Context) error {
	now := time.Now()
	filter := bson.M{
		"active_until": bson.M{"$gte": now},
		"updated_at":   bson.M{"$gte": now.Add(-48 * time.Hour)},
	}
	update := bson.M{"$set": bson.M{"notified": false}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reset notified flag for active subscriptions")
	}
	return err
}

// FindExpiredUnnotified returns subscriptions that may be in LimitedAccess (per
// model.Subscription.LimitedAccess()) and have not been notified yet. It is a
// coarse pre-filter: matches any sub whose active_until elapsed >24h ago OR
// whose updated_at is >48h old (outage-triggered LA path). The caller (the cron)
// must additionally verify sub.GetStatus() == StatusLimitedAccess so GracePeriod
// and PendingDelete subs are not emailed as LA.
func (r *SubscriptionRepository) FindExpiredUnnotified(ctx context.Context) ([]model.Subscription, error) {
	now := time.Now()
	filter := bson.M{
		"notified": false,
		"$or": []bson.M{
			{"active_until": bson.M{"$lt": now.Add(-24 * time.Hour)}},
			{"updated_at": bson.M{"$lt": now.Add(-48 * time.Hour)}},
		},
	}
	cursor, err := r.subscriptionsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []model.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// MarkNotified sets notified=true for the given subscription IDs.
func (r *SubscriptionRepository) MarkNotified(ctx context.Context, subscriptionIDs []uuid.UUID) error {
	if len(subscriptionIDs) == 0 {
		return nil
	}
	filter := bson.M{"_id": bson.M{"$in": subscriptionIDs}}
	update := bson.M{"$set": bson.M{"notified": true}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mark subscriptions as notified")
	}
	return err
}

// FindPendingDeleteUnnotified returns subscriptions where the model considers
// the sub PendingDelete (active_until + 14d < now OR updated_at + 14d < now)
// and notified_pending_delete is still false. This mirrors model.Subscription.PendingDelete()
// exactly so no Go-side post-filter is required.
func (r *SubscriptionRepository) FindPendingDeleteUnnotified(ctx context.Context) ([]model.Subscription, error) {
	fourteenDaysAgo := time.Now().AddDate(0, 0, -14)
	filter := bson.M{
		"notified_pending_delete": false,
		"$or": []bson.M{
			{"active_until": bson.M{"$lt": fourteenDaysAgo}},
			{"updated_at": bson.M{"$lt": fourteenDaysAgo}},
		},
	}
	cursor, err := r.subscriptionsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []model.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// MarkPendingDeleteNotified sets notified_pending_delete=true for the given subscription IDs.
func (r *SubscriptionRepository) MarkPendingDeleteNotified(ctx context.Context, subscriptionIDs []uuid.UUID) error {
	if len(subscriptionIDs) == 0 {
		return nil
	}
	filter := bson.M{"_id": bson.M{"$in": subscriptionIDs}}
	update := bson.M{"$set": bson.M{"notified_pending_delete": true}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mark subscriptions as pending delete notified")
	}
	return err
}

// ResetPendingDeleteNotifiedForActive sets notified_pending_delete=false for
// subscriptions that are genuinely Active again (active_until in future AND
// updated_at within the last 48h, mirroring model.Subscription.Active()).
func (r *SubscriptionRepository) ResetPendingDeleteNotifiedForActive(ctx context.Context) error {
	now := time.Now()
	filter := bson.M{
		"active_until": bson.M{"$gte": now},
		"updated_at":   bson.M{"$gte": now.Add(-48 * time.Hour)},
	}
	update := bson.M{"$set": bson.M{"notified_pending_delete": false}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reset notified_pending_delete flag for active subscriptions")
	}
	return err
}
