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
	a := newConsentedAccumulator(100)
	a.add(cevt("p1", "d1", model.Queries{Total: 1, Blocked: 1}, model.ReasonClassService, model.ProtocolDoT), t1300.Add(14*time.Minute))
	a.add(cevt("p1", "d1", model.Queries{Total: 1}, "", ""), t1300)
	a.add(cevt("p1", "d1", model.Queries{Total: 1}, "", ""), t1300.Add(15*time.Minute))
	assert.Equal(t, 2, a.size())

	docs := a.takeAll()
	require.Len(t, docs, 2)
	var first model.Statistics
	for _, d := range docs {
		if d.BucketStart.Equal(t1300) {
			first = d
		}
	}
	assert.Equal(t, model.StatisticsQueries{Total: 2, Blocked: 1}, first.Queries)
	assert.Equal(t, model.StatisticsReasons{Service: 1}, first.Reasons)
	assert.Equal(t, model.StatisticsProtocols{DoT: 1}, first.Protocols)
	assert.Zero(t, a.size())
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestConsentedAccumulator_DeviceCapFoldsIntoOther(t *testing.T) {
	a := newConsentedAccumulator(1000)
	for i := 0; i < model.MaxDevicesPerProfileBucket+3; i++ {
		a.add(plain("p1", fmt.Sprintf("dev-%d", i)), t1300)
	}
	a.add(plain("p2", "dev-x"), t1300)

	docs := a.takeAll()
	assert.Len(t, docs, model.MaxDevicesPerProfileBucket+2, "64 devices, _other, and the other profile")
	for _, d := range docs {
		if d.Meta.DeviceID == model.OtherDeviceID {
			assert.Equal(t, int64(3), d.Queries.Total)
		}
	}
}

// specRef: proxy-statistics-behaviour.md #Y16
func TestConsentedAccumulator_FullWhenEntriesReachTheCap(t *testing.T) {
	a := newConsentedAccumulator(2)
	a.add(plain("p1", "a"), t1300)
	assert.False(t, a.full())
	a.add(plain("p1", "a"), t1300)
	assert.False(t, a.full(), "the same entry is not a new one")
	a.add(plain("p1", "b"), t1300)
	assert.True(t, a.full())
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestConsentedAccumulator_TakeClosedLeavesOpenBuckets(t *testing.T) {
	a := newConsentedAccumulator(100)
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
