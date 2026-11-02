// Package statistics holds the per-profile statistics values shared by the API
// and the proxy, so the setting the API stores and the collection the proxy
// writes to cannot drift.
package statistics

import "time"

// Retention is how long a profile's daily statistics are kept
// (api-endpoint-behaviour.md J48). It is not the query-logs retention.
type Retention string

const (
	Retention30d Retention = "30d"
	Retention90d Retention = "90d"
	Retention1y  Retention = "1y"

	// DefaultRetention is what an empty or unknown value reads as.
	DefaultRetention = Retention30d
)

// Retentions lists the allowed values, shortest first.
func Retentions() []Retention {
	return []Retention{Retention30d, Retention90d, Retention1y}
}

// Valid reports whether r is one of the allowed values.
func (r Retention) Valid() bool {
	switch r {
	case Retention30d, Retention90d, Retention1y:
		return true
	}
	return false
}

// OrDefault returns r, or DefaultRetention when r is not allowed.
func (r Retention) OrDefault() Retention {
	if r.Valid() {
		return r
	}
	return DefaultRetention
}

// Window is how far back the retention reaches; an invalid value reads as the default.
func (r Retention) Window() time.Duration {
	switch r.OrDefault() {
	case Retention90d:
		return 90 * 24 * time.Hour
	case Retention1y:
		return 365 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// MarshalBinary lets go-redis write the value; it has no fallback for named strings.
func (r Retention) MarshalBinary() ([]byte, error) {
	return []byte(r), nil
}
