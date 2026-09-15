package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ServiceStatistics is one PoP's query counters for one collector flush,
// stamped when the flush happens. The type has no profile or device field on
// purpose: counters are summed across every profile in memory before anything
// is written.
type ServiceStatistics struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
	Pop       string             `json:"pop" bson:"pop"`
	Queries   Queries            `json:"queries" bson:"queries"`
}

// Aggregate adds one event's counters.
func (s *ServiceStatistics) Aggregate(event EventStatistics) {
	s.Queries.Add(event.Queries)
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
