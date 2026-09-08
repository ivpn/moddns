package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Label values for proxy_dns_filter_stage_errors_total raised before filtering
// starts; the filter phases and stage names are owned by the filter package.
const (
	PhaseAdmission       = "admission"
	StageProfileSettings = "profile_settings"
)

// Status label values for proxy_dns_profile_settings_cache_total.
const (
	CacheLookupHit         = "hit"
	CacheLookupMiss        = "miss"
	CacheLookupStale       = "stale"
	CacheLookupUnavailable = "unavailable"
)

// ServerMetrics implements server.Metrics using Prometheus collectors.
type ServerMetrics struct {
	queries              *prometheus.CounterVec
	profileCacheLookups  *prometheus.CounterVec
	queryDuration        *prometheus.HistogramVec
	domainFilterDuration *prometheus.HistogramVec
	ipFilterDuration     *prometheus.HistogramVec
	upstreamDuration     *prometheus.HistogramVec
	blocked              *prometheus.CounterVec
	filterStageErrors    *prometheus.CounterVec
}

// NewServerMetrics creates and registers all server-level Prometheus collectors.
func NewServerMetrics(reg prometheus.Registerer) *ServerMetrics {
	m := &ServerMetrics{
		queries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_dns_queries_total",
			Help: "Total number of DNS queries received by the proxy.",
		}, []string{"proto"}),
		profileCacheLookups: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_dns_profile_settings_cache_total",
			Help: "Profile settings lookups by outcome: hit, miss, stale (last-known-good served), unavailable.",
		}, []string{"status"}),
		queryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "proxy_dns_query_duration_seconds",
			Help:    "End-to-end DNS query duration in the proxy in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"proto"}),
		domainFilterDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "proxy_dns_domain_filter_duration_seconds",
			Help:    "Domain filter execution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"proto"}),
		ipFilterDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "proxy_dns_ip_filter_duration_seconds",
			Help:    "IP filter execution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"proto"}),
		upstreamDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "proxy_dns_upstream_duration_seconds",
			Help:    "Upstream DNS resolution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"upstream"}),
		blocked: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_dns_blocked_total",
			Help: "Total blocked DNS queries by filter phase.",
		}, []string{"phase"}),
		filterStageErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_dns_filter_stage_errors_total",
			Help: "Settings-store read failures by pipeline phase and stage; each one answers SERVFAIL.",
		}, []string{"phase", "stage"}),
	}
	reg.MustRegister(
		m.queries,
		m.profileCacheLookups,
		m.queryDuration,
		m.domainFilterDuration,
		m.ipFilterDuration,
		m.upstreamDuration,
		m.blocked,
		m.filterStageErrors,
	)
	return m
}

func (m *ServerMetrics) RecordQuery(proto string) {
	m.queries.WithLabelValues(proto).Inc()
}

func (m *ServerMetrics) RecordProfileCacheLookup(status string) {
	m.profileCacheLookups.WithLabelValues(status).Inc()
}

func (m *ServerMetrics) RecordQueryDuration(proto string, d time.Duration) {
	m.queryDuration.WithLabelValues(proto).Observe(d.Seconds())
}

func (m *ServerMetrics) RecordDomainFilterDuration(proto string, d time.Duration) {
	m.domainFilterDuration.WithLabelValues(proto).Observe(d.Seconds())
}

func (m *ServerMetrics) RecordIPFilterDuration(proto string, d time.Duration) {
	m.ipFilterDuration.WithLabelValues(proto).Observe(d.Seconds())
}

func (m *ServerMetrics) RecordUpstreamDuration(upstream string, d time.Duration) {
	m.upstreamDuration.WithLabelValues(upstream).Observe(d.Seconds())
}

func (m *ServerMetrics) RecordBlocked(phase string) {
	m.blocked.WithLabelValues(phase).Inc()
}

func (m *ServerMetrics) RecordFilterStageError(phase, stage string) {
	m.filterStageErrors.WithLabelValues(phase, stage).Inc()
}
