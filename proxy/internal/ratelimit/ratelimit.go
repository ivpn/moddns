package ratelimit

import (
	"net/netip"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Config holds rate limiter settings.
type Config struct {
	PerIPEnabled      bool
	PerIPRate         int
	PerIPBurst        int
	PerProfileEnabled bool
	PerProfileRate    int
	PerProfileBurst   int
}

// RateLimiter enforces per-IP and per-profile rate limits using token buckets.
type RateLimiter struct {
	cfg            Config
	ipBuckets      *gocache.Cache
	profileBuckets *gocache.Cache
	metrics        Metrics
	sampledLogger  zerolog.Logger
}

const (
	bucketExpiry  = 1 * time.Hour
	bucketCleanup = 5 * time.Minute
	layerIP       = "ip"
	layerProfile  = "profile"
)

// New creates a RateLimiter. Pass nil for m to disable metrics recording.
func New(cfg Config, m Metrics) *RateLimiter {
	if m == nil {
		m = noopMetrics{}
	}
	return &RateLimiter{
		cfg:            cfg,
		ipBuckets:      gocache.New(bucketExpiry, bucketCleanup),
		profileBuckets: gocache.New(bucketExpiry, bucketCleanup),
		metrics:        m,
		sampledLogger: log.Logger.Sample(&zerolog.BurstSampler{
			Burst:       5,
			Period:      10 * time.Second,
			NextSampler: &zerolog.BasicSampler{N: 100},
		}),
	}
}

// CheckIP returns true if the query from addr should be allowed (Layer 1).
func (rl *RateLimiter) CheckIP(addr netip.Addr, proto string) bool {
	if !rl.cfg.PerIPEnabled {
		return true
	}
	return rl.check(rl.ipBuckets, addr.String(), rl.cfg.PerIPRate, rl.cfg.PerIPBurst, layerIP, proto)
}

// CheckProfile returns true if the query for profileID should be allowed (Layer 2).
func (rl *RateLimiter) CheckProfile(profileID string, proto string) bool {
	if !rl.cfg.PerProfileEnabled {
		return true
	}
	return rl.check(rl.profileBuckets, profileID, rl.cfg.PerProfileRate, rl.cfg.PerProfileBurst, layerProfile, proto)
}

func (rl *RateLimiter) check(store *gocache.Cache, key string, rps, burst int, layer, proto string) bool {
	v, found := store.Get(key)
	if found {
		limiter := v.(*rate.Limiter)
		if limiter.Allow() {
			return true
		}
		rl.metrics.RecordRejection(layer, proto)
		rl.sampledLogger.Warn().Str("layer", layer).Str("key", key).Str("proto", proto).Msg("rate limited")
		return false
	}

	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	limiter.Allow() // consume first token
	store.Set(key, limiter, gocache.DefaultExpiration)
	return true
}
