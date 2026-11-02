package mongodb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/filterreasons"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	from := readFrom(start, retention, timespan)

	matchFilter := bson.D{
		primitive.E{Key: "profile_id", Value: profileId},
		primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$gte", Value: from}}},
	}
	switch status {
	case model.QueryLogStatusAll:
	case model.QueryLogStatusUnanswered:
		matchFilter = append(matchFilter, unansweredFilter())
	default:
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

	// Each collection returns at most the rows up to the requested page, already in order; the
	// merged sort then pages over them.
	branch := mongo.Pipeline{matchStage, sortStage}
	tail := mongo.Pipeline{sortStage}
	if pageSize > 0 { // apply pagination only when pageSize is positive
		if page <= 0 { // normalize page
			page = 1
		}
		branch = append(branch, bson.D{primitive.E{Key: "$limit", Value: page * pageSize}})
		skipStage := bson.D{primitive.E{Key: "$skip", Value: (page - 1) * pageSize}}
		limitStage := bson.D{primitive.E{Key: "$limit", Value: pageSize}}
		tail = append(tail, skipStage, limitStage)
	}

	results := make([]model.QueryLog, 0)
	cursor, err := r.aggregateUnion(ctx, profileId, from, branch, nil, tail)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}
	if cursor != nil {
		if err = cursor.All(ctx, &results); err != nil {
			return nil, err
		}
	}
	duration := time.Since(start)
	if duration > slowQueryThreshold {
		log.Ctx(ctx).Warn().
			Bool("slow", true).
			Str("retention", string(retention)).
			Str("status", status).
			Str("sort", sortBy).
			Int("search_len", len(search)).
			Int("page", page).
			Int("page_size", pageSize).
			Int("result_count", len(results)).
			Dur("duration", duration).
			Msg("Query logs fetch took too long")
	}

	return results, nil
}

