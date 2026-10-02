package model

import (
	"strings"
	"time"
)

const (
	// StatisticsBucket is the width of one consented statistics document.
	StatisticsBucket = 15 * time.Minute
	// MaxDevicesPerProfileBucket bounds distinct device ids per profile in one open bucket.
	MaxDevicesPerProfileBucket = 64
	// OtherDeviceID collects devices beyond the cap; '_' is outside the
	// normalised device id alphabet [A-Za-z0-9 -].
	OtherDeviceID = "_other"
)

// ReasonClass is the single reason a blocked query is attributed to.
type ReasonClass string

const (
	ReasonClassBlocklist   ReasonClass = "blocklist"
	ReasonClassService     ReasonClass = "service"
	ReasonClassCustomRule  ReasonClass = "custom_rule"
	ReasonClassRebinding   ReasonClass = "rebinding"
	ReasonClassDefaultRule ReasonClass = "default_rule"
	ReasonClassOther       ReasonClass = "other"
)

// StatisticsProtocol is an encrypted transport counted in statistics.
type StatisticsProtocol string

const (
	ProtocolDoH StatisticsProtocol = "doh"
	ProtocolDoT StatisticsProtocol = "dot"
	ProtocolDoQ StatisticsProtocol = "doq"
)

// ConsentedStatistics is the per-profile part of a statistics event; it is set
// only for profiles with statistics enabled.
type ConsentedStatistics struct {
	ProfileID string
	DeviceID  string
	// Retention is the profile's statistics retention setting, verbatim.
	Retention   Retention
	ReasonClass ReasonClass
	Protocol    StatisticsProtocol
}

const (
	classRankNone        = 0
	classRankDefaultRule = 1
	classRankService     = 2
	classRankBlocklist   = 3
	classRankRebinding   = 4
	classRankCustomRule  = 5
)

// ClassifyReasons maps a blocked query's filter reasons to one class by
// precedence custom_rule > rebinding > blocklist > service > default_rule.
// cname_uncloaking is a modifier and never a class.
func ClassifyReasons(reasons []string) ReasonClass {
	best := classRankNone
	for _, r := range reasons {
		rank := classRankNone
		switch {
		case r == FilterReasonCustomRules:
			rank = classRankCustomRule
		case r == FilterReasonRebinding:
			rank = classRankRebinding
		case r == FilterReasonBlocklists, r == FilterReasonBlocklistsSubdomains, strings.HasPrefix(r, FilterReasonBlocklistPrefix):
			rank = classRankBlocklist
		case r == FilterReasonServices, strings.HasPrefix(r, FilterReasonServicePrefix):
			rank = classRankService
		case r == FilterReasonDefaultRule:
			rank = classRankDefaultRule
		}
		if rank > best {
			best = rank
		}
	}
	switch best {
	case classRankCustomRule:
		return ReasonClassCustomRule
	case classRankRebinding:
		return ReasonClassRebinding
	case classRankBlocklist:
		return ReasonClassBlocklist
	case classRankService:
		return ReasonClassService
	case classRankDefaultRule:
		return ReasonClassDefaultRule
	default:
		return ReasonClassOther
	}
}

// StatisticsBucketStart returns the UTC start of the 15-minute bucket containing t.
func StatisticsBucketStart(t time.Time) time.Time {
	return t.UTC().Truncate(StatisticsBucket)
}

// Statistics is one profile's counters for one device and one 15-minute
// bucket, written to a time-series collection. Documents from several proxy
// instances and flushes share the same key and are summed by readers.
type Statistics struct {
	BucketStart time.Time           `json:"bucket_start" bson:"bucket_start"`
	Meta        StatisticsMeta      `json:"meta" bson:"meta"`
	Queries     StatisticsQueries   `json:"queries" bson:"queries"`
	Reasons     StatisticsReasons   `json:"reasons" bson:"reasons"`
	Protocols   StatisticsProtocols `json:"protocols" bson:"protocols"`
	// Retention only selects the target collection and is never stored.
	Retention string `json:"-" bson:"-"`
}

type StatisticsMeta struct {
	ProfileID string `json:"profile_id" bson:"profile_id"`
	DeviceID  string `json:"device_id" bson:"device_id"`
}

type StatisticsQueries struct {
	Total   int64 `json:"total" bson:"total"`
	Blocked int64 `json:"blocked" bson:"blocked"`
	DNSSEC  int64 `json:"dnssec" bson:"dnssec"`
}

type StatisticsReasons struct {
	Blocklist   int64 `json:"blocklist" bson:"blocklist"`
	Service     int64 `json:"service" bson:"service"`
	CustomRule  int64 `json:"custom_rule" bson:"custom_rule"`
	Rebinding   int64 `json:"rebinding" bson:"rebinding"`
	DefaultRule int64 `json:"default_rule" bson:"default_rule"`
	Other       int64 `json:"other" bson:"other"`
}

type StatisticsProtocols struct {
	DoH int64 `json:"doh" bson:"doh"`
	DoT int64 `json:"dot" bson:"dot"`
	DoQ int64 `json:"doq" bson:"doq"`
}

// Count adds one query's counters, its reason class and its protocol.
func (s *Statistics) Count(q Queries, c *ConsentedStatistics) {
	s.Queries.Total += int64(q.Total)
	s.Queries.Blocked += int64(q.Blocked)
	s.Queries.DNSSEC += int64(q.DNSSEC)
	switch c.ReasonClass {
	case ReasonClassBlocklist:
		s.Reasons.Blocklist++
	case ReasonClassService:
		s.Reasons.Service++
	case ReasonClassCustomRule:
		s.Reasons.CustomRule++
	case ReasonClassRebinding:
		s.Reasons.Rebinding++
	case ReasonClassDefaultRule:
		s.Reasons.DefaultRule++
	case ReasonClassOther:
		s.Reasons.Other++
	}
	switch c.Protocol {
	case ProtocolDoH:
		s.Protocols.DoH++
	case ProtocolDoT:
		s.Protocols.DoT++
	case ProtocolDoQ:
		s.Protocols.DoQ++
	}
}
