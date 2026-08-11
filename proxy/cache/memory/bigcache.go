package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/rs/zerolog/log"
)

const (
	// Must stay strictly greater than dnsproxy's upstream timeout (defaultTimeout,
	// 10s): a slow upstream otherwise races the entry's expiry and the response
	// handler loses its request context.
	ProfileIdExpirationTime = 15 * time.Second
	StatsLoggingInterval    = 10 * time.Minute
)

// ProfileIDCache is an in-app cache implementation using BigCache
type ProfileIDCache struct {
	cache *bigcache.BigCache
}

// NewBigcache creates a new ProfileIDCache instance
func NewBigcache(cacheCfg *cache.Config) (*ProfileIDCache, error) {
	profilesCache := &ProfileIDCache{}
	// TODO: investigate bigcache configuration
	cache, err := bigcache.New(context.Background(), bigcache.DefaultConfig(ProfileIdExpirationTime))
	if err != nil {
		return nil, err
	}
	profilesCache.cache = cache
	return profilesCache, nil
}

// SetRequestCtx sets request data in cache
func (c *ProfileIDCache) SetRequestCtx(requestId string, reqCtx *requestcontext.RequestContext) error {
	reqKey := "request:" + requestId
	data, err := json.Marshal(reqCtx)
	if err != nil {
		return fmt.Errorf("failed to marshal RequestContext: %w", err)
	}

	if err := c.cache.Set(reqKey, data); err != nil {
		return err
	}
	return nil
}

func (c *ProfileIDCache) GetRequestCtx(requestId string) (*requestcontext.RequestContext, error) {
	reqKey := "request:" + requestId
	entry, err := c.cache.Get(reqKey)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return nil, ErrRequestCtxNotFound
		}
		return nil, err
	}
	var reqCtx requestcontext.RequestContext
	if err := json.Unmarshal(entry, &reqCtx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal RequestContext: %w", err)
	}

	// Recreate the logger from the stored configuration
	if reqCtx.Logger == nil {
		reqCtx.Logger = logging.NewContextLogger(reqCtx.LoggerConfig)
	}

	return &reqCtx, nil
}

func (c *ProfileIDCache) Stats() {
	stats := c.cache.Stats()
	entries := c.cache.Len()
	log.Info().
		Int("entries", entries).
		Int64("collisions", stats.Collisions).
		Int64("del_hits", stats.DelHits).
		Int64("del_misses", stats.DelMisses).
		Int64("hits", stats.Hits).
		Int64("misses", stats.Misses).
		Msg("BigCache stats")
}
