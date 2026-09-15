package collector

import (
	"context"
	"time"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
)

// ServiceStatisticsCollector sums per-query counter events into one service-wide
// document per flush for this PoP. Only the Collect goroutine touches the
// accumulator, so no locking is needed.
type ServiceStatisticsCollector struct {
	Type      string
	BatchSize int
	StopChan  chan struct{}
	Frequency time.Duration
	StatsChan chan model.EventStatistics
	Emitter   emitter.Emitter
	Pop       string
	// Now stamps flushed documents; tests inject a clock.
	Now func() time.Time

	current *model.ServiceStatistics
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

// add sums one event into the document open for this flush.
func (c *ServiceStatisticsCollector) add(event model.EventStatistics) {
	if c.current == nil {
		c.current = &model.ServiceStatistics{Pop: c.Pop}
	}
	c.current.Aggregate(event)
	c.counter++
}

func (c *ServiceStatisticsCollector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// flush stamps the open document with the flush time, emits it and resets. A
// failed emit is logged and the counters are dropped, as for query logs.
func (c *ServiceStatisticsCollector) flush(trigger string) {
	if c.current == nil {
		return
	}
	c.current.Timestamp = c.now().UTC()
	batch := []model.ServiceStatistics{*c.current}

	ctx, cancel := context.WithTimeout(context.Background(), EmitTimeout)
	defer cancel()
	log.Info().Str("event_type", c.Type).Str("trigger", trigger).Int("events_number", len(batch)).Msg("Emitting stats events batch")
	if err := c.Emitter.EmitServiceStatistics(ctx, batch); err != nil {
		log.Error().Err(err).Msg("Failed to emit stats events")
	}

	c.current = nil
	c.counter = 0
}
