package collector

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var t1300 = time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)

func cevt(profile, device string, q model.Queries, class model.ReasonClass, proto model.StatisticsProtocol) model.EventStatistics {
	return model.EventStatistics{
		Queries: q,
		Consented: &model.ConsentedStatistics{
			ProfileID:   profile,
			DeviceID:    device,
			Retention:   "30d",
			ReasonClass: class,
			Protocol:    proto,
		},
	}
}

func plain(profile, device string) model.EventStatistics {
	return cevt(profile, device, model.Queries{Total: 1}, "", "")
}

func newConsentedCollector(t *testing.T, e *mocks.Emitter, now time.Time) (*StatisticsCollector, *time.Time) {
	t.Helper()
	c := newTestStatsCollector(t, e, 100, time.Minute)
	clock := now
	c.Now = func() time.Time { return clock }
	return c, &clock
}

func captureStatsEmit(e *mocks.Emitter, into *[][]model.Statistics) {
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*into = append(*into, args.Get(1).([]model.Statistics))
	}).Return(nil)
}

func docFor(t *testing.T, batch []model.Statistics, profile, device string, bucket time.Time) model.Statistics {
	t.Helper()
	for _, d := range batch {
		if d.Meta.ProfileID == profile && d.Meta.DeviceID == device && d.BucketStart.Equal(bucket) {
			return d
		}
	}
	t.Fatalf("no document for device %q bucket %s", device, bucket)
	return model.Statistics{}
}

