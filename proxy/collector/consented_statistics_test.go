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
			Retention:   model.StatisticsRetention30d,
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

// captureStatsEmit records the documents of every EmitStatistics call.
func captureStatsEmit(e *mocks.Emitter, into *[][]model.Statistics) {
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*into = append(*into, args.Get(1).([]model.Statistics))
	}).Return(nil)
}

// all flattens the emitted calls.
func all(calls [][]model.Statistics) []model.Statistics {
	var out []model.Statistics
	for _, c := range calls {
		out = append(out, c...)
	}
	return out
}

func tierDocs(docs []model.Statistics, tier model.StatisticsTier) []model.Statistics {
	var out []model.Statistics
	for _, d := range docs {
		if d.Tier == tier {
			out = append(out, d)
		}
	}
	return out
}

func docFor(t *testing.T, docs []model.Statistics, tier model.StatisticsTier, profile, device string, bucket time.Time) model.Statistics {
	t.Helper()
	for _, d := range docs {
		if d.Tier == tier && d.Meta.ProfileID == profile && d.Meta.DeviceID == device && d.BucketStart.Equal(bucket) {
			return d
		}
	}
	t.Fatalf("no %s document for profile %q device %q bucket %s", tier, profile, device, bucket)
	return model.Statistics{}
}

