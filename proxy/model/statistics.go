package model

import (
	"strings"
	"time"
)

const (
	// StatisticsBucket15Min and StatisticsBucket1Hour are the widths of the two
	// accumulated tiers; the 1-day tier is stamped from closed hours.
	StatisticsBucket15Min = 15 * time.Minute
	StatisticsBucket1Hour = time.Hour
	// MaxDevicesPerProfileBucket bounds distinct device ids per profile in one open bucket.
	MaxDevicesPerProfileBucket = 64
	// OtherDeviceID collects devices beyond the cap; '_' is outside the
	// normalised device id alphabet [A-Za-z0-9 -].
	OtherDeviceID = "_other"
)

// StatisticsRetention is a profile's statistics retention setting; it selects the
// collection documents are written to.
type StatisticsRetention string

const (
	StatisticsRetention30d StatisticsRetention = "30d"
	StatisticsRetention90d StatisticsRetention = "90d"
	StatisticsRetention1y  StatisticsRetention = "1y"
)

// StatisticsTier names a statistics resolution and selects the target collection.
type StatisticsTier string

const (
	StatisticsTier15Min StatisticsTier = "15min"
	StatisticsTier1Hour StatisticsTier = "1h"
	StatisticsTier1Day  StatisticsTier = "1d"
)

// BucketStart returns the UTC start of the tier's bucket containing t.
func (tier StatisticsTier) BucketStart(t time.Time) time.Time {
	t = t.UTC()
	switch tier {
	case StatisticsTier1Day:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case StatisticsTier1Hour:
		return t.Truncate(StatisticsBucket1Hour)
	default:
		return t.Truncate(StatisticsBucket15Min)
	}
}

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
	Retention   StatisticsRetention
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

// Statistics is one profile's counters for one device and one tier bucket,
// written to a time-series collection. Documents from several proxy instances
// and flushes share the same key and are summed by readers.
type Statistics struct {
	BucketStart time.Time      `json:"bucket_start" bson:"bucket_start"`
	Meta        StatisticsMeta `json:"meta" bson:"meta"`

	Total   int64 `json:"total" bson:"total"`
	Blocked int64 `json:"blocked" bson:"blocked"`
	DNSSEC  int64 `json:"dnssec" bson:"dnssec"`

	ReasonBlocklist   int64 `json:"reason_blocklist" bson:"reason_blocklist"`
	ReasonService     int64 `json:"reason_service" bson:"reason_service"`
	ReasonCustomRule  int64 `json:"reason_custom_rule" bson:"reason_custom_rule"`
	ReasonRebinding   int64 `json:"reason_rebinding" bson:"reason_rebinding"`
	ReasonDefaultRule int64 `json:"reason_default_rule" bson:"reason_default_rule"`
	ReasonOther       int64 `json:"reason_other" bson:"reason_other"`

	ProtoDoH int64 `json:"proto_doh" bson:"proto_doh"`
	ProtoDoT int64 `json:"proto_dot" bson:"proto_dot"`
	ProtoDoQ int64 `json:"proto_doq" bson:"proto_doq"`

	// Tier and Retention only select the target collection and are never stored.
	Tier      StatisticsTier      `json:"-" bson:"-"`
	Retention StatisticsRetention `json:"-" bson:"-"`
}

type StatisticsMeta struct {
	ProfileID string `json:"profile_id" bson:"profile_id"`
	DeviceID  string `json:"device_id" bson:"device_id"`
}

// ForDay returns the same counts as a 1-day measurement stamped at the UTC day start.
func (s Statistics) ForDay() Statistics {
	s.Tier = StatisticsTier1Day
	s.BucketStart = StatisticsTier1Day.BucketStart(s.BucketStart)
	return s
}

// Count adds one query's counters, its reason class and its protocol.
func (s *Statistics) Count(q Queries, c *ConsentedStatistics) {
	s.Total += int64(q.Total)
	s.Blocked += int64(q.Blocked)
	s.DNSSEC += int64(q.DNSSEC)
	switch c.ReasonClass {
	case ReasonClassBlocklist:
		s.ReasonBlocklist++
	case ReasonClassService:
		s.ReasonService++
	case ReasonClassCustomRule:
		s.ReasonCustomRule++
	case ReasonClassRebinding:
		s.ReasonRebinding++
	case ReasonClassDefaultRule:
		s.ReasonDefaultRule++
	case ReasonClassOther:
		s.ReasonOther++
	}
	switch c.Protocol {
	case ProtocolDoH:
		s.ProtoDoH++
	case ProtocolDoT:
		s.ProtoDoT++
	case ProtocolDoQ:
		s.ProtoDoQ++
	}
}
