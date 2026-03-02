package metrics

import (
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/prometheus/client_golang/prometheus"
)

// RateLimitMetrics implements ratelimit.Metrics using Prometheus counters.
type RateLimitMetrics struct {
	rejected *prometheus.CounterVec
}

var _ ratelimit.Metrics = (*RateLimitMetrics)(nil)

// NewRateLimitMetrics creates and registers the dns_ratelimited_total counter.
func NewRateLimitMetrics(reg prometheus.Registerer) *RateLimitMetrics {
	m := &RateLimitMetrics{
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dns_ratelimited_total",
			Help: "Total number of DNS queries rejected by the rate limiter.",
		}, []string{"layer", "proto"}),
	}
	reg.MustRegister(m.rejected)
	return m
}

func (m *RateLimitMetrics) RecordRejection(layer, proto string) {
	m.rejected.WithLabelValues(layer, proto).Inc()
}