// specRef: proxy-statistics-behaviour.md #Y14 #Y19
func TestConsented_AccumulatesPerProfileDeviceBucket(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, clock := newConsentedCollector(t, e, time.Date(2026, 9, 17, 13, 14, 59, 0, time.UTC))
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.add(cevt("p1", "d1", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassBlocklist, model.ProtocolDoH))
	c.add(cevt("p1", "d1", model.Queries{Total: 1, DNSSEC: 1}, "", model.ProtocolDoT))
	c.add(cevt("p1", "d2", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassCustomRule, model.ProtocolDoQ))
	c.add(cevt("p2", "d1", model.Queries{Total: 1}, "", ""))
	*clock = time.Date(2026, 9, 17, 13, 15, 0, 0, time.UTC) // next bucket
	c.add(plain("p1", "d1"))

	c.flushConsented(true)

	require.Len(t, emitted, 1)
	batch := emitted[0]
	require.Len(t, batch, 4)
	first := docFor(t, batch, "p1", "d1", t1300)
	assert.Equal(t, model.StatisticsQueries{Total: 2, Blocked: 1, DNSSEC: 1}, first.Queries)
	assert.Equal(t, model.StatisticsReasons{Blocklist: 1}, first.Reasons)
	assert.Equal(t, model.StatisticsProtocols{DoH: 1, DoT: 1}, first.Protocols)
	assert.Equal(t, "30d", first.Retention)
	d2 := docFor(t, batch, "p1", "d2", t1300)
	assert.Equal(t, model.StatisticsReasons{CustomRule: 1}, d2.Reasons)
	assert.Equal(t, model.StatisticsProtocols{DoQ: 1}, d2.Protocols)
	docFor(t, batch, "p2", "d1", t1300)
	next := docFor(t, batch, "p1", "d1", t1300.Add(15*time.Minute))
	assert.Equal(t, int64(1), next.Queries.Total)
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestConsented_KeepsLatestRetentionAndLeavesFleetCountersAlone(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	ev := plain("p1", "d1")
	c.add(ev)
	ev2 := plain("p1", "d1")
	ev2.Consented.Retention = "1y"
	c.add(ev2)
	c.add(model.EventStatistics{Queries: model.Queries{Total: 1}}) // not consented

	assert.Equal(t, 3, c.fleet.events, "every event counts toward the fleet batch")
	require.Len(t, c.fleet.buckets, 1)
	for _, doc := range c.fleet.buckets {
		assert.Equal(t, model.Queries{Total: 3}, doc.Queries)
	}
	c.flushConsented(true)
	require.Len(t, emitted, 1)
	require.Len(t, emitted[0], 1)
	assert.Equal(t, "1y", emitted[0][0].Retention)
	assert.Equal(t, int64(2), emitted[0][0].Queries.Total, "the non-consented event is in no per-profile document")
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestConsented_EmptyDeviceIDIsALegitimateKey(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.add(plain("p1", ""))
	c.add(plain("p1", ""))
	c.flushConsented(true)

	require.Len(t, emitted[0], 1)
	assert.Equal(t, "", emitted[0][0].Meta.DeviceID)
	assert.Equal(t, int64(2), emitted[0][0].Queries.Total)
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestConsented_DeviceCapFoldsIntoOther(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, clock := newConsentedCollector(t, e, t1300)
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	for i := 0; i < model.MaxDevicesPerProfileBucket; i++ {
		c.add(plain("p1", fmt.Sprintf("dev-%d", i)))
	}
	c.add(plain("p1", "dev-new-1"))
	c.add(plain("p1", "dev-new-2"))
	c.add(plain("p1", "dev-0")) // already open: keeps its own entry
	c.add(plain("p2", "dev-new-1"))
	*clock = clock.Add(15 * time.Minute)
	c.add(plain("p1", "dev-new-1")) // new bucket: cap starts again

	c.flushConsented(true)
	batch := emitted[0]

	other := docFor(t, batch, "p1", model.OtherDeviceID, t1300)
	assert.Equal(t, int64(2), other.Queries.Total)
	assert.Equal(t, int64(2), docFor(t, batch, "p1", "dev-0", t1300).Queries.Total)
	docFor(t, batch, "p2", "dev-new-1", t1300)
	docFor(t, batch, "p1", "dev-new-1", t1300.Add(15*time.Minute))
	p1First := 0
	for _, d := range batch {
		if d.Meta.ProfileID == "p1" && d.BucketStart.Equal(t1300) && d.Meta.DeviceID != model.OtherDeviceID {
			p1First++
		}
	}
	assert.Equal(t, model.MaxDevicesPerProfileBucket, p1First)
}

// specRef: proxy-statistics-behaviour.md #Y16
func TestConsented_EntryCapFlushesEverythingIncludingOpenBuckets(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	c.MaxOpenEntries = 3
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.add(plain("p1", "a"))
	c.add(plain("p1", "b"))
	c.add(plain("p1", "b"))
	assert.Empty(t, emitted, "below the cap nothing is flushed")

	c.add(plain("p1", "c"))

	require.Len(t, emitted, 1, "reaching the cap flushes at once")
	assert.Len(t, emitted[0], 3)
	assert.Empty(t, c.consented.entries)
	assert.Empty(t, c.consented.devices)
}

// specRef: proxy-statistics-behaviour.md #Y16 #Y26
func TestConsented_DefaultCapFlushesInAtMostFourChunks(t *testing.T) {
	assert.Equal(t, 20000, DefaultMaxOpenEntries)
	assert.LessOrEqual(t, DefaultMaxOpenEntries/StatisticsEmitChunkSize, 4)

	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	var sizes []int
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sizes = append(sizes, len(args.Get(1).([]model.Statistics)))
	}).Return(nil)

	for i := 0; i < DefaultMaxOpenEntries; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}

	assert.Equal(t, []int{5000, 5000, 5000, 5000}, sizes)
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsented_TickEmitsOnlyClosedBuckets(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, clock := newConsentedCollector(t, e, t1300)
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)
	e.On("EmitServiceStatistics", mock.Anything, mock.Anything).Return(nil)

	c.add(plain("p1", "d1"))
	*clock = t1300.Add(15 * time.Minute)
	c.add(plain("p1", "d1"))

	*clock = t1300.Add(14*time.Minute + 59*time.Second)
	c.tick()
	assert.Empty(t, emitted, "bucket 13:00 is still open at 13:14:59")

	*clock = t1300.Add(15 * time.Minute)
	c.tick()
	require.Len(t, emitted, 1)
	require.Len(t, emitted[0], 1)
	assert.True(t, emitted[0][0].BucketStart.Equal(t1300))
	assert.Len(t, c.consented.entries, 1, "the open bucket stays in memory")

	c.tick()
	assert.Len(t, emitted, 1, "nothing closed: nothing emitted")

	*clock = t1300.Add(30 * time.Minute)
	c.tick()
	require.Len(t, emitted, 2)
	assert.True(t, emitted[1][0].BucketStart.Equal(t1300.Add(15*time.Minute)))
	assert.Empty(t, c.consented.entries)
	assert.Empty(t, c.consented.devices)
}

