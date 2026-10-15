package mongodb

import (
	"context"
	stderrors "errors"
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

	cursor, err := r.aggregateUnion(ctx, profileId, retention, branch, nil, tail)
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
// profile's query logs (the collections the retention reads, whole window — the TTL
// already bounds it), each with its most recent timestamp, sorted by device ID.
// Aggregation, not Collection.Distinct(): the distinct command is unsupported
// on time-series collections.
func (r *QueryLogsRepository) GetQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error) {
	start := time.Now()

	branch := mongo.Pipeline{
		bson.D{primitive.E{Key: "$match", Value: bson.D{
			primitive.E{Key: "profile_id", Value: profileId},
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

	cursor, err := r.aggregateUnion(ctx, profileId, retention, branch, merge, tail)
	if err != nil {
		return nil, err
	}

	results := make([]model.QueryLogDevice, 0)
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
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
	match := bson.D{
		{Key: "profile_id", Value: profileId},
		{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: time.Now().Add(-time.Duration(timespanHours) * time.Hour)}}},
		{Key: "status", Value: status},
		{Key: "dns_request.domain", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
	}
	results := make([]model.QueryLogTopDomain, 0)
	if err := r.topGroup(ctx, profileId, retention, match, "$dns_request.domain", limit, &results); err != nil {
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
	match := bson.D{
		{Key: "profile_id", Value: profileId},
		{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: time.Now().Add(-time.Duration(timespanHours) * time.Hour)}}},
		{Key: "client_ip", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
	}
	results := make([]model.QueryLogTopClient, 0)
	if err := r.topGroup(ctx, profileId, retention, match, "$client_ip", limit, &results); err != nil {
		return nil, err
	}
	r.warnIfSlow(ctx, "Query log top clients fetch took too long", retention, len(results), start)
	return results, nil
}

// GetQueryLogCoveredFrom returns the timestamp of the profile's oldest row in
// the timespan window, truncated to the minute, or nil when the window is empty.
func (r *QueryLogsRepository) GetQueryLogCoveredFrom(ctx context.Context, profileId string, retention model.Retention, timespanHours int) (*time.Time, error) {
	oldest := bson.D{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: 1}}}}
	first := bson.D{{Key: "$limit", Value: 1}}
	branch := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "profile_id", Value: profileId},
			{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: time.Now().Add(-time.Duration(timespanHours) * time.Hour)}}},
		}}},
		oldest,
		first,
		bson.D{{Key: "$project", Value: bson.D{{Key: "timestamp", Value: 1}}}},
	}
	cursor, err := r.aggregateUnion(ctx, profileId, retention, branch, nil, mongo.Pipeline{oldest, first})
	if err != nil {
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
	from := time.Now().Add(-time.Duration(timespanHours) * time.Hour)
	cursor, err := r.aggregateUnion(ctx, profileId, retention, topBlocklistsBranch(profileId, from), topCountMerge, topRankTail(limit), queryLogTopAggregateOptions())
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Token string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
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
func (r *QueryLogsRepository) topGroup(ctx context.Context, profileId string, retention model.Retention, match bson.D, groupExpr string, limit int, out any) error {
	branch := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cursor, err := r.aggregateUnion(ctx, profileId, retention, branch, topCountMerge, topRankTail(limit), queryLogTopAggregateOptions())
	if err != nil {
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

// readRetentions are the collections a read covers: the retention's own and every shorter one,
// which keep rows from before a raise until their TTL (api-endpoint-behaviour.md J56). Longer ones
// hold only rows a lowering move has not reached yet; reading them could count a batch twice.
func readRetentions(retention model.Retention) []model.Retention {
	out := make([]model.Retention, 0, len(model.Retentions()))
	for _, rt := range model.Retentions() {
		if rt.Window() <= retention.Window() {
			out = append(out, rt)
		}
	}
	return out
}

// aggregateUnion runs branch on every collection the retention reads that holds rows of the
// profile, then tail. With rows in several collections the branches are merged with $unionWith
// and merge first; with one, the pipeline is that collection's alone, which keeps the top-list
// aggregations as fast as a single-collection read.
func (r *QueryLogsRepository) aggregateUnion(ctx context.Context, profileId string, retention model.Retention, branch, merge, tail mongo.Pipeline, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	read, err := r.populatedReadCollections(ctx, profileId, retention)
	if err != nil {
		return nil, err
	}
	pipeline := append(mongo.Pipeline{}, branch...)
	if len(read) > 1 {
		for _, coll := range read[1:] {
			pipeline = append(pipeline, bson.D{primitive.E{Key: "$unionWith", Value: bson.D{
				primitive.E{Key: "coll", Value: coll.Name()},
				primitive.E{Key: "pipeline", Value: branch},
			}}})
		}
		pipeline = append(pipeline, merge...)
	}
	pipeline = append(pipeline, tail...)
	return read[0].Aggregate(ctx, pipeline, opts...)
}

// populatedReadCollections returns the collections the retention reads that hold at least one row
// of the profile, or the retention's own when none does. Each check is one key of the meta+time index.
func (r *QueryLogsRepository) populatedReadCollections(ctx context.Context, profileId string, retention model.Retention) ([]*mongo.Collection, error) {
	read := readRetentions(retention)
	if len(read) == 1 {
		return []*mongo.Collection{r.getCollObject(read[0])}, nil
	}
	out := make([]*mongo.Collection, 0, len(read))
	opts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "_id", Value: 1}})
	for _, rt := range read {
		coll := r.getCollObject(rt)
		err := coll.FindOne(ctx, bson.D{primitive.E{Key: "profile_id", Value: profileId}}, opts).Err()
		if err == mongo.ErrNoDocuments {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, coll)
	}
	if len(out) == 0 {
		out = append(out, r.getCollObject(retention))
	}
	return out, nil
}

