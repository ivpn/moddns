package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specRef: proxy-statistics-behaviour.md #Y5 #Y6
func TestFleetAccumulator_SumsIntoHourDocuments(t *testing.T) {
	a := newFleetAccumulator("ams1")
	at := time.Date(2026, 9, 17, 13, 58, 30, 0, time.UTC)

	a.add(model.EventStatistics{Queries: model.Queries{Total: 1}}, at)
	a.add(model.EventStatistics{Queries: model.Queries{Total: 1, Blocked: 1}}, at)
	a.add(model.EventStatistics{Queries: model.Queries{Total: 1, DNSSEC: 1}}, at.Add(90*time.Second))
	assert.Equal(t, 3, a.pending())

	docs := a.take()
	require.Len(t, docs, 2)
	assert.Equal(t, "ams1:2026-09-17T13", docs[0].ID)
	assert.Equal(t, model.Queries{Total: 2, Blocked: 1}, docs[0].Queries)
	assert.Equal(t, "ams1:2026-09-17T14", docs[1].ID)
	assert.Equal(t, model.Queries{Total: 1, DNSSEC: 1}, docs[1].Queries)
}

// specRef: proxy-statistics-behaviour.md #Y8
func TestFleetAccumulator_TakeResets(t *testing.T) {
	a := newFleetAccumulator("ams1")
	assert.Nil(t, a.take(), "nothing pending: nothing to emit")

	a.add(model.EventStatistics{Queries: model.Queries{Total: 1}}, t1300)
	require.Len(t, a.take(), 1)
	assert.Zero(t, a.pending())
	assert.Nil(t, a.take())
}

// specRef: proxy-statistics-behaviour.md #Y1
func TestFleetAccumulator_IgnoresConsentedPart(t *testing.T) {
	a := newFleetAccumulator("ams1")
	a.add(cevt("p1", "d1", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassBlocklist, model.ProtocolDoH), t1300)

	docs := a.take()
	require.Len(t, docs, 1)
	assert.Equal(t, model.Queries{Total: 1, Blocked: 1}, docs[0].Queries)
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestConsentedAccumulator_CountsPerProfileDeviceBucket(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier15Min)
	a.add(cevt("p1", "d1", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassService, model.ProtocolDoT), t1300.Add(14*time.Minute))
	a.add(cevt("p1", "d1", model.Queries{Total: 1}, "", ""), t1300)
	a.add(cevt("p1", "d1", model.Queries{Total: 1}, "", ""), t1300.Add(15*time.Minute))
	assert.Equal(t, 2, a.size())

	docs := a.takeAll()
	require.Len(t, docs, 2)
	var first model.Statistics
	for _, d := range docs {
		assert.Equal(t, model.StatisticsTier15Min, d.Tier)
		if d.BucketStart.Equal(t1300) {
			first = d
		}
	}
	assert.Equal(t, int64(2), first.Total)
	assert.Equal(t, int64(1), first.Blocked)
	assert.Equal(t, int64(1), first.ReasonService)
	assert.Equal(t, int64(1), first.ProtoDoT)
	assert.Zero(t, a.size())
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestConsentedAccumulator_HourTierBucketsByHour(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier1Hour)
	a.add(plain("p1", "d1"), t1300.Add(5*time.Minute))
	a.add(plain("p1", "d1"), t1300.Add(55*time.Minute))
	a.add(plain("p1", "d1"), t1300.Add(time.Hour))

	docs := a.takeAll()
	require.Len(t, docs, 2)
	for _, d := range docs {
		assert.Equal(t, model.StatisticsTier1Hour, d.Tier)
		if d.BucketStart.Equal(t1300) {
			assert.Equal(t, int64(2), d.Total)
		}
	}
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestConsentedAccumulator_DeviceCapFoldsIntoOther(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier15Min)
	for i := 0; i < model.MaxDevicesPerProfileBucket+3; i++ {
		a.add(plain("p1", fmt.Sprintf("dev-%d", i)), t1300)
	}
	a.add(plain("p2", "dev-x"), t1300)

	docs := a.takeAll()
	assert.Len(t, docs, model.MaxDevicesPerProfileBucket+2, "64 devices, _other, and the other profile")
	for _, d := range docs {
		if d.Meta.DeviceID == model.OtherDeviceID {
			assert.Equal(t, int64(3), d.Total)
		}
	}
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsentedAccumulator_TakeClosedLeavesOpenBuckets(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier15Min)
	a.add(plain("p1", "d1"), t1300)
	a.add(plain("p1", "d1"), t1300.Add(15*time.Minute))

	assert.Empty(t, a.takeClosed(t1300.Add(14*time.Minute+59*time.Second)))
	closed := a.takeClosed(t1300.Add(15 * time.Minute))
	require.Len(t, closed, 1)
	assert.True(t, closed[0].BucketStart.Equal(t1300))
	assert.Equal(t, 1, a.size())

	rest := a.takeClosed(t1300.Add(30 * time.Minute))
	require.Len(t, rest, 1)
	assert.Zero(t, a.size())
	assert.Empty(t, a.devices, "device bookkeeping goes with its bucket")
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsentedAccumulator_HourClosesAfterTheHour(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier1Hour)
	a.add(plain("p1", "d1"), t1300.Add(10*time.Minute))

	assert.Empty(t, a.takeClosed(t1300.Add(59*time.Minute+59*time.Second)))
	assert.Len(t, a.takeClosed(t1300.Add(time.Hour)), 1)
}

