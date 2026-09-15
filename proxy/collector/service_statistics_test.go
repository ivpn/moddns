package collector

import (
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestStatsCollector(t *testing.T, emitter *mocks.Emitter, batchSize int, freq time.Duration) *ServiceStatisticsCollector {
	t.Helper()
	return &ServiceStatisticsCollector{
		Type:      model.TYPE_STATISTICS,
		BatchSize: batchSize,
		Frequency: freq,
		StopChan:  make(chan struct{}),
		StatsChan: make(chan model.EventStatistics, batchSize),
		Emitter:   emitter,
		Pop:       "ams1",
	}
}

func evt(q model.Queries) model.EventStatistics {
	return model.EventStatistics{Queries: q}
}

// specRef: proxy-statistics-behaviour.md #Y5 #Y6
func TestStatisticsCollector_SumsEventsIntoOneDocumentPerFlush(t *testing.T) {
	emitter := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, emitter, 100, time.Minute)
	flushedAt := time.Date(2026, 9, 15, 13, 58, 30, 0, time.UTC)
	c.Now = func() time.Time { return flushedAt }

	c.add(evt(model.Queries{Total: 1}))
	c.add(evt(model.Queries{Total: 1, Blocked: 1}))
	c.add(evt(model.Queries{Total: 1, DNSSEC: 1}))

	var got []model.ServiceStatistics
	emitter.On("EmitServiceStatistics", mock.Anything, mock.MatchedBy(func(batch []model.ServiceStatistics) bool {
		got = batch
		return true
	})).Return(nil).Once()

	c.flush("test")

	require.Len(t, got, 1, "one service-wide document per flush")
	assert.Equal(t, model.Queries{Total: 3, Blocked: 1, DNSSEC: 1}, got[0].Queries)
	assert.True(t, got[0].Timestamp.Equal(flushedAt), "stamped at flush")
	assert.Equal(t, "ams1", got[0].Pop)
}

// specRef: proxy-statistics-behaviour.md #Y8
func TestStatisticsCollector_FlushResetsAccumulator(t *testing.T) {
	emitter := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, emitter, 100, time.Minute)

	emitter.On("EmitServiceStatistics", mock.Anything, mock.MatchedBy(func(batch []model.ServiceStatistics) bool {
		return len(batch) == 1 && batch[0].Queries.Total == 3
	})).Return(nil).Once()

	for i := 0; i < 3; i++ {
		c.add(evt(model.Queries{Total: 1}))
	}
	c.flush("test")

	assert.Nil(t, c.current)
	assert.Zero(t, c.counter)
	c.flush("test") // nothing pending: no emit (the mock would fail on a second call)
}

// specRef: proxy-statistics-behaviour.md #Y8
func TestStatisticsCollector_Collect_FlushesOnBatchSizeAndInterval(t *testing.T) {
	emitter := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, emitter, 2, 50*time.Millisecond)

	batches := make(chan []model.ServiceStatistics, 4)
	emitter.On("EmitServiceStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		batches <- args.Get(1).([]model.ServiceStatistics)
	}).Return(nil)

	done := make(chan struct{})
	go func() { _ = c.Collect(); close(done) }()

	// Two events reach the batch size and flush immediately.
	c.StatsChan <- evt(model.Queries{Total: 1})
	c.StatsChan <- evt(model.Queries{Total: 1, Blocked: 1})
	select {
	case batch := <-batches:
		require.Len(t, batch, 1)
		assert.Equal(t, model.Queries{Total: 2, Blocked: 1}, batch[0].Queries)
	case <-time.After(time.Second):
		t.Fatal("batch-size flush did not happen")
	}

	// One event below the batch size is flushed by the ticker.
	c.StatsChan <- evt(model.Queries{Total: 1})
	select {
	case batch := <-batches:
		require.Len(t, batch, 1)
		assert.Equal(t, model.Queries{Total: 1}, batch[0].Queries)
	case <-time.After(time.Second):
		t.Fatal("interval flush did not happen")
	}

	close(c.StopChan)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector did not stop")
	}
}

// specRef: proxy-statistics-behaviour.md #Y9
func TestStatisticsCollector_EmitErrorDropsBatchAndContinues(t *testing.T) {
	emitter := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, emitter, 100, time.Minute)

	emitter.On("EmitServiceStatistics", mock.Anything, mock.Anything).Return(assert.AnError).Once()
	c.add(evt(model.Queries{Total: 1}))
	assert.NotPanics(t, func() { c.flush("test") })
	assert.Nil(t, c.current, "a failed batch is dropped, not retried")

	emitter.On("EmitServiceStatistics", mock.Anything, mock.MatchedBy(func(batch []model.ServiceStatistics) bool {
		return len(batch) == 1 && batch[0].Queries.Total == 1
	})).Return(nil).Once()
	c.add(evt(model.Queries{Total: 1}))
	c.flush("test")
}
