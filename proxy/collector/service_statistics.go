package collector

import (
	"context"
	"sort"
	"time"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
)

// ServiceStatisticsCollector sums per-query counter events into one
// service-wide document per hour for this PoP and hands the open documents to
// the emitter on every flush. Only the Collect goroutine touches the
// accumulator, so no locking is needed.
type ServiceStatisticsCollector struct {
	Type      string
	BatchSize int
	StopChan  chan struct{}
	Frequency time.Duration
	StatsChan chan model.EventStatistics
	Emitter   emitter.Emitter
	Pop       string
	// Now places events into their hour; tests inject a clock.
	Now func() time.Time

	buckets map[time.Time]*model.ServiceStatistics
	counter int
}

func (c *ServiceStatisticsCollector) Collect() error {
	ticker := time.NewTicker(c.Frequency)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-c.StatsChan:
			if !ok {
				log.Debug().Msg("Channel closed or empty")
				continue
			}
			c.add(event)
			if c.counter >= c.BatchSize {
				c.flush("batch_size")
			}
		case <-ticker.C:
			if c.counter == 0 {
				log.Trace().Msg("Postpone stats event emission")
				continue
			}
			c.flush("frequency")
		case <-c.StopChan:
			log.Info().Msg("Stopping statistics collector")
			return nil
		}
	}
}

func (c *ServiceStatisticsCollector) GetChannel() channel.CollectorChannel {
	return channel.EventStatisticsChannel{Channel: c.StatsChan}
}

func (c *ServiceStatisticsCollector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// add sums one event into the document for the hour that is open now.
func (c *ServiceStatisticsCollector) add(event model.EventStatistics) {
	if c.buckets == nil {
		c.buckets = make(map[time.Time]*model.ServiceStatistics)
	}
	now := c.now()
	bucket := model.BucketStart(now)
	doc, ok := c.buckets[bucket]
	if !ok {
		doc = model.NewServiceStatistics(c.Pop, now)
		c.buckets[bucket] = doc
	}
	doc.Aggregate(event)
	c.counter++
}

// flush emits every open document as an increment and resets. A failed emit
// is logged and the counters are dropped, as for query logs.
func (c *ServiceStatisticsCollector) flush(trigger string) {
	if len(c.buckets) == 0 {
		return
	}
	batch := make([]model.ServiceStatistics, 0, len(c.buckets))
	for _, doc := range c.buckets {
		batch = append(batch, *doc)
	}
	sort.Slice(batch, func(i, j int) bool { return batch[i].Timestamp.Before(batch[j].Timestamp) })

	ctx, cancel := context.WithTimeout(context.Background(), EmitTimeout)
	defer cancel()
	log.Info().Str("event_type", c.Type).Str("trigger", trigger).Int("events_number", len(batch)).Msg("Emitting stats events batch")
	if err := c.Emitter.EmitServiceStatistics(ctx, batch); err != nil {
		log.Error().Err(err).Msg("Failed to emit stats events")
	}

	c.buckets = make(map[time.Time]*model.ServiceStatistics)
	c.counter = 0
}
