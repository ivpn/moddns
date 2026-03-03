package collector

import (
	"context"
	"sync"
	"time"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog/log"
)

type StatisticsCollector struct {
	Type            string
	BatchSize       int
	StopChan        chan struct{}
	Frequency       time.Duration
	StatsChan       chan model.EventStatistics
	Emitter         emitter.Emitter
	mu              sync.Mutex
	statsAggregated sync.Map
	statsToEmit     []model.EventStatistics
}

func (c *StatisticsCollector) Collect() error {
	ticker := time.NewTicker(c.Frequency)
	counter := 0
	ctx := context.Background()
	for {
		select {
		case statsEvent, ok := <-c.StatsChan:
			if !ok {
				log.Debug().Msg("Channel closed or empty")
				continue
			}

			c.mu.Lock()
			stats, ok := c.statsAggregated.Load(statsEvent.Statistics.ProfileID)
			if ok {
				stats.(*model.Statistics).Aggregate(statsEvent.Statistics)
				c.statsAggregated.Store(statsEvent.Statistics.ProfileID, stats)
			} else {
				c.statsAggregated.Store(statsEvent.Statistics.ProfileID, statsEvent.Statistics)
			}
			counter++
			c.mu.Unlock()

			if counter == c.BatchSize {
				timeoutCtx, cancel := context.WithTimeout(ctx, EmitTimeout)
				c.statsAggregated.Range(func(key, value interface{}) bool {
					stats, _ := value.(*model.Statistics)
					c.statsToEmit = append(c.statsToEmit, model.EventStatistics{
						Statistics: stats,
					})
					return true
				})
				log.Info().Str("event_type", c.Type).Str("trigger", "batch_size").Int("events_number", len(c.statsToEmit)).Msg("Emitting stats events batch")
				if err := c.Emitter.EmitStatistics(timeoutCtx, c.statsToEmit); err != nil {
					log.Error().Err(err).Msg("Failed to emit stats events")
				}
				cancel()

				// reset for next batch
				c.mu.Lock()
				c.statsToEmit = make([]model.EventStatistics, 0)
				c.statsAggregated.Clear()
				counter = 0
				c.mu.Unlock()
			}
		case <-ticker.C:
			c.mu.Lock()
			c.statsAggregated.Range(func(key, value interface{}) bool {
				stats, _ := value.(*model.Statistics)
				c.statsToEmit = append(c.statsToEmit, model.EventStatistics{
					Statistics: stats,
				})
				return true
			})
			if len(c.statsToEmit) > 0 {
				timeoutCtx, cancel := context.WithTimeout(ctx, EmitTimeout)
				log.Info().Str("event_type", c.Type).Int("events_number", len(c.statsToEmit)).Str("trigger", "frequency").Msg("Emitting stats events batch")
				if err := c.Emitter.EmitStatistics(timeoutCtx, c.statsToEmit); err != nil {
					log.Error().Err(err).Msg("Failed to emit events")
				}
				cancel()

				c.statsToEmit = make([]model.EventStatistics, 0)
				c.statsAggregated.Clear()
				counter = 0
			}
			c.mu.Unlock()
			log.Trace().Msg("Postpone stats event emission")
		case <-c.StopChan:
			log.Info().Msg("Stopping statistics collector")
			ticker.Stop()
			return nil
		}
	}
}

func (c *StatisticsCollector) GetChannel() channel.CollectorChannel {
	return channel.EventStatisticsChannel{Channel: c.StatsChan}
}
