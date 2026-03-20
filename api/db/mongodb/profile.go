package mongodb

import (
	"context"

	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ProfileRepository is a MongoDB repository for profiles collection
type ProfileRepository struct {
	DbName             string
	CollectionName     string
	profilesCollection *mongo.Collection
}

// NewProfileRepository creates a new ProfileRepository instance
func NewProfileRepository(client *mongo.Client, dbName, collectionName string) ProfileRepository {
	collection := client.Database(dbName).Collection(collectionName)

	return ProfileRepository{
		DbName:             dbName,
		CollectionName:     collectionName,
		profilesCollection: collection,
	}
}

// Create adds a new profile to the profiles collection
func (r *ProfileRepository) CreateProfile(ctx context.Context, profile *model.Profile) error {
	_, err := r.profilesCollection.InsertOne(ctx, profile)
	if err != nil {
		return err
	}
	log.Info().Msgf("Created new profile")
	return nil
}

func (r *ProfileRepository) GetProfileById(ctx context.Context, profileId string) (*model.Profile, error) {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}

	var profile model.Profile
	if err := r.profilesCollection.FindOne(ctx, filterBson).Decode(&profile); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProfileNotFound
		}
		return nil, err
	}
	return &profile, nil
}

func (r *ProfileRepository) GetProfilesByAccountId(ctx context.Context, accountId string) ([]model.Profile, error) {
	filterBson := bson.D{primitive.E{Key: "account_id", Value: accountId}}
	cursor, err := r.profilesCollection.Find(ctx, filterBson)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var profiles = make([]model.Profile, 0)
	if err := cursor.All(ctx, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func (r *ProfileRepository) DeleteProfileById(ctx context.Context, profileId string) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	res, err := r.profilesCollection.DeleteOne(ctx, filterBson)
	if err != nil {
		return err
	}
	log.Info().Int64("count", res.DeletedCount).Msgf("Deleted profile")
	return nil
}

func (r *ProfileRepository) Update(ctx context.Context, profileId string, profile *model.Profile) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	res, err := r.profilesCollection.ReplaceOne(ctx, filterBson, profile)
	if err != nil {
		return err
	}
	log.Debug().Int64("count", res.MatchedCount).Msgf("Updated profile")

	return nil
}

func (r *ProfileRepository) UpdateSettings(ctx context.Context, profileId string, settings *model.ProfileSettings) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{primitive.E{Key: "$set", Value: bson.D{primitive.E{Key: "settings", Value: settings}}}}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().Int64("count", res.MatchedCount).Msgf("Updated profile settings")

	return nil
}

// RemoveCustomRules removes the custom rules with the given IDs from the profile's settings.custom_rules array atomically.
func (r *ProfileRepository) RemoveCustomRules(ctx context.Context, profileId string, ruleIds []string) error {
	var objectIDs []primitive.ObjectID
	for _, id := range ruleIds {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objectIDs = append(objectIDs, objectID)
	}
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "settings.custom_rules", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "$in", Value: objectIDs}}},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, update)
	if err != nil {
		return err
	}
	log.Info().Int64("count", res.MatchedCount).Msgf("Removed custom rules")

	return nil
}

// CreateCustomRules adds the given custom rules to the profile's settings.custom_rules array atomically.
func (r *ProfileRepository) CreateCustomRules(ctx context.Context, profileId string, rules []*model.CustomRule) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$push", Value: bson.D{
			{Key: "settings.custom_rules", Value: bson.D{
				{Key: "$each", Value: rules},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().
		Int64("count", res.ModifiedCount).
		Msgf("Added custom rules to profile")
	return nil
}

// EnableBlocklists adds the given blocklist IDs to the profile's enabled blocklists array atomically.
func (r *ProfileRepository) EnableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$addToSet", Value: bson.D{
			{Key: "settings.privacy.blocklists", Value: bson.D{
				{Key: "$each", Value: blocklistIds},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().
		Int64("count", res.ModifiedCount).
		Msgf("Enabled blocklists for profile")
	return nil
}

// DisableBlocklists removes the given blocklist IDs from the profile's enabled blocklists array atomically.
func (r *ProfileRepository) DisableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "settings.privacy.blocklists", Value: bson.D{
				{Key: "$in", Value: blocklistIds},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().
		Int64("count", res.ModifiedCount).
		Msgf("Disabled blocklists for profile")
	return nil
}

// EnableServices adds the given service IDs to the profile's blocked services array atomically.
func (r *ProfileRepository) EnableServices(ctx context.Context, profileId string, serviceIds []string) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$addToSet", Value: bson.D{
			{Key: "settings.privacy.services", Value: bson.D{
				{Key: "$each", Value: serviceIds},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().Int64("count", res.ModifiedCount).Msg("Enabled services for profile")
	return nil
}

// DisableServices removes the given service IDs from the profile's blocked services array atomically.
func (r *ProfileRepository) DisableServices(ctx context.Context, profileId string, serviceIds []string) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "settings.privacy.services", Value: bson.D{
				{Key: "$in", Value: serviceIds},
			}},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Info().Int64("count", res.ModifiedCount).Msg("Disabled services for profile")
	return nil
}
