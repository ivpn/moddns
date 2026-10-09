package model

import (
	"errors"
	"time"

	"github.com/ivpn/dns/libs/statistics"
)

// StatisticsRetention is the profile's statistics retention setting (api-endpoint-behaviour.md J48).
// The values and rules live in libs/statistics; this named type keeps the swagger enum
// separate from the logs Retention.
type StatisticsRetention string

const (
	StatisticsRetention30d StatisticsRetention = "30d"
	StatisticsRetention90d StatisticsRetention = "90d"
	StatisticsRetention1y  StatisticsRetention = "1y"
)

func (r StatisticsRetention) lib() statistics.Retention { return statistics.Retention(r) }

// Valid reports whether r is an allowed value.
func (r StatisticsRetention) Valid() bool { return r.lib().Valid() }

// OrDefault returns r, or 30d when r is not allowed.
func (r StatisticsRetention) OrDefault() StatisticsRetention {
	return StatisticsRetention(r.lib().OrDefault())
}

// Window is how far back the retention reaches.
func (r StatisticsRetention) Window() time.Duration { return r.lib().Window() }

// MarshalBinary lets go-redis write the value; it has no fallback for named strings.
func (r StatisticsRetention) MarshalBinary() ([]byte, error) { return []byte(r), nil }

// Timespans accepted by the statistics read endpoint; the logs endpoints keep
// their own enum (timespan.go).
const (
	StatisticsLast3Hours  = "LAST_3_HOURS"
	StatisticsLast6Hours  = "LAST_6_HOURS"
	StatisticsLast3Months = "LAST_3_MONTHS"
	StatisticsLastYear    = "LAST_YEAR"

	// StatisticsDefaultTimespan applies when the query parameter is absent.
	StatisticsDefaultTimespan = LAST_7_DAYS

	// StatisticsMaxDevices bounds the devices list, the folded entry included.
	StatisticsMaxDevices = 100

	// StatisticsOtherDeviceID is the proxy's overflow sentinel; the API reuses it for the fold.
	StatisticsOtherDeviceID = "_other"
)

// StatisticsTier is one stored resolution of the statistics collections.
type StatisticsTier string

const (
	StatisticsTier15Min StatisticsTier = "15min"
	StatisticsTier1H    StatisticsTier = "1h"
	StatisticsTier1D    StatisticsTier = "1d"
)

// StatisticsTimespanSpec is how far back a timespan reaches, the tier it reads
// and the width of its series points (the tier's own resolution).
type StatisticsTimespanSpec struct {
	Window time.Duration
	Bucket time.Duration
	Tier   StatisticsTier
}

var statisticsTimespans = map[string]StatisticsTimespanSpec{
	StatisticsLast3Hours:  {3 * time.Hour, 15 * time.Minute, StatisticsTier15Min},
	StatisticsLast6Hours:  {6 * time.Hour, 15 * time.Minute, StatisticsTier15Min},
	LAST_1_DAY:            {24 * time.Hour, time.Hour, StatisticsTier1H},
	LAST_7_DAYS:           {7 * 24 * time.Hour, time.Hour, StatisticsTier1H},
	LAST_MONTH:            {30 * 24 * time.Hour, 24 * time.Hour, StatisticsTier1D},
	StatisticsLast3Months: {90 * 24 * time.Hour, 24 * time.Hour, StatisticsTier1D},
	StatisticsLastYear:    {365 * 24 * time.Hour, 24 * time.Hour, StatisticsTier1D},
}

// StatisticsTimespans lists the accepted timespans.
func StatisticsTimespans() []string {
	return []string{StatisticsLast3Hours, StatisticsLast6Hours, LAST_1_DAY, LAST_7_DAYS, LAST_MONTH, StatisticsLast3Months, StatisticsLastYear}
}

// NewStatisticsTimespan resolves a statistics timespan.
func NewStatisticsTimespan(timespan string) (StatisticsTimespanSpec, error) {
	spec, ok := statisticsTimespans[timespan]
	if !ok {
		return StatisticsTimespanSpec{}, errors.New("invalid timespan")
	}
	return spec, nil
}

// StatisticsRetentions lists the allowed retention values, shortest first.
func StatisticsRetentions() []StatisticsRetention {
	out := []StatisticsRetention{}
	for _, r := range statistics.Retentions() {
		out = append(out, StatisticsRetention(r))
	}
	return out
}

// StatisticsRetentionWindow resolves a retention setting; an empty or unknown
// value reads as 30d.
func StatisticsRetentionWindow(r StatisticsRetention) (string, time.Duration) {
	return string(r.OrDefault()), r.Window()
}

type StatisticsTotals struct {
	Total   int64 `json:"total"`
	Blocked int64 `json:"blocked"`
	Dnssec  int64 `json:"dnssec"`
}

type StatisticsPoint struct {
	Ts      time.Time `json:"ts"`
	Total   int64     `json:"total"`
	Blocked int64     `json:"blocked"`
	Dnssec  int64     `json:"dnssec"`
}

type StatisticsReasons struct {
	Blocklist   int64 `json:"blocklist"`
	Service     int64 `json:"service"`
	CustomRule  int64 `json:"custom_rule"`
	Rebinding   int64 `json:"rebinding"`
	DefaultRule int64 `json:"default_rule"`
	Other       int64 `json:"other"`
}

type StatisticsProtocols struct {
	DoH int64 `json:"doh"`
	DoT int64 `json:"dot"`
	DoQ int64 `json:"doq"`
}

type StatisticsDevice struct {
	DeviceID string `json:"device_id"`
	Total    int64  `json:"total"`
	Blocked  int64  `json:"blocked"`
}

// StatisticsAggregate is what the repository returns for a range: sparse series
// (only buckets that hold data) and the unfolded device list.
type StatisticsAggregate struct {
	Totals    StatisticsTotals
	Series    []StatisticsPoint
	Reasons   StatisticsReasons
	Protocols StatisticsProtocols
	Devices   []StatisticsDevice
}

// StatisticsResponse is the body of GET /profiles/{id}/statistics.
type StatisticsResponse struct {
	Enabled   bool       `json:"enabled"`
	EnabledAt *time.Time `json:"enabled_at"`
	// Last "Delete statistics history" (J53); null while off or never deleted.
	HistoryDeletedAt *time.Time          `json:"history_deleted_at"`
	Retention        string              `json:"retention"`
	Timespan         string              `json:"timespan"`
	From             time.Time           `json:"from"`
	To               time.Time           `json:"to"`
	BucketSeconds    int                 `json:"bucket_seconds"`
	Totals           StatisticsTotals    `json:"totals"`
	Series           []StatisticsPoint   `json:"series"`
	Reasons          StatisticsReasons   `json:"reasons"`
	Protocols        StatisticsProtocols `json:"protocols"`
	Devices          []StatisticsDevice  `json:"devices"`
}
