package mongodb

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeLister struct {
	specs     map[string]string // collection name -> type
	err       error
	calls     int
	deadlines []bool
}

func (f *fakeLister) ListCollectionSpecifications(ctx context.Context, _ any, _ ...*options.ListCollectionsOptions) ([]*mongo.CollectionSpecification, error) {
	f.calls++
	_, has := ctx.Deadline()
	f.deadlines = append(f.deadlines, has)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]*mongo.CollectionSpecification, 0, len(f.specs))
	for name, typ := range f.specs {
		out = append(out, &mongo.CollectionSpecification{Name: name, Type: typ})
	}
	return out, nil
}

type fakeInserter struct {
	name    string
	inserts *[]insertCall
	err     error
}

type insertCall struct {
	coll      string
	docs      []any
	unordered bool
}

func (f fakeInserter) InsertMany(_ context.Context, docs []any, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	call := insertCall{coll: f.name, docs: docs}
	for _, o := range opts {
		if o != nil && o.Ordered != nil && !*o.Ordered {
			call.unordered = true
		}
	}
	*f.inserts = append(*f.inserts, call)
	return &mongo.InsertManyResult{}, f.err
}

var allTimeSeries = map[string]string{
	"statistics_15min":  "timeseries",
	"statistics_1h":     "timeseries",
	"statistics_1d_30d": "timeseries",
	"statistics_1d_90d": "timeseries",
	"statistics_1d_1y":  "timeseries",
	"query_logs_1h":     "timeseries",
}

func newTestRepo(lister *fakeLister, insertErr error) (*StatisticsRepository, *[]insertCall) {
	var inserts []insertCall
	repo := newStatisticsRepository(lister, func(name string) collectionInserter {
		return fakeInserter{name: name, inserts: &inserts, err: insertErr}
	})
	return repo, &inserts
}

func doc(profile string, retention model.StatisticsRetention) model.Statistics {
	return tierDoc(profile, model.StatisticsTier1Day, retention)
}

