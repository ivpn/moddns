package mongodb

import (
	"context"

	"github.com/ivpn/dns/api/model"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BlocklistRepository is a MongoDB repository for blocklists collection
type BlocklistRepository struct {
	DbName                 string
	CollectionName         string
	blocklistsMetadataColl *mongo.Collection
}

// NewBlocklistRepository creates a new BlocklistRepository instance
func NewBlocklistRepository(client *mongo.Client, dbName, collectionName string) BlocklistRepository {
	collection := client.Database(dbName).Collection(collectionName)

	return BlocklistRepository{
		DbName:                 dbName,
		CollectionName:         collectionName,
		blocklistsMetadataColl: collection,
	}
}

// Get returns blocklists from the blocklists collection
func (r *BlocklistRepository) Get(ctx context.Context, filter map[string]any, sortBy string) ([]*model.Blocklist, error) {
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

	sortSpec := buildBlocklistSortSpec(sortBy)
	findOptions := options.Find().SetSort(sortSpec)

	cursor, err := r.blocklistsMetadataColl.Find(ctx, filterBson, findOptions)
	if err != nil {
		return nil, err
	}

	blocklists := make([]*model.Blocklist, 0)
	if err = cursor.All(ctx, &blocklists); err != nil {
		return nil, err
	}

	return blocklists, nil
}

func buildBlocklistSortSpec(sortBy string) bson.D {
	switch sortBy {
	case "name":
		return bson.D{{Key: "name", Value: 1}, {Key: "last_modified", Value: -1}}
	case "entries":
		return bson.D{{Key: "entries", Value: -1}, {Key: "last_modified", Value: -1}}
	default:
		return bson.D{{Key: "last_modified", Value: -1}}
	}
}