// specRef: proxy-statistics-behaviour.md #Y17 #Y8
func TestConsented_BatchSizeFlushesFleetOnly(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, e, 2, time.Minute)
	c.Now = func() time.Time { return t1300 }
	fleet := make(chan []model.ServiceStatistics, 1)
	e.On("EmitServiceStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		fleet <- args.Get(1).([]model.ServiceStatistics)
	}).Return(nil).Once()

	done := make(chan struct{})
	go func() { _ = c.Collect(); close(done) }()
	c.StatsChan <- plain("p1", "d1")
	c.StatsChan <- plain("p1", "d2")

	select {
	case batch := <-fleet:
		require.Len(t, batch, 1)
		assert.Equal(t, 2, batch[0].Queries.Total)
	case <-time.After(time.Second):
		t.Fatal("batch-size flush did not happen")
	}
	e.AssertNotCalled(t, "EmitStatistics", mock.Anything, mock.Anything)

	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(nil) // shutdown flush (Y18)
	close(c.StopChan)
	<-done
}

// specRef: proxy-statistics-behaviour.md #Y18
func TestConsented_StopDrainsAndFlushesBothAccumulators(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, e, 100, time.Hour)
	c.Now = func() time.Time { return t1300 }

	var fleet [][]model.ServiceStatistics
	e.On("EmitServiceStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		fleet = append(fleet, args.Get(1).([]model.ServiceStatistics))
	}).Return(nil).Once()
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.StatsChan <- plain("p1", "d1")
	c.StatsChan <- plain("p1", "d1")
	c.StatsChan <- model.EventStatistics{Queries: model.Queries{Total: 1}}
	close(c.StopChan)

	require.NoError(t, c.Collect())

	require.Len(t, fleet, 1)
	assert.Equal(t, 3, fleet[0][0].Queries.Total)
	require.Len(t, emitted, 1)
	assert.Equal(t, int64(2), emitted[0][0].Queries.Total, "an open bucket is flushed on shutdown")
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsented_CollectTickEmitsClosedBucket(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, e, 100, 20*time.Millisecond)
	var clock atomic.Int64
	clock.Store(t1300.UnixNano())
	c.Now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
	got := make(chan []model.Statistics, 2)
	e.On("EmitServiceStatistics", mock.Anything, mock.Anything).Return(nil)
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		got <- args.Get(1).([]model.Statistics)
	}).Return(nil)

	done := make(chan struct{})
	go func() { _ = c.Collect(); close(done) }()
	c.StatsChan <- plain("p1", "d1")
	select {
	case <-got:
		t.Fatal("open bucket must not be emitted by the ticker")
	case <-time.After(100 * time.Millisecond):
	}

	clock.Store(t1300.Add(15 * time.Minute).UnixNano())
	select {
	case batch := <-got:
		require.Len(t, batch, 1)
	case <-time.After(time.Second):
		t.Fatal("closed bucket was not emitted on a tick")
	}
	close(c.StopChan)
	<-done
}

