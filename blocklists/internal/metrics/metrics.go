// Package metrics provides Prometheus instrumentation for the blocklists
// updater, mirroring the pattern used by the proxy service
// (proxy/internal/metrics). The service writes the shared Redis/Mongo data the
// proxy reads on the DNS hot path, so update failures must be observable: the
// blocklists_last_success_timestamp_seconds gauge in particular lets an alert
// fire when a source goes stale. With multiple coordinated instances, each
// gauge is per-process and only the tick winner advances it — staleness alerts
// must aggregate max by (source) across instances.
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

// Reason label values for blocklists_rules_skipped.
const (
	SkipRuleException = "exception"
	SkipRuleBadfilter = "badfilter"
	SkipRuleModifier  = "modifier"
	SkipRuleWildcard  = "wildcard"
	SkipRulePrefix    = "prefix"
	SkipRuleInvalid   = "invalid"
)

// Reason label values for blocklists_refresh_skipped_total.
const (
	// SkipReasonFresh: the source was refreshed recently (typically by a peer
	// instance), so no download was needed.
	SkipReasonFresh = "fresh"
	// SkipReasonLockHeld: another instance holds the source's lock and is
	// refreshing it right now.
	SkipReasonLockHeld = "lock_held"
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
	// SetDeclaredEntries records the entry count the source reports — the header
	// value when present, otherwise the service's own non-comment line count.
	// Compared against SetDomainsExtracted it is a divergence signal (a large
	// drop hints at a partial download or many malformed/duplicate lines).
	SetDeclaredEntries(source string, n int)
	// SetRulesSkipped records how many input rules the last published
	// conversion dropped for a source, by reason
	// (exception|badfilter|modifier|wildcard|prefix|invalid). The extractor
	// fails open — an unrecognized rule is dropped, never widened into a
	// block — so a jump in modifier/wildcard means the upstream list started
	// shipping syntax the extractor does not understand: an under-block worth
	// review, not a false-positive risk.
	SetRulesSkipped(source, reason string, n int)
	// SetLastSuccess records the wall-clock time of the last successful swap for a source.
	SetLastSuccess(source string, ts time.Time)
	// RecordDownloadBytes records the number of bytes downloaded for a source.
	RecordDownloadBytes(source string, n int64)
	// RecordValidationRejected counts a rejected swap by source and reason.
	RecordValidationRejected(source, reason string)
	// RecordRetry counts a download retry (transient network error, 429 or 5xx)
	// for a source. A rising rate signals a source server under strain or
	// throttling our requests.
	RecordRetry(source string)
	// RecordRefreshSkipped counts a refresh that ended without downloading,
	// by source and reason (fresh|lock_held). High rates are normal on a
	// multi-instance deployment — they show coordination working.
	RecordRefreshSkipped(source, reason string)
	// RecordLockError counts lock acquisitions that failed on a Redis error
	// (not contention). The affected tick is skipped, so a rising rate means
	// refreshes are silently not happening.
	RecordLockError(source string)
}

// NoopUpdates is a no-op Updates implementation used when metrics are disabled.
type NoopUpdates struct{}

func (NoopUpdates) RecordUpdate(string, string)             {}
func (NoopUpdates) RecordDuration(string, time.Duration)    {}
func (NoopUpdates) SetDomainsExtracted(string, int)         {}
func (NoopUpdates) SetDeclaredEntries(string, int)          {}
func (NoopUpdates) SetRulesSkipped(string, string, int)     {}
func (NoopUpdates) SetLastSuccess(string, time.Time)        {}
func (NoopUpdates) RecordDownloadBytes(string, int64)       {}
func (NoopUpdates) RecordValidationRejected(string, string) {}
func (NoopUpdates) RecordRetry(string)                      {}
func (NoopUpdates) RecordRefreshSkipped(string, string)     {}
func (NoopUpdates) RecordLockError(string)                  {}

// PromUpdates implements Updates using Prometheus collectors.
type PromUpdates struct {
	updates           *prometheus.CounterVec
	updateDuration    *prometheus.HistogramVec
	domainsExtracted  *prometheus.GaugeVec
	declaredEntries   *prometheus.GaugeVec
	rulesSkipped      *prometheus.GaugeVec
	lastSuccess       *prometheus.GaugeVec
	downloadBytes     *prometheus.GaugeVec
	validationRejects *prometheus.CounterVec
	downloadRetries   *prometheus.CounterVec
	refreshSkips      *prometheus.CounterVec
	lockErrors        *prometheus.CounterVec
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
		declaredEntries: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "blocklists_source_declared_entries",
			Help: "Entry count reported by the source (header value, or non-comment line count when no header is present) in the last update by source.",
		}, []string{"source"}),
		rulesSkipped: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "blocklists_rules_skipped",
			Help: "Number of input rules dropped during the last published conversion, by source and reason (exception|badfilter|modifier|wildcard|prefix|invalid). Rising modifier/wildcard counts signal upstream syntax the extractor does not understand (under-block, fail open).",
		}, []string{"source", "reason"}),
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
		downloadRetries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blocklists_download_retries_total",
			Help: "Total number of blocklist download retries (transient network error, 429 or 5xx) by source.",
		}, []string{"source"}),
		refreshSkips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blocklists_refresh_skipped_total",
			Help: "Total number of refreshes skipped without downloading, by source and reason (fresh: recently refreshed, typically by a peer instance; lock_held: another instance is refreshing right now).",
		}, []string{"source", "reason"}),
		lockErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blocklists_lock_errors_total",
			Help: "Total number of distributed-lock acquisitions failed on a Redis error (not contention), by source. The affected refresh is skipped.",
		}, []string{"source"}),
	}
	reg.MustRegister(
		m.updates,
		m.updateDuration,
		m.domainsExtracted,
		m.declaredEntries,
		m.rulesSkipped,
		m.lastSuccess,
		m.downloadBytes,
		m.validationRejects,
		m.downloadRetries,
		m.refreshSkips,
		m.lockErrors,
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

func (m *PromUpdates) SetDeclaredEntries(source string, n int) {
	m.declaredEntries.WithLabelValues(source).Set(float64(n))
}

func (m *PromUpdates) SetRulesSkipped(source, reason string, n int) {
	m.rulesSkipped.WithLabelValues(source, reason).Set(float64(n))
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

func (m *PromUpdates) RecordRetry(source string) {
	m.downloadRetries.WithLabelValues(source).Inc()
}

func (m *PromUpdates) RecordRefreshSkipped(source, reason string) {
	m.refreshSkips.WithLabelValues(source, reason).Inc()
}

func (m *PromUpdates) RecordLockError(source string) {
	m.lockErrors.WithLabelValues(source).Inc()
}
