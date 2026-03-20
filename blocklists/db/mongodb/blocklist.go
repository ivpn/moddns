package mongodb

import (
	"context"
	"errors"

	"github.com/ivpn/dns/blocklists/model"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BlocklistRepository is a MongoDB repository for blocklists collection
type BlocklistRepository struct {
	DbName                       string
	MetadataCollectionName       string
	ContentCollectionName        string
	blocklistsMetadataCollection *mongo.Collection
	blocklistsCollection         *mongo.Collection
}

// NewBlocklistRepository creates a new BlocklistRepository instance
func NewBlocklistRepository(client *mongo.Client, dbName, metadataCollName, contentCollName string) BlocklistRepository {
	metadataCollection := client.Database(dbName).Collection(metadataCollName)
	blocklistsCollection := client.Database(dbName).Collection(contentCollName)

	return BlocklistRepository{
		DbName:                       dbName,
		MetadataCollectionName:       metadataCollName,
		ContentCollectionName:        contentCollName,
		blocklistsMetadataCollection: metadataCollection,
		blocklistsCollection:         blocklistsCollection,
	}
}

// Upsert creates or updates a blocklist metadata in the blocklists_metadata collection
func (r *BlocklistRepository) UpsertMetadata(ctx context.Context, blocklist model.BlocklistMetadata) error {
	filter := bson.M{"blocklist_id": blocklist.BlocklistID}
	update := bson.M{"$set": blocklist}
	opts := options.Update().SetUpsert(true)
	res, err := r.blocklistsMetadataCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	log.Debug().Str("component", "mongoDB").Interface("result", res).Msg("Upserted blocklist")
	return nil
}

// Upsert creates or updates a blocklist content in the blocklists collection
func (r *BlocklistRepository) UpsertContent(ctx context.Context, blocklist model.BlocklistContent) error {
	// filter := bson.M{"blocklist_id": blocklist.BlocklistID}
	filter := bson.M{"_id": blocklist.ID}
	update := bson.M{"$set": blocklist}
	opts := options.Update().SetUpsert(true)
	res, err := r.blocklistsCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	log.Debug().Str("component", "mongoDB").Interface("result", res).Msg("Upserted blocklist")
	return nil
}

// Get returns blocklists from the blocklists_metadata collection
func (r *BlocklistRepository) GetMetadata(ctx context.Context, filter map[string]any) ([]model.BlocklistMetadata, error) {
	filterBson := bson.D{}
	defaultVal, exists := filter["default"]
	if exists {
		isDefault, err := cast.ToBoolE(defaultVal)
		if err != nil {
			return nil, err
		}
		filterBson = bson.D{primitive.E{Key: "default", Value: isDefault}}
	}

	blocklistVal, exists := filter["blocklist_id"]
	if exists {
		filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: blocklistVal}}
	}

	cursor, err := r.blocklistsMetadataCollection.Find(ctx, filterBson)
	if err != nil {
		return nil, err
	}

	blocklists := make([]model.BlocklistMetadata, 0)
	if err = cursor.All(ctx, &blocklists); err != nil {
		return nil, err
	}

	return blocklists, nil
}

// Get returns blocklists from the blocklists_metadata collection
func (r *BlocklistRepository) GetContent(ctx context.Context, filter map[string]any) ([]model.BlocklistContent, error) {
	filterBson := bson.D{}
	defaultVal, exists := filter["default"]
	if exists {
		isDefault, err := cast.ToBoolE(defaultVal)
		if err != nil {
			return nil, err
		}
		filterBson = bson.D{primitive.E{Key: "default", Value: isDefault}}
	}

	blocklistVal, exists := filter["blocklist_id"]
	if exists {
		filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: blocklistVal}}
	}

	cursor, err := r.blocklistsCollection.Find(ctx, filterBson)
	if err != nil {
		return nil, err
	}

	contents := make([]model.BlocklistContent, 0)
	if err = cursor.All(ctx, &contents); err != nil {
		return nil, err
	}

	return contents, nil
}

// DeleteMetadata removes blocklists from the blocklists_metadata collection based on the provided filter
func (r *BlocklistRepository) DeleteMetadata(ctx context.Context, filter map[string]any) error {
	filterBson := bson.D{}

	blocklistVal, exists := filter["blocklist_id"]
	if exists {
		switch v := blocklistVal.(type) {
		case string:
			filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: v}}
		case []string:
			filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: bson.D{
				primitive.E{Key: "$nin", Value: v},
			}}}
		}
	}

	result, err := r.blocklistsMetadataCollection.DeleteMany(ctx, filterBson)
	if err != nil {
		return err
	}

	log.Debug().
		Str("component", "mongoDB").
		Interface("filter", filter).
		Int64("deleted_count", result.DeletedCount).
		Msg("Deleted blocklist metadata documents")

	return nil
}

// Delete removes blocklists from the blocklists collection based on the provided filter
func (r *BlocklistRepository) Delete(ctx context.Context, filter map[string]any) error {
	filterBson := bson.D{}

	// Handle default filter
	defaultVal, exists := filter["default"]
	if exists {
		isDefault, err := cast.ToBoolE(defaultVal)
		if err != nil {
			return err
		}
		filterBson = bson.D{primitive.E{Key: "default", Value: isDefault}}
	}

	// Handle blocklist_id filter
	blocklistVal, exists := filter["blocklist_id"]
	if exists {
		switch v := blocklistVal.(type) {
		case string:
			filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: v}}
		case map[string]any:
			// Handle regex case for blocklist_id
			if regex, ok := v["$regex"]; ok {
				filterBson = bson.D{primitive.E{Key: "blocklist_id", Value: bson.D{
					primitive.E{Key: "$regex", Value: regex},
				}}}
			}
		}
	}

	// Handle _id filter
	idVal, exists := filter["_id"]
	if exists {
		switch v := idVal.(type) {
		case primitive.ObjectID:
			filterBson = bson.D{primitive.E{Key: "_id", Value: v}}
		case []primitive.ObjectID:
			// Handle array of ObjectIDs
			filterBson = bson.D{primitive.E{Key: "_id", Value: bson.D{
				primitive.E{Key: "$in", Value: v},
			}}}
		default:
			return errors.New("unsupported type for _id filter")
		}
	}

	result, err := r.blocklistsCollection.DeleteMany(ctx, filterBson)
	if err != nil {
		return err
	}

	log.Debug().
		Str("component", "mongoDB").
		Interface("filter", filter).
		Int64("deleted_count", result.DeletedCount).
		Msg("Deleted blocklist documents")

	return nil
}
