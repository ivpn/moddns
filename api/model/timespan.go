package model

import (
	"errors"
	"slices"
	"strings"
)

// Timespans: a read window ending now. Each endpoint family accepts its own TimespanSet.
const (
	LAST_1_HOUR   = "LAST_1_HOUR"
	LAST_3_HOURS  = "LAST_3_HOURS"
	LAST_6_HOURS  = "LAST_6_HOURS"
	LAST_12_HOURS = "LAST_12_HOURS"
	LAST_1_DAY    = "LAST_1_DAY"
	LAST_7_DAYS   = "LAST_7_DAYS"
	LAST_MONTH    = "LAST_MONTH"
	LAST_3_MONTHS = "LAST_3_MONTHS"
	LAST_YEAR     = "LAST_YEAR"
)

// TimespanSet is the timespans one endpoint family accepts, in display order.
type TimespanSet []string

// Contains reports whether timespan belongs to the set.
func (s TimespanSet) Contains(timespan string) bool { return slices.Contains(s, timespan) }

// String lists the set for error messages and docs: "A, B, C".
func (s TimespanSet) String() string { return strings.Join(s, ", ") }

var (
	// QueryLogsTimespans is accepted by GET /profiles/{id}/logs and its download (J1).
	QueryLogsTimespans = TimespanSet{LAST_1_HOUR, LAST_12_HOURS, LAST_1_DAY, LAST_7_DAYS, LAST_MONTH}
	// QueryLogsTopTimespans is accepted by GET /profiles/{id}/logs/top/* (J20): the logs set plus
	// the 3 h and 6 h windows the statistics page also offers.
	QueryLogsTopTimespans = TimespanSet{LAST_1_HOUR, LAST_3_HOURS, LAST_6_HOURS, LAST_12_HOURS, LAST_1_DAY, LAST_7_DAYS, LAST_MONTH}
	// StatisticsTimespanSet is accepted by GET /profiles/{id}/statistics (J40).
	StatisticsTimespanSet = TimespanSet{LAST_3_HOURS, LAST_6_HOURS, LAST_1_DAY, LAST_7_DAYS, LAST_MONTH, LAST_3_MONTHS, LAST_YEAR}
)

// QueryLogsTopDefaultTimespan applies when a top-list request has no timespan (J20).
const QueryLogsTopDefaultTimespan = LAST_1_DAY

var queryLogTimespanHours = map[string]int{
	LAST_1_HOUR:   1,
	LAST_3_HOURS:  3,
	LAST_6_HOURS:  6,
	LAST_12_HOURS: 12,
	LAST_1_DAY:    24,
	LAST_7_DAYS:   24 * 7,
	LAST_MONTH:    24 * 30,
}

var errInvalidTimespan = errors.New("invalid timespan")

// NewTimespan returns the number of hours of a QueryLogsTimespans value.
func NewTimespan(timespan string) (int, error) {
	if !QueryLogsTimespans.Contains(timespan) {
		return 0, errInvalidTimespan
	}
	return queryLogTimespanHours[timespan], nil
}

// NewTopTimespan returns the number of hours of a QueryLogsTopTimespans value.
func NewTopTimespan(timespan string) (int, error) {
	if !QueryLogsTopTimespans.Contains(timespan) {
		return 0, errInvalidTimespan
	}
	return queryLogTimespanHours[timespan], nil
}
