package server

import "time"

// Metrics records server-level events. Implementations live outside this package
// (e.g. in metrics/) so that server has no dependency on any specific
// telemetry library.
type Metrics interface {
	RecordQuery(proto string)
	// RecordProfileCacheLookup counts one settings lookup by outcome:
	// hit, miss (fetched), stale (last-known-good served) or unavailable.
	RecordProfileCacheLookup(status string)
	RecordQueryDuration(proto string, d time.Duration)
	RecordDomainFilterDuration(proto string, d time.Duration)
	RecordIPFilterDuration(proto string, d time.Duration)
	RecordUpstreamDuration(upstream string, d time.Duration)
	RecordBlocked(phase string)
	// RecordFilterStageError counts one failed settings-store read. phase is
	// "admission", "domain" or "ip"; stage names the reader that failed.
	RecordFilterStageError(phase, stage string)
	// RecordDoHRequest counts one DoH request by the HTTP version the proxy
	// received it over ("1.0"/"1.1", "2" or "3"); behind nginx that is nginx's hop.
	RecordDoHRequest(httpVersion string)
	doh3Metrics
}

// doh3Metrics counts what the HTTP/3 listener turns away.
type doh3Metrics interface {
	// RecordDoH3ConnRejected counts a refused connection; reason is one of
	// the doh3Reject* values.
	RecordDoH3ConnRejected(reason string)
	// RecordDoH3Busy counts a request answered 503 for want of a handler slot.
	RecordDoH3Busy()
	// SetDoH3ListenerUp reports whether the listener is serving.
	SetDoH3ListenerUp(up bool)
}

type noopMetrics struct{}

func (noopMetrics) RecordQuery(string)                               {}
func (noopMetrics) RecordProfileCacheLookup(string)                  {}
func (noopMetrics) RecordQueryDuration(string, time.Duration)        {}
func (noopMetrics) RecordDomainFilterDuration(string, time.Duration) {}
func (noopMetrics) RecordIPFilterDuration(string, time.Duration)     {}
func (noopMetrics) RecordUpstreamDuration(string, time.Duration)     {}
func (noopMetrics) RecordBlocked(string)                             {}
func (noopMetrics) RecordFilterStageError(string, string)            {}
func (noopMetrics) RecordDoHRequest(string)                          {}
func (noopMetrics) RecordDoH3ConnRejected(string)                    {}
func (noopMetrics) RecordDoH3Busy()                                  {}
func (noopMetrics) SetDoH3ListenerUp(bool)                           {}