// specRef: proxy-statistics-behaviour.md #Y14 #Y17 #Y19
func TestConsented_AccumulatesPerTierProfileDeviceBucket(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, clock := newConsentedCollector(t, e, time.Date(2026, 9, 17, 13, 14, 59, 0, time.UTC))
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.add(cevt("p1", "d1", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassBlocklist, model.ProtocolDoH))
	c.add(cevt("p1", "d1", model.Queries{Total: 1, DNSSEC: 1}, "", model.ProtocolDoT))
	c.add(cevt("p1", "d2", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassCustomRule, model.ProtocolDoQ))
	c.add(cevt("p2", "d1", model.Queries{Total: 1}, "", ""))
	*clock = time.Date(2026, 9, 17, 13, 15, 0, 0, time.UTC) // next quarter, same hour
	c.add(plain("p1", "d1"))

	c.flushConsented(true)
	docs := all(emitted)

	quarters := tierDocs(docs, model.StatisticsTier15Min)
	require.Len(t, quarters, 4)
	first := docFor(t, quarters, model.StatisticsTier15Min, "p1", "d1", t1300)
	assert.Equal(t, int64(2), first.Total)
	assert.Equal(t, int64(1), first.Blocked)
	assert.Equal(t, int64(1), first.DNSSEC)
	assert.Equal(t, int64(1), first.ReasonBlocklist)
	assert.Equal(t, int64(1), first.ProtoDoH)
	assert.Equal(t, int64(1), first.ProtoDoT)
	assert.Equal(t, model.StatisticsRetention30d, first.Retention)
	d2 := docFor(t, quarters, model.StatisticsTier15Min, "p1", "d2", t1300)
	assert.Equal(t, int64(1), d2.ReasonCustomRule)
	assert.Equal(t, int64(1), d2.ProtoDoQ)
	next := docFor(t, quarters, model.StatisticsTier15Min, "p1", "d1", t1300.Add(15*time.Minute))
	assert.Equal(t, int64(1), next.Total)

	hours := tierDocs(docs, model.StatisticsTier1Hour)
	require.Len(t, hours, 3, "the hour tier has one entry per profile and device")
	hourP1D1 := docFor(t, hours, model.StatisticsTier1Hour, "p1", "d1", t1300)
	assert.Equal(t, int64(3), hourP1D1.Total, "both quarters add into the hour")

	days := tierDocs(docs, model.StatisticsTier1Day)
	require.Len(t, days, 3, "every hour entry is also written as a 1-day measurement")
	dayStart := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	dayP1D1 := docFor(t, days, model.StatisticsTier1Day, "p1", "d1", dayStart)
	assert.Equal(t, int64(3), dayP1D1.Total)
	assert.Equal(t, model.StatisticsRetention30d, dayP1D1.Retention)
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
	ev2.Consented.Retention = model.StatisticsRetention1y
	c.add(ev2)
	c.add(model.EventStatistics{Queries: model.Queries{Total: 1}}) // not consented

	assert.Equal(t, 3, c.fleet.events, "every event counts toward the fleet batch")
	require.Len(t, c.fleet.buckets, 1)
	for _, doc := range c.fleet.buckets {
		assert.Equal(t, model.Queries{Total: 3}, doc.Queries)
	}
	c.flushConsented(true)

	days := tierDocs(all(emitted), model.StatisticsTier1Day)
	require.Len(t, days, 1)
	assert.Equal(t, model.StatisticsRetention1y, days[0].Retention)
	assert.Equal(t, int64(2), days[0].Total, "the non-consented event is in no per-profile document")
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

	quarters := tierDocs(all(emitted), model.StatisticsTier15Min)
	require.Len(t, quarters, 1)
	assert.Equal(t, "", quarters[0].Meta.DeviceID)
	assert.Equal(t, int64(2), quarters[0].Total)
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestConsented_DeviceCapFoldsIntoOtherInEachTier(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, clock := newConsentedCollector(t, e, t1300)
	c.MaxOpenEntries = 100000
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
	c.add(plain("p1", "dev-new-1")) // new quarter bucket: its cap starts again; same hour bucket: folds

	c.flushConsented(true)
	docs := all(emitted)

	other := docFor(t, docs, model.StatisticsTier15Min, "p1", model.OtherDeviceID, t1300)
	assert.Equal(t, int64(2), other.Total)
	assert.Equal(t, int64(2), docFor(t, docs, model.StatisticsTier15Min, "p1", "dev-0", t1300).Total)
	docFor(t, docs, model.StatisticsTier15Min, "p2", "dev-new-1", t1300)
	docFor(t, docs, model.StatisticsTier15Min, "p1", "dev-new-1", t1300.Add(15*time.Minute))

	hourOther := docFor(t, docs, model.StatisticsTier1Hour, "p1", model.OtherDeviceID, t1300)
	assert.Equal(t, int64(3), hourOther.Total, "the hour tier applies its own cap")
	for _, tier := range []model.StatisticsTier{model.StatisticsTier15Min, model.StatisticsTier1Hour} {
		p1First := 0
		for _, d := range tierDocs(docs, tier) {
			if d.Meta.ProfileID == "p1" && d.BucketStart.Equal(t1300) && d.Meta.DeviceID != model.OtherDeviceID {
				p1First++
			}
		}
		assert.Equal(t, model.MaxDevicesPerProfileBucket, p1First, tier)
	}
}

// specRef: proxy-statistics-behaviour.md #Y16
func TestConsented_EntryCapCountsBothTiersAndFlushesEverything(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	c.MaxOpenEntries = 4
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	c.add(plain("p1", "a")) // 2 entries: one per tier
	c.add(plain("p1", "a"))
	assert.Empty(t, emitted, "below the cap nothing is flushed")

	c.add(plain("p1", "b")) // 4 entries: reaches the cap

	require.Len(t, emitted, 1)
	docs := emitted[0]
	assert.Len(t, tierDocs(docs, model.StatisticsTier15Min), 2)
	assert.Len(t, tierDocs(docs, model.StatisticsTier1Hour), 2, "open buckets are included")
	assert.Len(t, tierDocs(docs, model.StatisticsTier1Day), 2)
	assert.Zero(t, c.quarter.size()+c.hour.size())
	assert.Empty(t, c.quarter.devices)
	assert.Empty(t, c.hour.devices)
}

// specRef: proxy-statistics-behaviour.md #Y16 #Y26
func TestConsented_DefaultCapFlushesInAtMostFourChunks(t *testing.T) {
	assert.Equal(t, 12000, DefaultMaxOpenEntries)
	// A flush at the cap writes up to 1.5 documents per entry (quarter + hour + day for
	// each pair of entries), so the cap keeps it within four chunks.
	assert.LessOrEqual(t, DefaultMaxOpenEntries*3/2, 4*StatisticsEmitChunkSize)

	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	var sizes []int
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sizes = append(sizes, len(args.Get(1).([]model.Statistics)))
	}).Return(nil)

	for i := 0; i < DefaultMaxOpenEntries/2; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}

	assert.Equal(t, []int{5000, 5000, 5000, 3000}, sizes)
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsented_TickEmitsOnlyClosedBucketsPerTier(t *testing.T) {
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
	assert.Empty(t, emitted, "nothing is closed at 13:14:59")

	*clock = t1300.Add(15 * time.Minute)
	c.tick()
	require.Len(t, emitted, 1)
	require.Len(t, emitted[0], 1, "only the 13:00 quarter closed; the hour is still open")
	assert.Equal(t, model.StatisticsTier15Min, emitted[0][0].Tier)
	assert.True(t, emitted[0][0].BucketStart.Equal(t1300))

	c.tick()
	assert.Len(t, emitted, 1, "nothing newly closed: nothing emitted")

	*clock = t1300.Add(30 * time.Minute)
	c.tick()
	require.Len(t, emitted, 2)
	assert.True(t, emitted[1][0].BucketStart.Equal(t1300.Add(15*time.Minute)))

	*clock = t1300.Add(time.Hour)
	c.tick()
	require.Len(t, emitted, 3)
	hour := tierDocs(emitted[2], model.StatisticsTier1Hour)
	day := tierDocs(emitted[2], model.StatisticsTier1Day)
	require.Len(t, hour, 1)
	require.Len(t, day, 1, "a closed hour is also written to the 1-day tier")
	assert.Equal(t, int64(2), hour[0].Total)
	assert.True(t, day[0].BucketStart.Equal(time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)))
	assert.Zero(t, c.quarter.size()+c.hour.size())
	assert.Empty(t, c.hour.devices)
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
	c.Now = func() time.Time { return t1300.Add(7 * time.Minute) }

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
	docs := all(emitted)
	require.Len(t, docs, 3, "the partial quarter, the partial hour and its 1-day measurement")
	for _, tier := range []model.StatisticsTier{model.StatisticsTier15Min, model.StatisticsTier1Hour, model.StatisticsTier1Day} {
		d := tierDocs(docs, tier)
		require.Len(t, d, 1, tier)
		assert.Equal(t, int64(2), d[0].Total, tier)
	}
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsented_CollectTickEmitsClosedBucket(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, e, 100, 20*time.Millisecond)
	var clock atomic.Int64
	clock.Store(t1300.UnixNano())
	c.Now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
	got := make(chan []model.Statistics, 4)
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
		assert.Equal(t, model.StatisticsTier15Min, batch[0].Tier)
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

	c.MaxOpenEntries = 4
	c.add(plain("profile-secret-1", "device-secret-1"))
	assert.NotPanics(t, func() { c.add(plain("profile-secret-2", "device-secret-2")) })

	assert.Zero(t, c.quarter.size()+c.hour.size(), "a failed batch is dropped, not retried")
	assert.NotEmpty(t, buf.String(), "the failure is logged")
	for _, secret := range []string{"profile-secret", "device-secret"} {
		assert.False(t, strings.Contains(buf.String(), secret), "log output must not carry ids: %s", buf.String())
	}

	// The collector keeps working after a failed emit.
	e.On("EmitStatistics", mock.Anything, mock.MatchedBy(func(b []model.Statistics) bool { return len(b) == 3 })).Return(nil).Once()
	c.add(plain("profile-secret-3", "device-secret-3"))
	c.flushConsented(true)
}

