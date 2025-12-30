package mongodb

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const slowQueryThreshold = 300 * time.Millisecond

var (
	queryLogsCollOneHour  = fmt.Sprintf("%s_1h", collNameQueryLogs)
	queryLogsCollSixHours = fmt.Sprintf("%s_6h", collNameQueryLogs)
	queryLogsCollOneDay   = fmt.Sprintf("%s_1d", collNameQueryLogs)
	queryLogsCollOneWeek  = fmt.Sprintf("%s_1w", collNameQueryLogs)
	queryLogsCollOneMonth = fmt.Sprintf("%s_1m", collNameQueryLogs)
)

// QueryLogsRepository is a MongoDB repository for query_logs timeseries collections
type QueryLogsRepository struct {
	DbName                string
	CollectionName        string
	queryLogsCollOneHour  *mongo.Collection
	queryLogsCollSixHours *mongo.Collection
	queryLogsCollOneDay   *mongo.Collection
	queryLogsCollOneWeek  *mongo.Collection
	queryLogsCollOneMonth *mongo.Collection
	queryLogsCollections  map[model.Retention]*mongo.Collection
}

// NewQueryLogsRepository creates a new QueryLogsRepository instance
func NewQueryLogsRepository(client *mongo.Client, dbName, collectionName string) QueryLogsRepository {
	repo := QueryLogsRepository{
		DbName:         dbName,
		CollectionName: collectionName,
	}

	repo.queryLogsCollOneHour = client.Database(repo.DbName).Collection(queryLogsCollOneHour)
	repo.queryLogsCollSixHours = client.Database(repo.DbName).Collection(queryLogsCollSixHours)
	repo.queryLogsCollOneDay = client.Database(repo.DbName).Collection(queryLogsCollOneDay)
	repo.queryLogsCollOneWeek = client.Database(repo.DbName).Collection(queryLogsCollOneWeek)
	repo.queryLogsCollOneMonth = client.Database(repo.DbName).Collection(queryLogsCollOneMonth)

	repo.queryLogsCollections = map[model.Retention]*mongo.Collection{
		model.RetentionOneHour:  repo.queryLogsCollOneHour,
		model.RetentionSixHours: repo.queryLogsCollSixHours,
		model.RetentionOneDay:   repo.queryLogsCollOneDay,
		model.RetentionOneWeek:  repo.queryLogsCollOneWeek,
		model.RetentionOneMonth: repo.queryLogsCollOneMonth,
	}

	return repo
}

// GetQueryLogs returns query logs for a profile with optional pagination.
// Pagination applies only when pageSize > 0. Page is 1-based when pagination is active.
// When pageSize <= 0, all matching logs are returned (no skip/limit applied).
func (r *QueryLogsRepository) GetQueryLogs(ctx context.Context, profileId string, retention model.Retention, status string, timespan int, deviceId, search, sortBy string, page, pageSize int) ([]model.QueryLog, error) {
	start := time.Now()
	coll := r.getCollObject(retention)

	matchFilter := bson.D{
		primitive.E{Key: "profile_id", Value: profileId},
	}
	if timespan != 0 {
		matchFilter = append(matchFilter, bson.E{
			Key: "timestamp",
			Value: bson.D{
				primitive.E{Key: "$gte", Value: time.Now().Add(time.Duration(-timespan) * time.Hour)},
			},
		})
	}
	if status != "all" {
		matchFilter = append(matchFilter, bson.E{
			Key:   "status",
			Value: status,
		})
	}
	if deviceId != "" {
		matchFilter = append(matchFilter, bson.E{
			Key:   "device_id",
			Value: deviceId,
		})
	}
	if search != "" {
		// build case-insensitive substring regex; escape user input to avoid regex injection
		escaped := regexp.QuoteMeta(search)
		matchFilter = append(matchFilter, bson.E{
			Key: "dns_request.domain",
			Value: bson.D{
				{Key: "$regex", Value: escaped},
				{Key: "$options", Value: "i"},
			},
		})
	}

	matchStage := bson.D{
		primitive.E{Key: "$match", Value: matchFilter},
	}

	sortSpec := buildSortSpec(sortBy)
	sortStage := bson.D{
		primitive.E{Key: "$sort", Value: sortSpec},
	}

	pipeline := mongo.Pipeline{matchStage, sortStage}
	if pageSize > 0 { // apply pagination only when pageSize is positive
		if page <= 0 { // normalize page
			page = 1
		}
		skipStage := bson.D{primitive.E{Key: "$skip", Value: (page - 1) * pageSize}}
		limitStage := bson.D{primitive.E{Key: "$limit", Value: pageSize}}
		pipeline = append(pipeline, skipStage, limitStage)
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}

	results := make([]model.QueryLog, 0)
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	duration := time.Since(start)
	if duration > slowQueryThreshold {
		log.Warn().
			Bool("slow", true).
			Str("retention", string(retention)).
			Str("status", status).
			Str("sort", sortBy).
			Str("search", search).
			Int("page", page).
			Int("page_size", pageSize).
			Int("result_count", len(results)).
			Dur("duration", duration).
			Msg("Query logs fetch took too long")
	}

	return results, nil
}

func buildSortSpec(sortBy string) bson.D {
	switch sortBy {
	case "domain":
		return bson.D{
			{Key: "dns_request.domain", Value: 1},
			{Key: "timestamp", Value: -1},
		}
	case "client_ip":
		return bson.D{
			{Key: "client_ip", Value: 1},
			{Key: "timestamp", Value: -1},
		}
	default:
		return bson.D{{Key: "timestamp", Value: -1}}
	}
}

// DeleteQueryLogs deletes query logs for the given profile ID
func (r *QueryLogsRepository) DeleteQueryLogs(ctx context.Context, profileId string) error {
	// delete query logs from all available collections (in case user has reconfigured retention)
	for retention, coll := range r.queryLogsCollections {
		res, err := coll.DeleteMany(ctx, bson.D{
			primitive.E{Key: "profile_id", Value: profileId},
		})
		log.Debug().Str("collection_name", string(retention)).Int64("count", res.DeletedCount).Msg("Deleted query logs")
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *QueryLogsRepository) getCollObject(retention model.Retention) *mongo.Collection {
	switch retention {
	case model.RetentionOneHour:
		return r.queryLogsCollOneHour
	case model.RetentionSixHours:
		return r.queryLogsCollSixHours
	case model.RetentionOneDay:
		return r.queryLogsCollOneDay
	case model.RetentionOneWeek:
		return r.queryLogsCollOneWeek
	case model.RetentionOneMonth:
		return r.queryLogsCollOneMonth
	default:
		return r.queryLogsCollOneHour
	}
}