func tierDoc(profile string, tier model.StatisticsTier, retention model.StatisticsRetention) model.Statistics {
	return model.Statistics{
		Tier:        tier,
		BucketStart: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC),
		Meta:        model.StatisticsMeta{ProfileID: profile, DeviceID: "d"},
		Retention:   retention,
	}
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestStatisticsCollectionFor(t *testing.T) {
	tests := []struct {
		tier      model.StatisticsTier
		retention model.StatisticsRetention
		want      string
	}{
		{model.StatisticsTier15Min, "", "statistics_15min"},
		{model.StatisticsTier15Min, model.StatisticsRetention1y, "statistics_15min"},
		{model.StatisticsTier1Hour, model.StatisticsRetention90d, "statistics_1h"},
		{model.StatisticsTier1Day, model.StatisticsRetention30d, "statistics_1d_30d"},
		{model.StatisticsTier1Day, model.StatisticsRetention90d, "statistics_1d_90d"},
		{model.StatisticsTier1Day, model.StatisticsRetention1y, "statistics_1d_1y"},
		{model.StatisticsTier1Day, "", "statistics_1d_30d"},
		{model.StatisticsTier1Day, "7d", "statistics_1d_30d"},
		{model.StatisticsTier1Day, "garbage", "statistics_1d_30d"},
		{"", "", "statistics_15min"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, statisticsCollectionFor(tt.tier, tt.retention), "tier %q retention %q", tt.tier, tt.retention)
	}
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestInsertStatistics_RoutesByTierAndRetentionUnordered(t *testing.T) {
	repo, inserts := newTestRepo(&fakeLister{specs: allTimeSeries}, nil)

	err := repo.InsertStatistics(context.Background(), []model.Statistics{
		tierDoc("p1", model.StatisticsTier15Min, model.StatisticsRetention90d),
		tierDoc("p2", model.StatisticsTier15Min, ""),
		tierDoc("p3", model.StatisticsTier1Hour, model.StatisticsRetention1y),
		tierDoc("p4", model.StatisticsTier1Day, model.StatisticsRetention30d),
		tierDoc("p5", model.StatisticsTier1Day, model.StatisticsRetention90d),
		tierDoc("p6", model.StatisticsTier1Day, "weird"),
		tierDoc("p7", model.StatisticsTier1Day, model.StatisticsRetention1y),
		tierDoc("p8", model.StatisticsTier1Day, model.StatisticsRetention90d),
	})
	require.NoError(t, err)

	byColl := map[string]int{}
	for _, c := range *inserts {
		assert.True(t, c.unordered, "InsertMany must be unordered")
		byColl[c.coll] += len(c.docs)
	}
	assert.Equal(t, map[string]int{
		"statistics_15min":  2,
		"statistics_1h":     1,
		"statistics_1d_30d": 2,
		"statistics_1d_90d": 2,
		"statistics_1d_1y":  1,
	}, byColl)
	assert.Len(t, *inserts, 5, "one InsertMany per collection")
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestInsertStatistics_EmptyBatchTouchesNothing(t *testing.T) {
	lister := &fakeLister{specs: allTimeSeries}
	repo, inserts := newTestRepo(lister, nil)
	require.NoError(t, repo.InsertStatistics(context.Background(), nil))
	assert.Empty(t, *inserts)
}

// specRef: proxy-statistics-behaviour.md #Y22
func TestInsertStatistics_InsertErrorIsReturned(t *testing.T) {
	repo, _ := newTestRepo(&fakeLister{specs: allTimeSeries}, errors.New("boom"))
	err := repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", model.StatisticsRetention30d)})
	require.Error(t, err)
}

func without(m map[string]string, key string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func with(m map[string]string, key, typ string) map[string]string {
	out := without(m, key)
	out[key] = typ
	return out
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_NeverInsertsIntoMissingOrPlainCollections(t *testing.T) {
	tests := []struct {
		name  string
		specs map[string]string
	}{
		{name: "none exist", specs: map[string]string{"query_logs_1h": "timeseries"}},
		{name: "one missing", specs: without(allTimeSeries, "statistics_1d_1y")},
		{name: "one is a plain collection", specs: with(allTimeSeries, "statistics_1h", "collection")},
		{name: "one is a view", specs: with(allTimeSeries, "statistics_15min", "view")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, inserts := newTestRepo(&fakeLister{specs: tt.specs}, nil)
			require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", model.StatisticsRetention30d), doc("p2", model.StatisticsRetention1y)}))
			assert.Empty(t, *inserts, "the batch is dropped, no InsertMany may run")
			assert.False(t, repo.Verify(context.Background()))
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_ListErrorDisablesUntilItRecovers(t *testing.T) {
	lister := &fakeLister{err: errors.New("unreachable")}
	repo, inserts := newTestRepo(lister, nil)

	require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", model.StatisticsRetention30d)}))
	assert.Empty(t, *inserts)

	lister.err = nil
	lister.specs = allTimeSeries
	require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", model.StatisticsRetention30d)}))
	assert.Len(t, *inserts, 1)
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_SelfHealsThenVerifiesOnlyOnce(t *testing.T) {
	lister := &fakeLister{specs: map[string]string{}}
	repo, inserts := newTestRepo(lister, nil)
	batch := []model.Statistics{doc("p1", model.StatisticsRetention30d)}

	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Equal(t, 2, lister.calls, "re-checked on every emit while disabled")
	assert.Empty(t, *inserts)

	lister.specs = allTimeSeries // the API migration ran
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Len(t, *inserts, 1)
	callsAfterHeal := lister.calls

	// A collection dropped while the proxy runs is not detected: verified once, never again.
	lister.specs = map[string]string{}
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Equal(t, callsAfterHeal, lister.calls)
	assert.Len(t, *inserts, 3)
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestVerifyAtStartup_UsesABoundedContext(t *testing.T) {
	lister := &fakeLister{specs: allTimeSeries}
	repo, _ := newTestRepo(lister, nil)

	repo.VerifyAtStartup()

	require.Len(t, lister.deadlines, 1)
	assert.True(t, lister.deadlines[0], "the startup check must not hang on an unreachable database")
	assert.True(t, repo.Verify(context.Background()))
	assert.Equal(t, 1, lister.calls, "the startup result is final")
}

// specRef: proxy-statistics-behaviour.md #Y23
func TestInsertStatistics_LogsCarryNoIdentifiers(t *testing.T) {
	var buf bytes.Buffer
	old := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = old })

	repo, _ := newTestRepo(&fakeLister{specs: map[string]string{}}, nil)
	require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{
		{Meta: model.StatisticsMeta{ProfileID: "profile-secret", DeviceID: "device-secret"}},
	}))
	repo.Verify(context.Background())

	out := buf.String()
	assert.NotEmpty(t, out)
	assert.False(t, strings.Contains(out, "secret"), "log output must not carry ids: %s", out)
	assert.Contains(t, out, "statistics_15min", "collection names are logged")
}
