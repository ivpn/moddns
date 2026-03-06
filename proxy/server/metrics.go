package server

import "time"

// Metrics records server-level events. Implementations live outside this package
// (e.g. in metrics/) so that server has no dependency on any specific
// telemetry library.
type Metrics interface {
	RecordQuery(proto string)
	RecordProfileCacheLookup(hit bool)
	RecordQueryDuration(proto string, d time.Duration)
	RecordDomainFilterDuration(proto string, d time.Duration)
	RecordIPFilterDuration(proto string, d time.Duration)
	RecordUpstreamDuration(upstream string, d time.Duration)
	RecordBlocked(phase string)
}

type noopMetrics struct{}

func (noopMetrics) RecordQuery(string)                            {}
func (noopMetrics) RecordProfileCacheLookup(bool)                 {}
func (noopMetrics) RecordQueryDuration(string, time.Duration)     {}
func (noopMetrics) RecordDomainFilterDuration(string, time.Duration) {}
func (noopMetrics) RecordIPFilterDuration(string, time.Duration)  {}
func (noopMetrics) RecordUpstreamDuration(string, time.Duration)  {}
func (noopMetrics) RecordBlocked(string)                          {}
