package model

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	RetentionOneHour  Retention = "1h"
	RetentionSixHours Retention = "6h"
	RetentionOneDay   Retention = "1d"
	RetentionOneWeek  Retention = "1w"
	RetentionOneMonth Retention = "1m"
)

type QueryLog struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	Timestamp time.Time          `json:"timestamp" bson:"timestamp"`
	ProfileID string             `json:"profile_id" bson:"profile_id"`
	DeviceId  string             `json:"device_id" bson:"device_id"`
	Status    string             `json:"status" bson:"status"`
	Reasons   []string           `json:"reasons" bson:"reasons"`
	// Outcome is the proxy-computed resolution-outcome token
	// (docs/specs/query-log-outcomes-behaviour.md). Empty on legacy entries.
	Outcome    string     `json:"outcome,omitempty" bson:"outcome,omitempty"`
	DNSRequest DNSRequest `json:"dns_request" bson:"dns_request"`
	ClientIP   string     `json:"client_ip" bson:"client_ip"`
	Protocol   string     `json:"protocol" bson:"protocol"`
}

// MarshalJSON renders Reasons as an empty JSON array ([]) instead of null when
// nil, so the API always returns a list for this field.
func (q QueryLog) MarshalJSON() ([]byte, error) {
	type alias QueryLog
	a := alias(q)
	if a.Reasons == nil {
		a.Reasons = []string{}
	}
	return json.Marshal(a)
}

type DNSRequest struct {
	Domain       string `json:"domain" bson:"domain"`
	QueryType    string `json:"query_type" bson:"query_type"`
	ResponseCode string `json:"response_code" bson:"response_code"`
	DNSSEC       bool   `json:"dnssec" bson:"dnssec"`
}

// QueryLogDevice is one distinct device seen in a profile's query logs within
// the current retention window. DeviceId is the user-authored device name from
// the DNS stamp / DoH path (libs/deviceid, max 36 chars). The bson "_id" tag
// decodes the $group aggregation output directly.
type QueryLogDevice struct {
	DeviceId string    `json:"device_id" bson:"_id"`
	LastSeen time.Time `json:"last_seen" bson:"last_seen"`
}
