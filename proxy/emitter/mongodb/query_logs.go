package mongodb

import (
	"context"
	"fmt"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	queryLogsCollOneHour        = fmt.Sprintf("%s_1h", collNameQueryLogs)
	queryLogsCollSixHours       = fmt.Sprintf("%s_6h", collNameQueryLogs)
	queryLogsCollOneDay         = fmt.Sprintf("%s_1d", collNameQueryLogs)
	queryLogsCollOneWeek        = fmt.Sprintf("%s_1w", collNameQueryLogs)
	queryLogsCollOneMonth       = fmt.Sprintf("%s_1m", collNameQueryLogs)
	expirationOneHour     int64 = 3600
	expirationSixHours    int64 = 21600
	expirationOneDay      int64 = 86400
	expirationOneWeek     int64 = 604800
	expirationOneMonth    int64 = 2592000
	metafieldProfileId          = "profile_id"
	timeField                   = "timestamp"
	granularitySeconds          = "seconds"
	granularityMinutes          = "minutes"
	queryLogsCollections        = map[string]int64{
		queryLogsCollOneHour:  expirationOneHour,
		queryLogsCollSixHours: expirationSixHours,
		queryLogsCollOneDay:   expirationOneDay,
		queryLogsCollOneWeek:  expirationOneWeek,
		queryLogsCollOneMonth: expirationOneMonth,
	}
)

// QueryLogsRepository is a MongoDB repository for query_logs collection
type QueryLogsRepository struct {
	client                *mongo.Client
	database              *mongo.Database
	DbName                string
	CollectionName        string
	queryLogsCollOneHour  *mongo.Collection
	queryLogsCollSixHours *mongo.Collection
	queryLogsCollOneDay   *mongo.Collection
	queryLogsCollOneWeek  *mongo.Collection
	queryLogsCollOneMonth *mongo.Collection
}

// NewQueryLogsRepository creates a new query logs instance
func NewQueryLogsRepository(client *mongo.Client, dbName, collectionName string) (*QueryLogsRepository, error) {
	database := client.Database(dbName)
	repo := &QueryLogsRepository{
		client:         client,
		database:       database,
		DbName:         dbName,
		CollectionName: collectionName,
	}

	ctx := context.Background()
	// TODO: create migrations
	if err := repo.createTimeSeriesCollections(ctx); err != nil {
		return nil, err
	}

	return repo, nil
}

// InsertBatch upserts a batch of user query logs
func (r *QueryLogsRepository) InsertBatch(ctx context.Context, batch []model.EventQueryLog) error {
	oneHourdocs := make([]interface{}, 0)
	sixHoursdocs := make([]interface{}, 0)
	oneDaydocs := make([]interface{}, 0)
	oneWeekdocs := make([]interface{}, 0)
	oneMonthdocs := make([]interface{}, 0)

	for _, queryLogEvent := range batch {
		queryLogEvent.QueryLog.ID = primitive.NewObjectID()
		switch queryLogEvent.Metadata.Retention {
		case model.RetentionOneHour:
			oneHourdocs = append(oneHourdocs, queryLogEvent.QueryLog)
		case model.RetentionSixHours:
			sixHoursdocs = append(sixHoursdocs, queryLogEvent.QueryLog)
		case model.RetentionOneDay:
			oneDaydocs = append(oneDaydocs, queryLogEvent.QueryLog)
		case model.RetentionOneWeek:
			oneWeekdocs = append(oneWeekdocs, queryLogEvent.QueryLog)
		case model.RetentionOneMonth:
			oneMonthdocs = append(oneMonthdocs, queryLogEvent.QueryLog)
		default:
			// Entry content is deliberately not logged.
			log.Warn().Str("retention", string(queryLogEvent.Metadata.Retention)).Msg("Dropping query log entry with unknown retention")
		}
	}

	queryLogsMap := map[*mongo.Collection][]interface{}{
		r.queryLogsCollOneHour:  oneHourdocs,
		r.queryLogsCollSixHours: sixHoursdocs,
		r.queryLogsCollOneDay:   oneDaydocs,
		r.queryLogsCollOneWeek:  oneWeekdocs,
		r.queryLogsCollOneMonth: oneMonthdocs,
	}

	for coll, docs := range queryLogsMap {
		if len(docs) > 0 {
			_, err := coll.InsertMany(ctx, docs, &options.InsertManyOptions{
				Ordered: new(bool),
			})
			if err != nil {
				return err
			}
			log.Info().Str("collection_name", coll.Name()).Int("batch_size", len(docs)).Msgf("Inserted batch of user query logs")
		}
	}

	return nil
}

func (r *QueryLogsRepository) createTimeSeriesCollections(ctx context.Context) error {
	existingCollNames, err := r.database.ListCollectionNames(ctx, bson.D{}, nil)
	if err != nil {
		log.Error().Err(err).Msg("Error listing collection names")
		return err
	}
	defer func() {
		if err == nil {
			r.queryLogsCollOneHour = r.client.Database(r.DbName).Collection(queryLogsCollOneHour)
			r.queryLogsCollSixHours = r.client.Database(r.DbName).Collection(queryLogsCollSixHours)
			r.queryLogsCollOneDay = r.client.Database(r.DbName).Collection(queryLogsCollOneDay)
			r.queryLogsCollOneWeek = r.client.Database(r.DbName).Collection(queryLogsCollOneWeek)
			r.queryLogsCollOneMonth = r.client.Database(r.DbName).Collection(queryLogsCollOneMonth)
		}
	}()

	// Timeseries collections must be explicitly created
	for collName, expiration := range queryLogsCollections {
		collExists := contains(existingCollNames, collName)
		if collExists {
			log.Info().Msgf("%s collection already exists. continuing.", collName)
			continue
		}
		err = r.database.CreateCollection(
			ctx,
			collName,
			&options.CreateCollectionOptions{
				TimeSeriesOptions: &options.TimeSeriesOptions{
					TimeField:   timeField,
					MetaField:   &metafieldProfileId,
					Granularity: &granularitySeconds,
				},
				ExpireAfterSeconds: &expiration,
			},
		)
		if err != nil {
			log.Error().Err(err).Msgf("Error creating collection [%s]", collName)
			return err
		} else {
			log.Info().Msgf("Successfully created %s collection for the first time.", collName)
		}
	}
	return nil
}

func contains(existingCollections []string, collName string) bool {
	for _, name := range existingCollections {
		if name == collName {
			return true
		}
	}
	return false
}