// specRef: proxy-statistics-behaviour.md #Y27
func TestStatisticsCollector_ClosedChannelDoesNotSpin(t *testing.T) {
	e := newNoEmitter(t)
	c := newTestStatsCollector(t, e, 100, time.Hour)
	var logs lockedBuffer
	restore := captureLogs(&logs)
	defer restore()

	close(c.StatsChan)
	done := make(chan struct{})
	go func() { _ = c.Collect(); close(done) }()
	time.Sleep(100 * time.Millisecond)
	close(c.StopChan)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after its channel closed")
	}

	assert.LessOrEqual(t, logs.count("Channel closed"), 1, "a closed channel is noticed once, not on every loop")
}

// specRef: proxy-statistics-behaviour.md #Y28
func TestConsentedAccumulator_TakeCountsEmitsOpenHourDeltas(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier1Hour)
	a.add(plain("p1", "d1"), t1300.Add(5*time.Minute))
	a.add(plain("p1", "d1"), t1300.Add(10*time.Minute))

	first := a.takeCounts(t1300.Add(15 * time.Minute))
	require.Len(t, first, 1)
	assert.True(t, first[0].BucketStart.Equal(t1300), "the open hour keeps its hour start")
	assert.Equal(t, int64(2), first[0].Total)
	assert.Zero(t, a.size())
	assert.Empty(t, a.takeCounts(t1300.Add(30*time.Minute)), "no new counts: nothing to emit")

	a.add(plain("p1", "d1"), t1300.Add(40*time.Minute))
	second := a.takeCounts(t1300.Add(45 * time.Minute))
	require.Len(t, second, 1)
	assert.Equal(t, int64(1), second[0].Total, "only the counts since the last emit")
	assert.NotEmpty(t, a.devices, "an open hour keeps its device bookkeeping")

	a.add(plain("p1", "d1"), t1300.Add(50*time.Minute))
	closed := a.takeCounts(t1300.Add(time.Hour))
	require.Len(t, closed, 1)
	assert.Equal(t, int64(1), closed[0].Total)
	assert.Empty(t, a.devices, "a closed hour drops its device bookkeeping")
	assert.Empty(t, a.admitted)
}

// specRef: proxy-statistics-behaviour.md #Y15 #Y28
func TestConsentedAccumulator_DeviceCapHoldsAcrossTakeCounts(t *testing.T) {
	a := newConsentedAccumulator(model.StatisticsTier1Hour)
	for i := 0; i < model.MaxDevicesPerProfileBucket+1; i++ {
		a.add(plain("p1", fmt.Sprintf("dev-%d", i)), t1300)
	}
	require.Len(t, a.takeCounts(t1300.Add(15*time.Minute)), model.MaxDevicesPerProfileBucket+1)

	at := t1300.Add(20 * time.Minute)
	a.add(plain("p1", "dev-0"), at)
	a.add(plain("p1", fmt.Sprintf("dev-%d", model.MaxDevicesPerProfileBucket)), at)
	a.add(plain("p1", "dev-new"), at)

	docs := a.takeCounts(t1300.Add(30 * time.Minute))
	require.Len(t, docs, 2)
	byDevice := map[string]int64{}
	for _, d := range docs {
		byDevice[d.Meta.DeviceID] = d.Total
	}
	assert.Equal(t, int64(1), byDevice["dev-0"], "an admitted device keeps its own entry")
	assert.Equal(t, int64(2), byDevice[model.OtherDeviceID], "the cap still holds for the rest of the hour")
}
