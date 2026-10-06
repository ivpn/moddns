package model

import "errors"

const (
	LAST_1_HOUR   = "LAST_1_HOUR"
	LAST_3_HOURS  = "LAST_3_HOURS"
	LAST_6_HOURS  = "LAST_6_HOURS"
	LAST_12_HOURS = "LAST_12_HOURS"
	LAST_1_DAY    = "LAST_1_DAY"
	LAST_7_DAYS   = "LAST_7_DAYS"
	LAST_MONTH    = "LAST_MONTH"
)

// NewTimespan returns the number of hours for the given timespan
func NewTimespan(timespan string) (int, error) {
	switch timespan {
	case LAST_1_HOUR:
		return 1, nil
	case LAST_12_HOURS:
		return 12, nil
	case LAST_1_DAY:
		return 24, nil
	case LAST_7_DAYS:
		return 24 * 7, nil
	case LAST_MONTH:
		return 24 * 30, nil
	}
	return 0, errors.New("invalid timespan")
}

// NewTopTimespan returns the number of hours for a top-list timespan: the NewTimespan
// values plus the 3 h and 6 h windows (api-endpoint-behaviour.md J20).
func NewTopTimespan(timespan string) (int, error) {
	switch timespan {
	case LAST_3_HOURS:
		return 3, nil
	case LAST_6_HOURS:
		return 6, nil
	}
	return NewTimespan(timespan)
}