// specRef: proxy-statistics-behaviour.md #Y24
func TestConsented_FlushIsChunkedWithOwnTimeoutPerChunk(t *testing.T) {
	e := mocks.NewEmitter(t)
	total := 4000 // profiles: 3 documents each
	c := &StatisticsCollector{Emitter: e, Now: func() time.Time { return t1300 }, MaxOpenEntries: 3 * total}
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
	assert.Zero(t, c.quarter.size()+c.hour.size())
}

// specRef: proxy-statistics-behaviour.md #Y24 #Y22
func TestConsented_FailedChunkLosesOnlyThatChunk(t *testing.T) {
	e := mocks.NewEmitter(t)
	total := 3334 // 10002 documents: three chunks
	c := &StatisticsCollector{Emitter: e, Now: func() time.Time { return t1300 }, MaxOpenEntries: 3 * total}
	for i := 0; i < total; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}

	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(nil).Once()
	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(assert.AnError).Once()
	e.On("EmitStatistics", mock.Anything, mock.Anything).Return(nil).Once()

	c.flushConsented(true)

	e.AssertNumberOfCalls(t, "EmitStatistics", 3)
	assert.Zero(t, c.quarter.size()+c.hour.size())
}

// specRef: proxy-statistics-behaviour.md #Y24 #Y18
func TestConsented_ShutdownBudgetDropsRemainingChunks(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := &StatisticsCollector{Emitter: e, Now: func() time.Time { return t1300 }, MaxOpenEntries: 100000}
	for i := 0; i < StatisticsEmitChunkSize; i++ {
		c.add(plain(fmt.Sprintf("p%d", i), "d"))
	}
	c.flushDeadline = time.Now().Add(-time.Second) // budget already spent

	assert.NotPanics(t, func() { c.flushConsented(true) })

	e.AssertNotCalled(t, "EmitStatistics", mock.Anything, mock.Anything)
	assert.Zero(t, c.quarter.size()+c.hour.size())
}

