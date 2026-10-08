package collector

import (
	"context"
	"time"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
)

// StatisticsCollector feeds per-query events to a fleet accumulator (one
// service-wide document per hour for this PoP) and, for consenting profiles, to two
// consented accumulators (per-profile, per-device 15-minute and 1-hour entries), and
// emits all of them. Hours are also written as 1-day measurements. Only the
// Collect goroutine touches the accumulators, so no locking is needed.
type StatisticsCollector struct {
	Type      string
	BatchSize int
	StopChan  chan struct{}
	Frequency time.Duration
	StatsChan chan model.EventStatistics
	Emitter   emitter.Emitter
	Pop       string
	// MaxOpenEntries bounds open consented entries across both tiers; zero means the default.
	MaxOpenEntries int
	// Now places events into their bucket; tests inject a clock.
	Now func() time.Time

	fleet   *fleetAccumulator
	quarter *consentedAccumulator
	hour    *consentedAccumulator
	// flushDeadline, when set, bounds the total time consented chunks may take.
	flushDeadline time.Time
}

const (
	// DefaultMaxOpenEntries is used when MaxOpenEntries is unset.
	DefaultMaxOpenEntries = config.DefaultStatisticsMaxOpenEntries
	// StatisticsEmitChunkSize is the most consented documents handed to the emitter per call.
	StatisticsEmitChunkSize = 5000
)

func (c *StatisticsCollector) Collect() error {
	ticker := time.NewTicker(c.Frequency)
	defer ticker.Stop()
	events := c.StatsChan
	for {
		select {
		case event, ok := <-events:
			if !ok {
				log.Debug().Msg("Channel closed; waiting for stop")
				events = nil
				continue
			}
			c.add(event)
			if c.fleet.pending() >= c.BatchSize {
				c.flush("batch_size")
			}
		case <-ticker.C:
			c.tick()
		case <-c.StopChan:
			c.stop()
			return nil
		}
	}
}

func (c *StatisticsCollector) GetChannel() channel.CollectorChannel {
	return channel.EventStatisticsChannel{Channel: c.StatsChan}
}

func (c *StatisticsCollector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// stop adds the queued events and flushes everything under the shutdown budget.
func (c *StatisticsCollector) stop() {
	log.Info().Msg("Stopping statistics collector")
	c.drain()
	c.flush("shutdown")
	c.flushDeadline = time.Now().Add(EmitTimeout)
	c.flushConsented(true)
}

// ensure creates the accumulators on first use; MaxOpenEntries is read here once.
func (c *StatisticsCollector) ensure() {
	if c.fleet == nil {
		c.fleet = newFleetAccumulator(c.Pop)
		c.quarter = newConsentedAccumulator(model.StatisticsTier15Min)
		c.hour = newConsentedAccumulator(model.StatisticsTier1Hour)
	}
}

// tick flushes pending fleet counters and the consented entries whose bucket has closed.
func (c *StatisticsCollector) tick() {
	c.ensure()
	if c.fleet.pending() == 0 {
		log.Trace().Msg("Postpone stats event emission")
	} else {
		c.flush("frequency")
	}
	c.flushConsented(false)
}

// drain adds the events already queued so the shutdown flush does not lose them. It skips the
// entry-cap flush: the final flush emits everything under the shutdown budget.
func (c *StatisticsCollector) drain() {
	for {
		select {
		case event, ok := <-c.StatsChan:
			if !ok {
				return
			}
			c.record(event)
		default:
			return
		}
	}
}

// add records one event and flushes the consented entries when they reach the cap.
func (c *StatisticsCollector) add(event model.EventStatistics) {
	if c.record(event) && c.quarter.size()+c.hour.size() >= c.maxOpenEntries() {
		c.flushConsented(true)
	}
}

// record counts one event in the fleet accumulator, and in the consented one when
// the event carries a consented part; it reports whether it did the latter.
func (c *StatisticsCollector) record(event model.EventStatistics) bool {
	c.ensure()
	now := c.now()
	c.fleet.add(event, now)
	if event.Consented == nil {
		return false
	}
	c.quarter.add(event, now)
	c.hour.add(event, now)
	return true
}

func (c *StatisticsCollector) maxOpenEntries() int {
	if c.MaxOpenEntries > 0 {
		return c.MaxOpenEntries
	}
	return DefaultMaxOpenEntries
}

// flush emits every open fleet document as an increment and resets. A failed
// emit is logged and the counters are dropped, as for query logs.
func (c *StatisticsCollector) flush(trigger string) {
	c.ensure()
	batch := c.fleet.take()
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), EmitTimeout)
	defer cancel()
	log.Info().Str("event_type", c.Type).Str("trigger", trigger).Int("events_number", len(batch)).Msg("Emitting stats events batch")
	if err := c.Emitter.EmitServiceStatistics(ctx, batch); err != nil {
		log.Error().Err(err).Msg("Failed to emit stats events")
	}
}

// flushConsented emits the closed-bucket entries of both tiers, or every entry when all
// is set. When a quarter closed, the open hour's counts so far go out too (Y28). Each
// hour entry also goes out as a 1-day measurement. A failed emit is logged without
// identifiers and the entries are dropped.
func (c *StatisticsCollector) flushConsented(all bool) {
	c.ensure()
	var quarters, hours []model.Statistics
	if all {
		quarters, hours = c.quarter.takeAll(), c.hour.takeAll()
	} else {
		now := c.now()
		quarters = c.quarter.takeClosed(now)
		if len(quarters) > 0 {
			hours = c.hour.takeCounts(now)
		} else {
			hours = c.hour.takeClosed(now)
		}
	}
	batch := make([]model.Statistics, 0, len(quarters)+2*len(hours))
	batch = append(batch, quarters...)
	for _, h := range hours {
		batch = append(batch, h, h.ForDay())
	}
	if len(batch) > 0 {
		c.emitConsented(batch)
	}
}

// emitConsented hands the batch to the emitter in chunks, each with its own
// timeout, so one slow write loses one chunk. Past flushDeadline the rest is dropped.
func (c *StatisticsCollector) emitConsented(batch []model.Statistics) {
	log.Info().Str("event_type", c.Type).Int("events_number", len(batch)).Msg("Emitting consented stats batch")
	for start := 0; start < len(batch); start += StatisticsEmitChunkSize {
		end := min(start+StatisticsEmitChunkSize, len(batch))
		timeout := EmitTimeout
		if !c.flushDeadline.IsZero() {
			remaining := time.Until(c.flushDeadline)
			if remaining <= 0 {
				log.Error().Int("dropped", len(batch)-start).Msg("Shutdown flush budget spent; dropping consented stats")
				return
			}
			timeout = min(timeout, remaining)
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := c.Emitter.EmitStatistics(ctx, batch[start:end])
		cancel()
		if err != nil {
			log.Error().Err(err).Int("dropped", end-start).Msg("Failed to emit consented stats")
		}
	}
}
