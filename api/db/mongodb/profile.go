package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
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
	log.Ctx(ctx).Info().Msgf("Created new profile")
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

func (r *ProfileRepository) GetProfilesStatisticsSettings(ctx context.Context, profileIds []string) (map[string]*model.StatisticsSettings, error) {
	if len(profileIds) == 0 {
		return map[string]*model.StatisticsSettings{}, nil
	}

	// Primary read: a lagging secondary must not make a fresh profile look deleted.
	coll, err := r.profilesCollection.Clone(options.Collection().SetReadPreference(readpref.Primary()))
	if err != nil {
		return nil, err
	}

	filter := bson.D{primitive.E{Key: "profile_id", Value: bson.D{primitive.E{Key: "$in", Value: profileIds}}}}
	projection := bson.D{
		primitive.E{Key: "profile_id", Value: 1},
		primitive.E{Key: "settings.statistics", Value: 1},
	}
	cursor, err := coll.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []struct {
		ProfileId string `bson:"profile_id"`
		Settings  *struct {
			Statistics *model.StatisticsSettings `bson:"statistics"`
		} `bson:"settings"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	out := make(map[string]*model.StatisticsSettings, len(docs))
	for _, d := range docs {
		var st *model.StatisticsSettings
		if d.Settings != nil {
			st = d.Settings.Statistics
		}
		out[d.ProfileId] = st
	}
	return out, nil
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
	log.Ctx(ctx).Info().Int64("count", res.DeletedCount).Msgf("Deleted profile")
	return nil
}

// UpdateFields applies upd as one pipeline update, so enabled_at and the custom-rule
// append are evaluated against the stored document they replace.
func (r *ProfileRepository) UpdateFields(ctx context.Context, profileId string, upd repository.ProfileFieldsUpdate) (*model.Profile, *model.Profile, error) {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}

	set := bson.D{}
	for _, f := range upd.Set {
		set = append(set, primitive.E{Key: f.Field, Value: bson.D{primitive.E{Key: "$literal", Value: f.Value}}})
		if f.Field == statisticsEnabledField {
			set = append(set, primitive.E{Key: statisticsEnabledAtField, Value: enabledAtExpr(f.Value, upd.EnabledAtNow)})
		}
	}
	pipeline := mongo.Pipeline{}
	if len(set) > 0 {
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$set", Value: set}})
	}
	// One stage per rule: fields within a single $set stage all read the input document.
	for _, rule := range upd.AppendCustomRules {
		pipeline = append(pipeline, appendCustomRuleIfAbsentStage(rule))
	}
	if len(pipeline) == 0 {
		return nil, nil, fmt.Errorf("profile update has no fields")
	}

	var before model.Profile
	opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)
	if err := r.profilesCollection.FindOneAndUpdate(ctx, filterBson, pipeline, opts).Decode(&before); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, errors.ErrProfileNotFound
		}
		return nil, nil, err
	}

	coll, err := r.profilesCollection.Clone(options.Collection().SetReadPreference(readpref.Primary()))
	if err != nil {
		return nil, nil, err
	}
	var after model.Profile
	if err := coll.FindOne(ctx, filterBson).Decode(&after); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, errors.ErrProfileNotFound
		}
		return nil, nil, err
	}
	log.Ctx(ctx).Debug().Int("fields", len(upd.Set)).Msg("Updated profile fields")
	return &before, &after, nil
}

const (
	statisticsEnabledField   = "settings.statistics.enabled"
	statisticsEnabledAtField = "settings.statistics.enabled_at"
)

// enabledAtExpr: set on false->true, removed on true->false, otherwise kept (api-endpoint-behaviour.md G7).
func enabledAtExpr(enabled any, now time.Time) bson.D {
	wasEnabled := bson.D{primitive.E{Key: "$eq", Value: bson.A{"$" + statisticsEnabledField, true}}}
	keep := "$" + statisticsEnabledAtField
	on, off := bson.A{wasEnabled, keep, now}, bson.A{wasEnabled, "$$REMOVE", keep}
	branch := off
	if b, _ := enabled.(bool); b {
		branch = on
	}
	return bson.D{primitive.E{Key: "$cond", Value: branch}}
}

func appendCustomRuleIfAbsentStage(rule *model.CustomRule) bson.D {
	rules := bson.D{primitive.E{Key: "$ifNull", Value: bson.A{"$settings.custom_rules", bson.A{}}}}
	values := bson.D{primitive.E{Key: "$ifNull", Value: bson.A{"$settings.custom_rules.value", bson.A{}}}}
	present := bson.D{primitive.E{Key: "$in", Value: bson.A{bson.D{primitive.E{Key: "$literal", Value: rule.Value}}, values}}}
	appended := bson.D{primitive.E{Key: "$concatArrays", Value: bson.A{rules, bson.A{bson.D{primitive.E{Key: "$literal", Value: rule}}}}}}
	return bson.D{primitive.E{Key: "$set", Value: bson.D{primitive.E{Key: "settings.custom_rules",
		Value: bson.D{primitive.E{Key: "$cond", Value: bson.A{present, rules, appended}}}}}}}
}

func (r *ProfileRepository) UpdateSettings(ctx context.Context, profileId string, settings *model.ProfileSettings) error {
	filterBson := bson.D{primitive.E{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{primitive.E{Key: "$set", Value: bson.D{primitive.E{Key: "settings", Value: settings}}}}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Int64("count", res.MatchedCount).Msgf("Updated profile settings")

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
	log.Ctx(ctx).Info().Int64("count", res.MatchedCount).Msgf("Removed custom rules")

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

	_, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	return nil
}

// UpdateCustomRule updates a single custom rule in place, matched by its ObjectID,
// preserving the rule's position in the settings.custom_rules array.
func (r *ProfileRepository) UpdateCustomRule(ctx context.Context, profileId string, rule *model.CustomRule) error {
	filterBson := bson.D{
		{Key: "profile_id", Value: profileId},
		{Key: "settings.custom_rules._id", Value: rule.ID},
	}
	updateBson := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "settings.custom_rules.$.action", Value: rule.Action},
			{Key: "settings.custom_rules.$.value", Value: rule.Value},
			{Key: "settings.custom_rules.$.syntax", Value: rule.Syntax},
			{Key: "settings.custom_rules.$.note", Value: rule.Note},
			{Key: "settings.custom_rules.$.group", Value: rule.Group},
			{Key: "settings.custom_rules.$.order", Value: rule.Order},
		}},
	}

	res, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.ErrCustomRuleNotFound
	}
	return nil
}

// UpdateCustomRulesOrder sets the display `order` of each rule identified by its
// hex ObjectID, in a single atomic update using arrayFilters. Rules not present
// in idToOrder keep their stored order.
func (r *ProfileRepository) UpdateCustomRulesOrder(ctx context.Context, profileId string, idToOrder map[string]int) error {
	if len(idToOrder) == 0 {
		return nil
	}

	setDoc := bson.D{}
	arrayFilters := make([]any, 0, len(idToOrder))
	i := 0
	for id, order := range idToOrder {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		identifier := fmt.Sprintf("r%d", i)
		setDoc = append(setDoc, bson.E{
			Key:   fmt.Sprintf("settings.custom_rules.$[%s].order", identifier),
			Value: order,
		})
		arrayFilters = append(arrayFilters, bson.M{identifier + "._id": objectID})
		i++
	}

	filterBson := bson.D{{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{{Key: "$set", Value: setDoc}}
	opts := options.Update().SetArrayFilters(options.ArrayFilters{Filters: arrayFilters})

	_, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson, opts)
	return err
}

// SetCustomRuleGroups replaces the profile's per-list group registry wholesale.
// The registry is metadata only and is never synced to the proxy.
func (r *ProfileRepository) SetCustomRuleGroups(ctx context.Context, profileId string, groups model.CustomRuleGroups) error {
	filterBson := bson.D{{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "settings.custom_rule_groups", Value: groups},
		}},
	}

	_, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson)
	return err
}

// ReassignCustomRuleGroup sets group=to on every rule with the given action whose
// group==from, in a single atomic update using an arrayFilter. Scoping by action
// keeps denylist and allowlist groups independent. Passing to="" moves the rules
// to Ungrouped (used by group deletion). Group labels are metadata only.
func (r *ProfileRepository) ReassignCustomRuleGroup(ctx context.Context, profileId, action, from, to string) error {
	filterBson := bson.D{{Key: "profile_id", Value: profileId}}
	updateBson := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "settings.custom_rules.$[elem].group", Value: to},
		}},
	}
	opts := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []any{bson.M{"elem.group": from, "elem.action": action}},
	})

	_, err := r.profilesCollection.UpdateOne(ctx, filterBson, updateBson, opts)
	return err
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
	log.Ctx(ctx).Info().
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
	log.Ctx(ctx).Info().
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
	log.Ctx(ctx).Info().Int64("count", res.ModifiedCount).Msg("Enabled services for profile")
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
	log.Ctx(ctx).Info().Int64("count", res.ModifiedCount).Msg("Disabled services for profile")
	return nil
}