// specRef: proxy-statistics-behaviour.md #Y15 #Y16
func TestConsented_EntryCapFlushResetsDeviceCap(t *testing.T) {
	e := mocks.NewEmitter(t)
	c, _ := newConsentedCollector(t, e, t1300)
	c.MaxOpenEntries = 2 * (model.MaxDevicesPerProfileBucket + 1)
	var emitted [][]model.Statistics
	captureStatsEmit(e, &emitted)

	for i := 0; i < model.MaxDevicesPerProfileBucket; i++ {
		c.add(plain("p1", fmt.Sprintf("dev-%d", i)))
	}
	c.add(plain("p1", "over")) // folds into _other in both tiers; 130 entries reach the cap
	require.Len(t, emitted, 1)

	c.add(plain("p1", "fresh"))
	c.flushConsented(true)
	require.Len(t, emitted, 2)
	for _, d := range tierDocs(emitted[1], model.StatisticsTier15Min) {
		assert.Equal(t, "fresh", d.Meta.DeviceID, "the device cap starts over after a Y16 flush")
	}
}

// specRef: proxy-statistics-behaviour.md #Y18
func TestConsented_StopDoesNotCapFlushWhileDraining(t *testing.T) {
	e := mocks.NewEmitter(t)
	c := newTestStatsCollector(t, e, 100, time.Hour)
	c.Now = func() time.Time { return t1300 }
	c.MaxOpenEntries = 4
	e.On("EmitServiceStatistics", mock.Anything, mock.Anything).Return(nil)
	var sizes []int
	var deadlines []time.Duration
	e.On("EmitStatistics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sizes = append(sizes, len(args.Get(1).([]model.Statistics)))
		dl, _ := args.Get(0).(context.Context).Deadline()
		deadlines = append(deadlines, time.Until(dl))
	}).Return(nil)

	// stop() is called directly: inside Collect, select may take queued events through the
	// normal path, with its cap flush, before it sees the stop signal.
	for i := 0; i < 5; i++ {
		c.StatsChan <- plain(fmt.Sprintf("p%d", i), "d")
	}
	c.stop()

	assert.Equal(t, []int{15}, sizes, "queued events are added without cap flushes; one final flush emits them")
	require.Len(t, deadlines, 1)
	assert.LessOrEqual(t, deadlines[0], EmitTimeout, "the final flush runs under the shutdown budget")
}
