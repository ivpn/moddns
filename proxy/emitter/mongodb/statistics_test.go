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
	specs map[string]string // collection name -> type
	err   error
	calls int
}

func (f *fakeLister) ListCollectionSpecifications(_ context.Context, _ interface{}, _ ...*options.ListCollectionsOptions) ([]*mongo.CollectionSpecification, error) {
	f.calls++
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
	docs      []interface{}
	unordered bool
}

func (f fakeInserter) InsertMany(_ context.Context, docs []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
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
	"statistics_30d": "timeseries",
	"statistics_90d": "timeseries",
	"statistics_1y":  "timeseries",
	"query_logs_1h":  "timeseries",
}

func newTestRepo(lister *fakeLister, insertErr error) (*StatisticsRepository, *[]insertCall) {
	var inserts []insertCall
	repo := newStatisticsRepository(lister, func(name string) collectionInserter {
		return fakeInserter{name: name, inserts: &inserts, err: insertErr}
	})
	return repo, &inserts
}

func doc(profile, retention string) model.Statistics {
	return model.Statistics{
		BucketStart: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC),
		Meta:        model.StatisticsMeta{ProfileID: profile, DeviceID: "d"},
		Retention:   retention,
	}
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestStatisticsCollectionFor(t *testing.T) {
	tests := map[string]string{
		"30d":     "statistics_30d",
		"90d":     "statistics_90d",
		"1y":      "statistics_1y",
		"":        "statistics_30d",
		"7d":      "statistics_30d",
		"garbage": "statistics_30d",
	}
	for retention, want := range tests {
		assert.Equal(t, want, statisticsCollectionFor(retention), "retention %q", retention)
	}
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestInsertStatistics_RoutesByRetentionUnordered(t *testing.T) {
	repo, inserts := newTestRepo(&fakeLister{specs: allTimeSeries}, nil)

	err := repo.InsertStatistics(context.Background(), []model.Statistics{
		doc("p1", "30d"), doc("p2", "90d"), doc("p3", "1y"), doc("p4", ""), doc("p5", "weird"), doc("p6", "90d"),
	})
	require.NoError(t, err)

	byColl := map[string]int{}
	for _, c := range *inserts {
		assert.True(t, c.unordered, "InsertMany must be unordered")
		byColl[c.coll] += len(c.docs)
	}
	assert.Equal(t, map[string]int{"statistics_30d": 3, "statistics_90d": 2, "statistics_1y": 1}, byColl)
	assert.Len(t, *inserts, 3, "one InsertMany per collection")
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
	err := repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", "30d")})
	require.Error(t, err)
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_NeverInsertsIntoMissingOrPlainCollections(t *testing.T) {
	tests := []struct {
		name  string
		specs map[string]string
	}{
		{name: "none exist", specs: map[string]string{"query_logs_1h": "timeseries"}},
		{name: "one missing", specs: map[string]string{"statistics_30d": "timeseries", "statistics_90d": "timeseries"}},
		{name: "one is a plain collection", specs: map[string]string{"statistics_30d": "timeseries", "statistics_90d": "collection", "statistics_1y": "timeseries"}},
		{name: "one is a view", specs: map[string]string{"statistics_30d": "timeseries", "statistics_90d": "timeseries", "statistics_1y": "view"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, inserts := newTestRepo(&fakeLister{specs: tt.specs}, nil)
			require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", "30d"), doc("p2", "1y")}))
			assert.Empty(t, *inserts, "the batch is dropped, no InsertMany may run")
			assert.False(t, repo.Ready())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_ListErrorDisablesUntilItRecovers(t *testing.T) {
	lister := &fakeLister{err: errors.New("unreachable")}
	repo, inserts := newTestRepo(lister, nil)

	require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", "30d")}))
	assert.Empty(t, *inserts)

	lister.err = nil
	lister.specs = allTimeSeries
	require.NoError(t, repo.InsertStatistics(context.Background(), []model.Statistics{doc("p1", "30d")}))
	assert.Len(t, *inserts, 1)
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestInsertStatistics_SelfHealsThenStopsChecking(t *testing.T) {
	lister := &fakeLister{specs: map[string]string{}}
	repo, inserts := newTestRepo(lister, nil)
	batch := []model.Statistics{doc("p1", "30d")}

	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Equal(t, 2, lister.calls, "re-checked on every emit while disabled")
	assert.Empty(t, *inserts)

	lister.specs = allTimeSeries // the API migration ran
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Len(t, *inserts, 1)
	assert.True(t, repo.Ready())
	callsAfterHeal := lister.calls

	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	require.NoError(t, repo.InsertStatistics(context.Background(), batch))
	assert.Equal(t, callsAfterHeal, lister.calls, "no re-check once verified")
	assert.Len(t, *inserts, 3)
}

// specRef: proxy-statistics-behaviour.md #Y21
func TestNewStatisticsRepository_VerifiesAtStartup(t *testing.T) {
	lister := &fakeLister{specs: allTimeSeries}
	repo, _ := newTestRepo(lister, nil)
	repo.Verify(context.Background())
	assert.True(t, repo.Ready())
	assert.GreaterOrEqual(t, lister.calls, 1)

	bad := &fakeLister{specs: map[string]string{}}
	repo2, _ := newTestRepo(bad, nil)
	repo2.Verify(context.Background())
	assert.False(t, repo2.Ready())
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
	assert.Contains(t, out, "statistics_30d", "collection names are logged")
}
