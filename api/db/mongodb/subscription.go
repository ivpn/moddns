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

// ResetNotifiedForActive sets notified=false for all subscriptions where active_until >= now.
func (r *SubscriptionRepository) ResetNotifiedForActive(ctx context.Context) error {
	filter := bson.M{"active_until": bson.M{"$gte": time.Now()}}
	update := bson.M{"$set": bson.M{"notified": false}}
	_, err := r.subscriptionsCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reset notified flag for active subscriptions")
	}
	return err
}

// FindExpiredUnnotified returns subscriptions where notified=false and active_until < now - 24h.
func (r *SubscriptionRepository) FindExpiredUnnotified(ctx context.Context) ([]model.Subscription, error) {
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	filter := bson.M{
		"notified":     false,
		"active_until": bson.M{"$lt": oneDayAgo},
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