// specRef: proxy-statistics-behaviour.md #Y22 #Y23
func TestConsented_EmitErrorDropsBatchAndLogsNoIdentifiers(t *testing.T) {
	var buf bytes.Buffer
	old := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = old })

	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(assert.AnError).Once()

	c.MaxOpenEntries = 2
	c.add(plain("profile-secret-1", "device-secret-1"))
	assert.NotPanics(t, func() { c.add(plain("profile-secret-2", "device-secret-2")) })

	assert.Empty(t, c.consented.entries, "a failed batch is dropped, not retried")
	assert.NotEmpty(t, buf.String(), "the failure is logged")
	for _, secret := range []string{"profile-secret", "device-secret"} {
		assert.False(t, strings.Contains(buf.String(), secret), "log output must not carry ids: %s", buf.String())
	}

	// The collector keeps working after a failed emit.
	e.On("EmitStatistics", mock.Anything, mock.MatchedBy(func(b []model.Statistics) bool { return len(b) == 1 })).Return(nil).Once()
	c.add(plain("profile-secret-3", "device-secret-3"))
	c.flushConsented(true)
}

// specRef: proxy-statistics-behaviour.md #Y24
func TestConsented_FlushIsChunkedWithOwnTimeoutPerChunk(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	total := 2*StatisticsEmitChunkSize + 2000
	c = &StatisticsCollector{Emitter: e, Now: c.Now, MaxOpenEntries: total + 1}
	for i := 0; i < total; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}

	var sizes []int
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, hasDeadline := args.Get(0).(context.Context).Deadline()
		assert.True(t, hasDeadline, "each chunk has its own timeout")
		sizes = append(sizes, len(args.Get(1).([]model.Statistics)))
	}).Return(nil)

	c.flushConsented(true)

	assert.Equal(t, []int{StatisticsEmitChunkSize, StatisticsEmitChunkSize, 2000}, sizes)
	assert.Empty(t, c.consented.entries)
}

// specRef: proxy-statistics-behaviour.md #Y24 #Y22
func TestConsented_FailedChunkLosesOnlyThatChunk(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := &StatisticsCollector{Emitter: e, Now: func() time.Time { return t1300 }, MaxOpenEntries: 3 * StatisticsEmitChunkSize}
	total := 2*StatisticsEmitChunkSize + 10
	for i := 0; i < total; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}

	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(nil).Once()
	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(assert.AnError).Once()
	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(nil).Once()

	c.flushConsented(true)

	e.AssertNumberOfCalls(t, "EmitStatistics", 3)
	assert.Empty(t, c.consented.entries)
}

// specRef: proxy-statistics-behaviour.md #Y24 #Y18
func TestConsented_ShutdownBudgetDropsRemainingChunks(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := &StatisticsCollector{Emitter: e, Now: func() time.Time { return t1300 }, MaxOpenEntries: 3 * StatisticsEmitChunkSize}
	for i := 0; i < StatisticsEmitChunkSize+1; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}
	c.flushDeadline = time.Now().Add(-time.Second) // budget already spent

	assert.NotPanics(t, func() { c.flushConsented(true) })

	e.AssertNotCalled(t, "EmitStatistics", mock.Anything, mock.Anything)
	assert.Empty(t, c.consented.entries)
}

// specRef: proxy-statistics-behaviour.md #Y15 #Y16
func TestConsented_EntryCapFlushResetsDeviceCap(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	c.MaxOpenEntries = model.MaxDevicesPerProfileBucket + 1
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	for i := 0; i < model.MaxDevicesPerProfileBucket; i++ {
		c.add(plain("p1", fmt.Sprintf("dev-%d", i)))
	}
	c.add(plain("p1", "over")) // folds into _other, the 65th entry, which reaches the cap
	require.Len(t, emitted, 1)

	c.add(plain("p1", "fresh"))
	c.flushConsented(true)
	require.Len(t, emitted, 2)
	assert.Equal(t, "fresh", emitted[1][0].Meta.DeviceID, "the device cap starts over after a Y16 flush")
}