// GetQueryLogDevices returns the distinct non-empty device IDs present in the
// profile's query logs within the retention, each with its most recent
// timestamp, sorted by device ID.
// Aggregation, not Collection.Distinct(): the distinct command is unsupported
// on time-series collections.
func (r *QueryLogsRepository) GetQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error) {
	start := time.Now()
	from := readFrom(start, retention, 0)

	branch := mongo.Pipeline{
		bson.D{primitive.E{Key: "$match", Value: bson.D{
			primitive.E{Key: "profile_id", Value: profileId},
			primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$gte", Value: from}}},
			// $nin (not $ne "") also excludes docs where the field is missing.
			primitive.E{Key: "device_id", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
		}}},
		bson.D{primitive.E{Key: "$group", Value: bson.D{
			primitive.E{Key: "_id", Value: "$device_id"},
			primitive.E{Key: "last_seen", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
		}}},
	}
	merge := mongo.Pipeline{
		bson.D{primitive.E{Key: "$group", Value: bson.D{
			primitive.E{Key: "_id", Value: "$_id"},
			primitive.E{Key: "last_seen", Value: bson.D{{Key: "$max", Value: "$last_seen"}}},
		}}},
	}
	tail := mongo.Pipeline{
		bson.D{primitive.E{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		// Safety valve against pathological device counts; ids are ≤36 chars.
		bson.D{primitive.E{Key: "$limit", Value: 500}},
	}

	results := make([]model.QueryLogDevice, 0)
	cursor, err := r.aggregateUnion(ctx, profileId, from, branch, merge, tail)
	if err != nil {
		return nil, err
	}
	if cursor != nil {
		if err = cursor.All(ctx, &results); err != nil {
			return nil, err
		}
	}
	duration := time.Since(start)
	if duration > slowQueryThreshold {
		// Counts/durations only — device IDs are sensitive log keys.
		log.Ctx(ctx).Warn().
			Bool("slow", true).
			Str("retention", string(retention)).
			Int("result_count", len(results)).
			Dur("duration", duration).
			Msg("Query log devices fetch took too long")
	}

	return results, nil
}

// GetQueryLogTopDomains returns the profile's most frequent domains with the
// given status inside the timespan window, count desc then domain asc. The
// $group must unpack every bucket in the window (dns_request.domain is a
// measurement field, so no index can serve it).
func (r *QueryLogsRepository) GetQueryLogTopDomains(ctx context.Context, profileId string, retention model.Retention, status string, timespanHours, limit int) ([]model.QueryLogTopDomain, error) {
	start := time.Now()
	from := readFrom(start, retention, timespanHours)
	match := bson.D{
		{Key: "profile_id", Value: profileId},
		{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: from}}},
		{Key: "status", Value: status},
		{Key: "dns_request.domain", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
	}
	results := make([]model.QueryLogTopDomain, 0)
	if err := r.topGroup(ctx, profileId, from, match, "$dns_request.domain", limit, &results); err != nil {
		return nil, err
	}
	r.warnIfSlow(ctx, "Query log top domains fetch took too long", retention, len(results), start)
	return results, nil
}

// GetQueryLogTopClients returns the profile's most frequent client IPs inside
// the timespan window, count desc then ip asc. Same cost profile as
// GetQueryLogTopDomains.
func (r *QueryLogsRepository) GetQueryLogTopClients(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopClient, error) {
	start := time.Now()
	from := readFrom(start, retention, timespanHours)
	match := bson.D{
		{Key: "profile_id", Value: profileId},
		{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: from}}},
		{Key: "client_ip", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
	}
	results := make([]model.QueryLogTopClient, 0)
	if err := r.topGroup(ctx, profileId, from, match, "$client_ip", limit, &results); err != nil {
		return nil, err
	}
	r.warnIfSlow(ctx, "Query log top clients fetch took too long", retention, len(results), start)
	return results, nil
}

// GetQueryLogCoveredFrom returns the timestamp of the profile's oldest row in
// the timespan window, truncated to the minute, or nil when the window is empty.
func (r *QueryLogsRepository) GetQueryLogCoveredFrom(ctx context.Context, profileId string, retention model.Retention, timespanHours int) (*time.Time, error) {
	from := readFrom(time.Now(), retention, timespanHours)
	oldest := bson.D{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: 1}}}}
	first := bson.D{{Key: "$limit", Value: 1}}
	branch := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "profile_id", Value: profileId},
			{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: from}}},
		}}},
		oldest,
		first,
		bson.D{{Key: "$project", Value: bson.D{{Key: "timestamp", Value: 1}}}},
	}
	cursor, err := r.aggregateUnion(ctx, profileId, from, branch, nil, mongo.Pipeline{oldest, first})
	if err != nil || cursor == nil {
		return nil, err
	}
	var rows []struct {
		Timestamp time.Time `bson:"timestamp"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	covered := rows[0].Timestamp.UTC().Truncate(time.Minute)
	return &covered, nil
}

// queryLogTopAggregateOptions bounds a top-list aggregation on the server: it unpacks every bucket
// in the window, and the request context is not cancelled when the client goes away.
func queryLogTopAggregateOptions() *options.AggregateOptions {
	return options.Aggregate().SetMaxTime(15 * time.Second)
}

// topBlocklistsBranch counts the profile's blocked rows since from in one collection once
// per "blocklist: <id>" reason token.
func topBlocklistsBranch(profileId string, from time.Time) mongo.Pipeline {
	prefix := bson.D{{Key: "$regex", Value: "^" + regexp.QuoteMeta(filterreasons.BlocklistPrefix)}}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "profile_id", Value: profileId},
			{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: from}}},
			{Key: "status", Value: model.QueryLogStatusBlocked},
		}}},
		bson.D{{Key: "$unwind", Value: "$reasons"}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "reasons", Value: prefix}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$reasons"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
}

// topCountMerge adds up the per-collection counts of a top list.
var topCountMerge = mongo.Pipeline{
	bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: "$_id"},
		{Key: "count", Value: bson.D{{Key: "$sum", Value: "$count"}}},
	}}},
}

// topRankTail ranks a top list count desc then key asc. Sorting a blocklist token equals
// sorting its id: every token shares the prefix.
func topRankTail(limit int) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
}

// GetQueryLogTopBlocklists returns the blocklists that blocked the profile's
// queries most often inside the timespan window. A row matched by several lists
// counts once for each. Same cost profile as GetQueryLogTopDomains.
func (r *QueryLogsRepository) GetQueryLogTopBlocklists(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopBlocklist, error) {
	start := time.Now()
	from := readFrom(start, retention, timespanHours)
	cursor, err := r.aggregateUnion(ctx, profileId, from, topBlocklistsBranch(profileId, from), topCountMerge, topRankTail(limit), queryLogTopAggregateOptions())
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Token string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if cursor != nil {
		if err := cursor.All(ctx, &rows); err != nil {
			return nil, err
		}
	}
	results := make([]model.QueryLogTopBlocklist, len(rows))
	for i, row := range rows {
		results[i] = model.QueryLogTopBlocklist{BlocklistID: strings.TrimPrefix(row.Token, filterreasons.BlocklistPrefix), Count: row.Count}
	}
	r.warnIfSlow(ctx, "Query log top blocklists fetch took too long", retention, len(results), start)
	return results, nil
}

// topGroup counts documents matching match per groupExpr, ordered count desc
// then key asc, and decodes the first limit rows into out.
func (r *QueryLogsRepository) topGroup(ctx context.Context, profileId string, from time.Time, match bson.D, groupExpr string, limit int, out any) error {
	branch := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cursor, err := r.aggregateUnion(ctx, profileId, from, branch, topCountMerge, topRankTail(limit), queryLogTopAggregateOptions())
	if err != nil || cursor == nil {
		return err
	}
	return cursor.All(ctx, out)
}

// warnIfSlow logs durations and counts only; domains and IPs are sensitive.
func (r *QueryLogsRepository) warnIfSlow(ctx context.Context, msg string, retention model.Retention, resultCount int, start time.Time) {
	if duration := time.Since(start); duration > slowQueryThreshold {
		log.Ctx(ctx).Warn().
			Bool("slow", true).
			Str("retention", string(retention)).
			Int("result_count", resultCount).
			Dur("duration", duration).
			Msg(msg)
	}
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
		if err != nil {
			return err
		}
		log.Ctx(ctx).Debug().Str("collection_name", string(retention)).Int64("count", res.DeletedCount).Msg("Deleted query logs")
	}

	return nil
}

// ListQueryLogProfileIDs groups on the metaField, which time-series collections answer
// from the buckets without unpacking measurements.
func (r *QueryLogsRepository) ListQueryLogProfileIDs(ctx context.Context) ([]string, error) {
	group := bson.D{primitive.E{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$profile_id"}}}}
	pipeline := mongo.Pipeline{group}
	for _, name := range queryLogsCollectionNames()[1:] {
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
			primitive.E{Key: "coll", Value: name},
			primitive.E{Key: "pipeline", Value: bson.A{group}},
		}}})
	}
	pipeline = append(pipeline, bson.D{primitive.E{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$_id"}}}})

	cursor, err := r.queryLogsCollOneHour.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

// queryLogsCollectionNames lists the retention collections, the 1h one first.
func queryLogsCollectionNames() []string {
	return []string{queryLogsCollOneHour, queryLogsCollSixHours, queryLogsCollOneDay, queryLogsCollOneWeek, queryLogsCollOneMonth}
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

// readFrom is the earliest timestamp a read returns: the start of the timespan, but never older
// than the retention (api-endpoint-behaviour.md J56). Rows of a longer collection wait up to one
// purge run before they are deleted, and must not show in the meantime. timespanHours <= 0
// means the whole retention.
func readFrom(now time.Time, retention model.Retention, timespanHours int) time.Time {
	from := now.Add(-retention.Window())
	if timespanHours > 0 {
		if t := now.Add(-time.Duration(timespanHours) * time.Hour); t.After(from) {
			from = t
		}
	}
	return from
}

// aggregateUnion runs branch on every collection that holds rows of the profile since from, then
// tail. With rows in several collections the branches are merged with $unionWith and merge first;
// with one, the pipeline is that collection's alone. With none it returns a nil cursor.
func (r *QueryLogsRepository) aggregateUnion(ctx context.Context, profileId string, from time.Time, branch, merge, tail mongo.Pipeline, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	colls, err := r.populatedCollections(ctx, profileId, from)
	if err != nil || len(colls) == 0 {
		return nil, err
	}
	pipeline := append(mongo.Pipeline{}, branch...)
	if len(colls) > 1 {
		for _, coll := range colls[1:] {
			pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
				primitive.E{Key: "coll", Value: coll.Name()},
				primitive.E{Key: "pipeline", Value: branch},
			}}})
		}
		pipeline = append(pipeline, merge...)
	}
	pipeline = append(pipeline, tail...)
	return colls[0].Aggregate(ctx, pipeline, opts...)
}

// populatedCollections returns the collections holding at least one row of the profile since
// from, in one round trip: each branch reads one key of the meta+time index.
func (r *QueryLogsRepository) populatedCollections(ctx context.Context, profileId string, from time.Time) ([]*mongo.Collection, error) {
	probe := func(name string) mongo.Pipeline {
		return mongo.Pipeline{
			bson.D{primitive.E{Key: "$match", Value: bson.D{
				primitive.E{Key: "profile_id", Value: profileId},
				primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$gte", Value: from}}},
			}}},
			bson.D{primitive.E{Key: "$limit", Value: 1}},
			bson.D{primitive.E{Key: "$project", Value: bson.D{
				primitive.E{Key: "_id", Value: 0},
				primitive.E{Key: "coll", Value: bson.D{primitive.E{Key: "$literal", Value: name}}},
			}}},
		}
	}
	names := queryLogsCollectionNames()
	pipeline := probe(names[0])
	for _, name := range names[1:] {
		pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
			primitive.E{Key: "coll", Value: name},
			primitive.E{Key: "pipeline", Value: probe(name)},
		}}})
	}
	cursor, err := r.queryLogsCollOneHour.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Coll string `bson:"coll"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]*mongo.Collection, 0, len(rows))
	for _, row := range rows {
		if coll, ok := r.queryLogsCollections[model.Retention(strings.TrimPrefix(row.Coll, collNameQueryLogs+"_"))]; ok {
			out = append(out, coll)
		}
	}
	return out, nil
}

// TrimQueryLogs deletes the profile's rows older than now - retention from every collection longer
// than retention (api-endpoint-behaviour.md J54, J13); the TTL of the retention's own collection
// and of the shorter ones already bounds them. It returns the number of rows deleted.
func (r *QueryLogsRepository) TrimQueryLogs(ctx context.Context, profileId string, retention model.Retention, now time.Time) (int64, error) {
	filter := bson.D{
		primitive.E{Key: "profile_id", Value: profileId},
		primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$lt", Value: now.Add(-retention.Window())}}},
	}
	var deleted int64
	for _, rt := range model.Retentions() {
		if rt.Window() <= retention.Window() {
			continue
		}
		res, err := r.getCollObject(rt).DeleteMany(ctx, filter)
		if err != nil {
			return deleted, err
		}
		deleted += res.DeletedCount
	}
	return deleted, nil
}

// unansweredFilter selects the "No answer" class by outcome
// (query-log-outcomes-behaviour.md C5).
func unansweredFilter() bson.E {
	return bson.E{Key: "outcome", Value: bson.D{{Key: "$in", Value: model.UnansweredOutcomes}}}
}
