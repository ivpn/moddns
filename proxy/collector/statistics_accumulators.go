package collector

import (
	"sort"
	"time"

	"github.com/ivpn/dns/proxy/model"
)

// fleetAccumulator sums events into one service-wide document per PoP and hour.
// Not safe for concurrent use.
type fleetAccumulator struct {
	pop     string
	buckets map[time.Time]*model.ServiceStatistics
	events  int
}

func newFleetAccumulator(pop string) *fleetAccumulator {
	return &fleetAccumulator{pop: pop, buckets: make(map[time.Time]*model.ServiceStatistics)}
}

// add sums one event into the document for the hour containing now.
func (a *fleetAccumulator) add(event model.EventStatistics, now time.Time) {
	bucket := model.BucketStart(now)
	doc, ok := a.buckets[bucket]
	if !ok {
		doc = model.NewServiceStatistics(a.pop, now)
		a.buckets[bucket] = doc
	}
	doc.Aggregate(event)
	a.events++
}

// pending is the number of events added since the last take.
func (a *fleetAccumulator) pending() int { return a.events }

// take returns the open documents ordered by hour and resets the accumulator.
func (a *fleetAccumulator) take() []model.ServiceStatistics {
	if len(a.buckets) == 0 {
		return nil
	}
	batch := make([]model.ServiceStatistics, 0, len(a.buckets))
	for _, doc := range a.buckets {
		batch = append(batch, *doc)
	}
	sort.Slice(batch, func(i, j int) bool { return batch[i].Timestamp.Before(batch[j].Timestamp) })
	a.buckets = make(map[time.Time]*model.ServiceStatistics)
	a.events = 0
	return batch
}

type consentedKey struct {
	profileID string
	deviceID  string
	bucket    time.Time
}

type profileBucket struct {
	profileID string
	bucket    time.Time
}

// consentedAccumulator holds per-profile, per-device counters for the open
// 15-minute buckets. Not safe for concurrent use.
type consentedAccumulator struct {
	maxOpen int
	entries map[consentedKey]*model.Statistics
	devices map[profileBucket]int
}

func newConsentedAccumulator(maxOpen int) *consentedAccumulator {
	return &consentedAccumulator{
		maxOpen: maxOpen,
		entries: make(map[consentedKey]*model.Statistics),
		devices: make(map[profileBucket]int),
	}
}

// add counts the event in the entry for its profile, device and the bucket
// containing now. Devices beyond the per-profile cap share one entry.
func (a *consentedAccumulator) add(event model.EventStatistics, now time.Time) {
	cs := event.Consented
	bucket := model.StatisticsBucketStart(now)
	key := consentedKey{profileID: cs.ProfileID, deviceID: cs.DeviceID, bucket: bucket}

	doc, ok := a.entries[key]
	if !ok {
		pb := profileBucket{profileID: cs.ProfileID, bucket: bucket}
		if a.devices[pb] >= model.MaxDevicesPerProfileBucket {
			key.deviceID = model.OtherDeviceID
			doc, ok = a.entries[key]
		} else {
			a.devices[pb]++
		}
	}
	if !ok {
		doc = &model.Statistics{
			BucketStart: bucket,
			Meta:        model.StatisticsMeta{ProfileID: key.profileID, DeviceID: key.deviceID},
		}
		a.entries[key] = doc
	}
	doc.Retention = string(cs.Retention)
	doc.Count(event.Queries, cs)
}

func (a *consentedAccumulator) size() int { return len(a.entries) }

// full reports whether the open entries have reached the cap.
func (a *consentedAccumulator) full() bool { return len(a.entries) >= a.maxOpen }

// takeClosed removes and returns the entries whose bucket closed at or before now.
func (a *consentedAccumulator) takeClosed(now time.Time) []model.Statistics {
	return a.take(func(bucket time.Time) bool { return !bucket.Add(model.StatisticsBucket).After(now) })
}

// takeAll removes and returns every entry, open buckets included.
func (a *consentedAccumulator) takeAll() []model.Statistics {
	return a.take(func(time.Time) bool { return true })
}

func (a *consentedAccumulator) take(want func(bucket time.Time) bool) []model.Statistics {
	var batch []model.Statistics
	for key, doc := range a.entries {
		if want(key.bucket) {
			batch = append(batch, *doc)
			delete(a.entries, key)
		}
	}
	for pb := range a.devices {
		if want(pb.bucket) {
			delete(a.devices, pb)
		}
	}
	return batch
}
