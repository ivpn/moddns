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

const (
	EmitTimeout = 5 * time.Second
)

type QueryLogsCollector struct {
	Type      string
	BatchSize int
	StopChan  chan struct{}
	Frequency time.Duration
	QueryChan chan model.EventQueryLog
	Emitter   emitter.Emitter
}

func (c *QueryLogsCollector) Collect() error {
	ticker := time.NewTicker(c.Frequency)
	counter := 0
	var queryLogsToEmit []model.EventQueryLog
	var mu sync.Mutex
	ctx := context.Background()
	for {
		select {
		case queryLogEvent, ok := <-c.QueryChan:
			if !ok {
				log.Debug().Msg("Channel closed or empty")
				continue
			}
			mu.Lock()
			queryLogsToEmit = append(queryLogsToEmit, queryLogEvent)
			counter++
			mu.Unlock()

			if counter == c.BatchSize {
				log.Info().Str("event_type", c.Type).Str("trigger", "batch_size").Int("events_number", len(queryLogsToEmit)).Msg("Emitting events batch")
				timeoutCtx, cancel := context.WithTimeout(ctx, EmitTimeout)
				if err := c.Emitter.EmitQueryLogs(timeoutCtx, queryLogsToEmit); err != nil {
					log.Error().Err(err).Msg("Failed to emit events")
				}
				cancel()

				// reset for next batch
				mu.Lock()
				queryLogsToEmit = make([]model.EventQueryLog, 0)
				counter = 0
				mu.Unlock()
			}
		case <-ticker.C:
			mu.Lock()
			if len(queryLogsToEmit) > 0 {
				timeoutCtx, cancel := context.WithTimeout(ctx, EmitTimeout)
				if err := c.Emitter.EmitQueryLogs(timeoutCtx, queryLogsToEmit); err != nil {
					log.Error().Err(err).Msg("Failed to emit events")
				}
				cancel()
				log.Info().Str("event_type", c.Type).Str("trigger", "frequency").Int("events_number", len(queryLogsToEmit)).Msg("Emitting events batch")
				queryLogsToEmit = make([]model.EventQueryLog, 0)
				counter = 0
			}
			mu.Unlock()
			log.Trace().Msg("Postpone event emission")
		case <-c.StopChan:
			log.Info().Msg("Stopping query logs collector")
			ticker.Stop()
			return nil
		}
	}
}

func (c *QueryLogsCollector) GetChannel() channel.CollectorChannel {
	return channel.EventQueryLogChannel{Channel: c.QueryChan}
}