// unansweredFilter selects the "No answer" class by outcome
// (query-log-outcomes-behaviour.md C5).
func unansweredFilter() bson.E {
	return bson.E{Key: "outcome", Value: bson.D{{Key: "$in", Value: model.UnansweredOutcomes}}}
}

const (
	queryLogsMoveBatch       = 1000
	queryLogsMoveUndoTimeout = 30 * time.Second
)

// ConformQueryLogs makes the profile's stored rows match the retention (api-endpoint-behaviour.md
// J54, J13): rows older than now - retention are deleted from every collection, and the rest of the
// longer collections move unchanged into the retention's own. It returns the rows moved and deleted.
func (r *QueryLogsRepository) ConformQueryLogs(ctx context.Context, profileId string, retention model.Retention, now time.Time) (int, int64, error) {
	since := now.Add(-retention.Window())
	target := r.getCollObject(retention)
	moved, deleted := 0, int64(0)
	for _, rt := range model.Retentions() {
		coll := r.getCollObject(rt)
		res, err := coll.DeleteMany(ctx, bson.D{
			primitive.E{Key: "profile_id", Value: profileId},
			primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$lt", Value: since}}},
		})
		if err != nil {
			return moved, deleted, err
		}
		deleted += res.DeletedCount
		if rt.Window() <= retention.Window() {
			continue
		}
		n, err := moveQueryLogs(ctx, coll, target, profileId)
		moved += n
		if err != nil {
			return moved, deleted, err
		}
	}
	return moved, deleted, nil
}

// moveQueryLogs copies then deletes batch by batch: $merge and $out cannot write into a
// time-series collection.
func moveQueryLogs(ctx context.Context, source, target *mongo.Collection, profileId string) (int, error) {
	moved := 0
	for {
		opts := options.Find().SetSort(bson.D{primitive.E{Key: "timestamp", Value: 1}}).SetLimit(queryLogsMoveBatch)
		cur, err := source.Find(ctx, bson.D{primitive.E{Key: "profile_id", Value: profileId}}, opts)
		if err != nil {
			return moved, err
		}
		var docs []bson.D
		if err := cur.All(ctx, &docs); err != nil {
			return moved, err
		}
		if len(docs) == 0 {
			return moved, nil
		}

		batch := queryLogsBatchFilter(profileId, docs)
		// Time-series collections do not enforce a unique _id: a batch whose earlier run stopped
		// between insert and delete already has copies in the target, so replace them.
		if _, err := target.DeleteMany(ctx, batch); err != nil {
			return moved, err
		}
		undo := func(err error) error {
			// The batch may have failed on ctx's deadline; the undo must still run.
			undoCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), queryLogsMoveUndoTimeout)
			defer cancel()
			if _, undoErr := target.DeleteMany(undoCtx, batch); undoErr != nil {
				return stderrors.Join(err, undoErr)
			}
			return err
		}
		rows := make([]any, len(docs))
		for i, d := range docs {
			rows[i] = d
		}
		if _, err := target.InsertMany(ctx, rows); err != nil {
			return moved, undo(err)
		}
		if _, err := source.DeleteMany(ctx, batch); err != nil {
			return moved, undo(err)
		}
		moved += len(docs)
	}
}

// queryLogsBatchFilter selects a move batch. The timestamp range bounds the profile index scan to
// the batch's buckets; `_id $in` alone cannot narrow it.
func queryLogsBatchFilter(profileId string, docs []bson.D) bson.D {
	ids := make(bson.A, 0, len(docs))
	var lo, hi primitive.DateTime
	for i, doc := range docs {
		ids = append(ids, docField(doc, "_id"))
		ts, _ := docField(doc, "timestamp").(primitive.DateTime)
		if i == 0 || ts < lo {
			lo = ts
		}
		if i == 0 || ts > hi {
			hi = ts
		}
	}
	return bson.D{
		primitive.E{Key: "profile_id", Value: profileId},
		primitive.E{Key: "timestamp", Value: bson.D{primitive.E{Key: "$gte", Value: lo}, primitive.E{Key: "$lte", Value: hi}}},
		primitive.E{Key: "_id", Value: bson.D{primitive.E{Key: "$in", Value: ids}}},
	}
}
