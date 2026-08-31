package ratelimit

import (
	"net/netip"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
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
	// MaxBuckets caps each bucket store; 0 or negative selects defaultMaxBuckets.
	MaxBuckets int
	// IPv6PrefixLen is the prefix length IPv6 client addresses are grouped by;
	// values outside 1..128 select defaultIPv6PrefixLen.
	IPv6PrefixLen int
}

// RateLimiter enforces per-IP and per-profile rate limits using token buckets.
type RateLimiter struct {
	cfg            Config
	ipBuckets      *expirable.LRU[string, *rate.Limiter]
	profileBuckets *expirable.LRU[string, *rate.Limiter]
	metrics        Metrics
	sampledLogger  zerolog.Logger
}

const (
	bucketExpiry = 1 * time.Hour
	layerIP      = "ip"
	layerProfile = "profile"

	defaultMaxBuckets = 100_000
	// RFC 4291 §2.5.4: /64 is the standard interface-identifier boundary, so
	// one subscriber allocation maps to one bucket.
	defaultIPv6PrefixLen = 64
)

// New creates a RateLimiter. Pass nil for m to disable metrics recording.
func New(cfg Config, m Metrics) *RateLimiter {
	if m == nil {
		m = noopMetrics{}
	}
	if cfg.MaxBuckets <= 0 {
		cfg.MaxBuckets = defaultMaxBuckets
	}
	if cfg.IPv6PrefixLen < 1 || cfg.IPv6PrefixLen > 128 {
		cfg.IPv6PrefixLen = defaultIPv6PrefixLen
	}
	return &RateLimiter{
		cfg:            cfg,
		ipBuckets:      expirable.NewLRU[string, *rate.Limiter](cfg.MaxBuckets, nil, bucketExpiry),
		profileBuckets: expirable.NewLRU[string, *rate.Limiter](cfg.MaxBuckets, nil, bucketExpiry),
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
	return rl.check(rl.ipBuckets, rl.ipKey(addr), rl.cfg.PerIPRate, rl.cfg.PerIPBurst, layerIP, proto)
}

// ipKey groups IPv6 clients by prefix: a subscriber typically holds an entire
// prefix delegation, so keying on the full address would give one client a
// bucket per address. 4-mapped-6 addresses are unmapped so they key as IPv4.
func (rl *RateLimiter) ipKey(addr netip.Addr) string {
	addr = addr.Unmap()
	if !addr.Is6() {
		return addr.String()
	}
	prefix, err := addr.Prefix(rl.cfg.IPv6PrefixLen)
	if err != nil {
		return addr.String()
	}
	return prefix.Addr().String()
}

// CheckProfile returns true if the query for profileID should be allowed (Layer 2).
func (rl *RateLimiter) CheckProfile(profileID string, proto string) bool {
	if !rl.cfg.PerProfileEnabled {
		return true
	}
	return rl.check(rl.profileBuckets, profileID, rl.cfg.PerProfileRate, rl.cfg.PerProfileBurst, layerProfile, proto)
}

func (rl *RateLimiter) check(store *expirable.LRU[string, *rate.Limiter], key string, rps, burst int, layer, proto string) bool {
	limiter, found := store.Get(key)
	if found {
		if limiter.Allow() {
			return true
		}
		rl.metrics.RecordRejection(layer, proto)
		// The key is a client IP or profile ID; Warn-level lines feed
		// telemetry, so log only the dimension. Per-client detail is
		// available via the rejection metrics and on-host debug logs.
		rl.sampledLogger.Warn().Str("layer", layer).Str("proto", proto).Msg("rate limited")
		return false
	}

	limiter = rate.NewLimiter(rate.Limit(rps), burst)
	limiter.Allow() // consume first token
	store.Add(key, limiter)
	return true
}
