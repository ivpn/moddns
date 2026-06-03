// Package metrics provides Prometheus instrumentation for the blocklists
// updater, mirroring the pattern used by the proxy service
// (proxy/internal/metrics). The service is a singleton writer to the shared
// Redis/Mongo data the proxy reads on the DNS hot path, so update failures must
// be observable: the blocklists_last_success_timestamp_seconds gauge in
// particular lets an alert fire when a source goes stale.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Status label values for blocklists_update_total.
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// Reason label values for blocklists_validation_rejected_total.
const (
	ReasonEmpty     = "empty"
	ReasonShrink    = "shrink"
	ReasonScanError = "scan_error"
	ReasonTruncated = "truncated"
)

// Updates is the instrumentation surface for the blocklist update pipeline.
// The concrete Prometheus implementation lives here; a noop implementation is
// used when metrics are disabled, keeping Prometheus an optional dependency of
// the call sites.
type Updates interface {
	// RecordUpdate counts an update attempt by source and status (success|failure).
	RecordUpdate(source, status string)
	// RecordDuration observes end-to-end update duration for a source.
	RecordDuration(source string, d time.Duration)
	// SetDomainsExtracted records the number of validated domains published for a source.
	SetDomainsExtracted(source string, n int)
	// SetLastSuccess records the wall-clock time of the last successful swap for a source.
	SetLastSuccess(source string, ts time.Time)
	// RecordDownloadBytes records the number of bytes downloaded for a source.
	RecordDownloadBytes(source string, n int64)
	// RecordValidationRejected counts a rejected swap by source and reason.
	RecordValidationRejected(source, reason string)
}

// NoopUpdates is a no-op Updates implementation used when metrics are disabled.
type NoopUpdates struct{}

func (NoopUpdates) RecordUpdate(string, string)              {}
func (NoopUpdates) RecordDuration(string, time.Duration)     {}
func (NoopUpdates) SetDomainsExtracted(string, int)          {}
func (NoopUpdates) SetLastSuccess(string, time.Time)         {}
func (NoopUpdates) RecordDownloadBytes(string, int64)        {}
func (NoopUpdates) RecordValidationRejected(string, string)  {}

// PromUpdates implements Updates using Prometheus collectors.
type PromUpdates struct {
	updates           *prometheus.CounterVec
	updateDuration    *prometheus.HistogramVec
	domainsExtracted  *prometheus.GaugeVec
	lastSuccess       *prometheus.GaugeVec
	downloadBytes     *prometheus.GaugeVec
	validationRejects *prometheus.CounterVec
}

// NewPromUpdates creates and registers all blocklist update collectors.
func NewPromUpdates(reg prometheus.Registerer) *PromUpdates {
	m := &PromUpdates{
		updates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blocklists_update_total",
			Help: "Total number of blocklist update attempts by source and status.",
		}, []string{"source", "status"}),
		updateDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "blocklists_update_duration_seconds",
			Help:    "End-to-end blocklist update duration in seconds by source.",
			Buckets: prometheus.DefBuckets,
		}, []string{"source"}),
		domainsExtracted: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "blocklists_domains_extracted",
			Help: "Number of validated domains published in the last update by source.",
		}, []string{"source"}),
		lastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "blocklists_last_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful blocklist update by source.",
		}, []string{"source"}),
		downloadBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "blocklists_download_bytes",
			Help: "Number of bytes downloaded in the last update by source.",
		}, []string{"source"}),
		validationRejects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blocklists_validation_rejected_total",
			Help: "Total number of blocklist swaps rejected by validation, by source and reason.",
		}, []string{"source", "reason"}),
	}
	reg.MustRegister(
		m.updates,
		m.updateDuration,
		m.domainsExtracted,
		m.lastSuccess,
		m.downloadBytes,
		m.validationRejects,
	)
	return m
}

func (m *PromUpdates) RecordUpdate(source, status string) {
	m.updates.WithLabelValues(source, status).Inc()
}

func (m *PromUpdates) RecordDuration(source string, d time.Duration) {
	m.updateDuration.WithLabelValues(source).Observe(d.Seconds())
}

func (m *PromUpdates) SetDomainsExtracted(source string, n int) {
	m.domainsExtracted.WithLabelValues(source).Set(float64(n))
}

func (m *PromUpdates) SetLastSuccess(source string, ts time.Time) {
	m.lastSuccess.WithLabelValues(source).Set(float64(ts.Unix()))
}

func (m *PromUpdates) RecordDownloadBytes(source string, n int64) {
	m.downloadBytes.WithLabelValues(source).Set(float64(n))
}

func (m *PromUpdates) RecordValidationRejected(source, reason string) {
	m.validationRejects.WithLabelValues(source, reason).Inc()
}
