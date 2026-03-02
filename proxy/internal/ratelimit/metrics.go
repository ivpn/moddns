package ratelimit

// Metrics records rate-limiter events. Implementations live outside this package
// (e.g. in metrics/) so that ratelimit has no dependency on any specific
// telemetry library.
type Metrics interface {
	RecordRejection(layer, proto string)
}

type noopMetrics struct{}

func (noopMetrics) RecordRejection(string, string) {}
