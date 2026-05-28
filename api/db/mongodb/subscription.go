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

// FindActiveByTokenHash returns subscriptions whose token_hash equals tokenHash
// and whose account_id differs from excludeAccountID. Retired subscriptions
// have token_hash $unset and therefore never match. Backed by the partial
// index on token_hash (migration 018). See signup-reset-behaviour.md RT3.
func (r *SubscriptionRepository) FindActiveByTokenHash(ctx context.Context, tokenHash string, excludeAccountID primitive.ObjectID) ([]model.Subscription, error) {
	filter := bson.M{
		"token_hash": tokenHash,
		"account_id": bson.M{"$ne": excludeAccountID},
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

// MarkSubscriptionRetired unsets token_hash and sets deletion_scheduled_at on
// the subscription. $unset (not set-to-empty) keeps the doc out of the partial
// token_hash index and out of FindActiveByTokenHash matches. See
// signup-reset-behaviour.md rows RT5/RT6.
func (r *SubscriptionRepository) MarkSubscriptionRetired(ctx context.Context, subscriptionID uuid.UUID, when time.Time) error {
	filter := bson.M{"_id": subscriptionID}
	update := bson.M{
		"$set":   bson.M{"deletion_scheduled_at": when},
		"$unset": bson.M{"token_hash": ""},
	}
	if _, err := r.subscriptionsCollection.UpdateOne(ctx, filter, update); err != nil {
		log.Error().Err(err).Str("subscription_id", subscriptionID.String()).Msg("Failed to mark subscription retired")
		return err
	}
	return nil
}

// FindScheduledForDeletion returns subscriptions whose deletion_scheduled_at is
// at or before `before`. Used by the DeleteRetiredAccounts cron.
func (r *SubscriptionRepository) FindScheduledForDeletion(ctx context.Context, before time.Time) ([]model.Subscription, error) {
	filter := bson.M{"deletion_scheduled_at": bson.M{"$lte": before}}
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

// FindDuplicateTokenHashGroups returns every token_hash shared by more than one
// non-retired subscription. READ-ONLY (aggregation, no writes). Non-retired =
// deletion_scheduled_at absent; token_hash must exist and be non-empty (legacy
// pre-ZLA subs have no token_hash and are excluded). See
// signup-reset-behaviour.md (reconciliation report).
func (r *SubscriptionRepository) FindDuplicateTokenHashGroups(ctx context.Context) ([]model.DuplicateTokenHashGroup, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"token_hash":            bson.M{"$exists": true, "$ne": ""},
			"deletion_scheduled_at": bson.M{"$exists": false},
		}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":         "$token_hash",
			"count":       bson.M{"$sum": 1},
			"account_ids": bson.M{"$push": "$account_id"},
		}}},
		bson.D{{Key: "$match", Value: bson.M{"count": bson.M{"$gt": 1}}}},
	}

	cursor, err := r.subscriptionsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groups []model.DuplicateTokenHashGroup
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, err
	}
	return groups, nil
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

// DeleteSubscriptionByAccountId removes the subscription document for an account.
// Returns nil if no subscription exists (some accounts never had one).
func (r *SubscriptionRepository) DeleteSubscriptionByAccountId(ctx context.Context, accountId string) error {
	objID, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}
	filter := bson.D{{Key: "account_id", Value: objID}}
	if _, err := r.subscriptionsCollection.DeleteOne(ctx, filter); err != nil {
		log.Error().Err(err).Msg("Failed to delete subscription for account")
		return err
	}
	return nil
}

// FindExpiredUnnotified is a coarse pre-filter: returns any sub with
// notified=false whose active_until elapsed >24h ago OR whose updated_at is
// >48h old (outage-triggered LA path). Callers MUST post-filter via
// sub.GetStatus() == StatusLimitedAccess — predicate logic lives only in
// model.Subscription.
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

// FindInactiveUnnotified is a coarse pre-filter: returns any sub with
// notified_inactive=false whose active_until or updated_at is older
// than 14 days, OR whose tier identifies the IVPN Standard plan (substring
// "Tier 1" or "Standard" — terminal inactive state). Callers MUST post-filter
// via sub.GetStatus() == StatusInactive. The tier regex mirrors
// model.hasStandardTier; if the model rule is extended, this filter may
// need to widen, but never to narrow.
func (r *SubscriptionRepository) FindInactiveUnnotified(ctx context.Context) ([]model.Subscription, error) {
	fourteenDaysAgo := time.Now().AddDate(0, 0, -14)
	filter := bson.M{
		"notified_inactive": false,
		"$or": []bson.M{
			{"active_until": bson.M{"$lt": fourteenDaysAgo}},
			{"updated_at": bson.M{"$lt": fourteenDaysAgo}},
			{"tier": bson.M{"$regex": "Tier 1|Standard"}},
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

// FindWithLANotified returns all subscriptions whose `notified` flag is true.
// Used by the LA cron's re-arm step: it iterates the result, calls
// sub.GetStatus(), and clears the flag for any sub no longer in LimitedAccess.
func (r *SubscriptionRepository) FindWithLANotified(ctx context.Context) ([]model.Subscription, error) {
	filter := bson.M{"notified": true}
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

// FindWithInactiveNotified returns all subscriptions whose
// `notified_inactive` flag is true. Used by the inactive cron's re-arm step:
// it iterates the result, calls sub.GetStatus(), and clears the flag for any
// sub no longer Inactive.
func (r *SubscriptionRepository) FindWithInactiveNotified(ctx context.Context) ([]model.Subscription, error) {
	filter := bson.M{"notified_inactive": true}
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

// SetNotified sets the `notified` field to `value` for the given subscription IDs.
func (r *SubscriptionRepository) SetNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error {
	if len(subscriptionIDs) == 0 {
		return nil
	}
	filter := bson.M{"_id": bson.M{"$in": subscriptionIDs}}
	update := bson.M{"$set": bson.M{"notified": value}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Bool("value", value).Msg("Failed to set notified flag")
	}
	return err
}

// SetInactiveNotified sets the `notified_inactive` field to `value`
// for the given subscription IDs.
func (r *SubscriptionRepository) SetInactiveNotified(ctx context.Context, subscriptionIDs []uuid.UUID, value bool) error {
	if len(subscriptionIDs) == 0 {
		return nil
	}
	filter := bson.M{"_id": bson.M{"$in": subscriptionIDs}}
	update := bson.M{"$set": bson.M{"notified_inactive": value}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Bool("value", value).Msg("Failed to set notified_inactive flag")
	}
	return err
}
