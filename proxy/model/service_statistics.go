package model

import "time"

// ServiceStatisticsBucket is the width of one service_statistics document.
const ServiceStatisticsBucket = time.Hour

// ServiceStatistics is one PoP's query counters for one hour. The type has no
// profile or device field on purpose: counters are summed across every
// profile in memory before anything is written, and every proxy instance in
// a PoP adds into the same document.
type ServiceStatistics struct {
	// ID is "<pop>:<hour>", e.g. "ams1:2026-09-17T13", so concurrent writers
	// converge on one document per PoP and hour.
	ID string `json:"-" bson:"_id"`
	// Timestamp is the start of the hour the counters belong to.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
	Pop       string    `json:"pop" bson:"pop"`
	Queries   Queries   `json:"queries" bson:"queries"`
}

// NewServiceStatistics opens an empty document for pop and the bucket
// containing t.
func NewServiceStatistics(pop string, t time.Time) *ServiceStatistics {
	bucket := BucketStart(t)
	return &ServiceStatistics{
		ID:        pop + ":" + bucket.Format("2006-01-02T15"),
		Timestamp: bucket,
		Pop:       pop,
	}
}

// Aggregate adds one event's counters; the bucket is untouched.
func (s *ServiceStatistics) Aggregate(event EventStatistics) {
	s.Queries.Add(event.Queries)
}

// BucketStart returns the UTC start of the bucket containing t.
func BucketStart(t time.Time) time.Time {
	return t.UTC().Truncate(ServiceStatisticsBucket)
}

type Queries struct {
	Total   int `json:"total" bson:"total"`
	Blocked int `json:"blocked" bson:"blocked"`
	DNSSEC  int `json:"dnssec" bson:"dnssec"`
}

// Add sums other into q.
func (q *Queries) Add(other Queries) {
	q.Total += other.Total
	q.Blocked += other.Blocked
	q.DNSSEC += other.DNSSEC
}
