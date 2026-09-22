package model

// EventQueryLog holds a DNS query log data with additional retention information
type EventQueryLog struct {
	QueryLog QueryLog
	Metadata Metadata
}

// EventStatistics is one query's counter increments (total 1, blocked and
// dnssec 0 or 1). It carries no identifier and no time on purpose.
type EventStatistics struct {
	Queries Queries
}

type Metadata struct {
	Retention Retention
}

type Retention string

const (
	RetentionOneHour  Retention = "1h"
	RetentionSixHours Retention = "6h"
	RetentionOneDay   Retention = "1d"
	RetentionOneWeek  Retention = "1w"
	RetentionOneMonth Retention = "1m"
)
