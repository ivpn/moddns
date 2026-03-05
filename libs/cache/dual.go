package cache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	healthCheckInterval  = 3 * time.Second
	healthCheckThreshold = 3 // consecutive failures before swap
	healthCheckTimeout   = 2 * time.Second
)

// DualClient manages a primary and fallback Redis client with automatic
// failover. It periodically health-checks the primary; after consecutive
// failures it swaps reads to the fallback, and swaps back when the primary
// recovers.
//
// When no fallback is configured (no sentinel), it behaves as a thin
// wrapper around a single direct client with no health check overhead.
type DualClient struct {
	primary  *redis.Client
	fallback *redis.Client // nil if no sentinel config
	active   atomic.Pointer[redis.Client]
	quit     chan struct{}
}

// NewDualClient creates a Redis client pair from the given config.
//
// If both Address and sentinel config (MasterName + FailoverAddresses) are
// provided, it creates a primary (direct) and fallback (sentinel) client
// with an active health check. Otherwise it creates a single direct client.
func NewDualClient(cfg *Config) (*DualClient, error) {
	primary, err := NewDirectClient(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()
	if _, err := primary.Ping(ctx).Result(); err != nil {
		primary.Close()
		return nil, err
	}

	dc := &DualClient{primary: primary}
	dc.active.Store(primary)

	if cfg.MasterName != "" && len(cfg.FailoverAddresses) > 0 {
		fallback, err := NewFailoverClient(cfg)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create fallback Redis client, running without failover")
		} else {
			fCtx, fCancel := context.WithTimeout(context.Background(), redisPingTimeout)
			defer fCancel()
			if _, fErr := fallback.Ping(fCtx).Result(); fErr != nil {
				log.Warn().Err(fErr).Msg("Fallback Redis ping failed, running without failover")
				fallback.Close()
			} else {
				dc.fallback = fallback
				dc.quit = make(chan struct{})
				go dc.healthCheck()
				log.Info().Msg("Redis dual-client mode: primary=direct, fallback=sentinel")
			}
		}
	}

	log.Info().Msg("Connected to Redis")
	return dc, nil
}

// NewSingleClient wraps an existing *redis.Client in a DualClient with no
// fallback or health check. Useful for tests that manage their own client.
func NewSingleClient(client *redis.Client) *DualClient {
	dc := &DualClient{primary: client}
	dc.active.Store(client)
	return dc
}

// Client returns the currently active Redis client.
func (dc *DualClient) Client() *redis.Client {
	return dc.active.Load()
}

// Close stops the health check goroutine and closes both clients.
func (dc *DualClient) Close() {
	if dc.quit != nil {
		close(dc.quit)
	}
	dc.primary.Close()
	if dc.fallback != nil {
		dc.fallback.Close()
	}
}

func (dc *DualClient) healthCheck() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			err := dc.primary.Ping(ctx).Err()
			cancel()

			if err != nil {
				failures++
				if failures >= healthCheckThreshold && dc.active.Load() != dc.fallback {
					log.Warn().Err(err).Int("failures", failures).
						Msg("Primary Redis unreachable, switching to fallback")
					dc.active.Store(dc.fallback)
				}
			} else {
				if failures >= healthCheckThreshold && dc.active.Load() != dc.primary {
					log.Info().Msg("Primary Redis recovered, switching back from fallback")
					dc.active.Store(dc.primary)
				}
				failures = 0
			}
		case <-dc.quit:
			return
		}
	}
}
