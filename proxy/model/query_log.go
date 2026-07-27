package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type QueryLog struct {
	ID         primitive.ObjectID `json:"id" bson:"_id"`
	Timestamp  time.Time          `json:"timestamp" bson:"timestamp"`
	ProfileID  string             `json:"profile_id" bson:"profile_id"`
	DeviceId   string             `json:"device_id" bson:"device_id"`
	Status     string             `json:"status" bson:"status"`
	Reasons    []string           `json:"reasons" bson:"reasons"`
	// Outcome is the resolution-outcome token (docs/specs/query-log-outcomes-behaviour.md
	// rows O1-O10). Empty on entries written before the field existed.
	Outcome string `json:"outcome,omitempty" bson:"outcome,omitempty"`
	DNSRequest DNSRequest         `json:"dns_request" bson:"dns_request"`
	ClientIP   string             `json:"client_ip" bson:"client_ip"`
	Protocol   string             `json:"protocol" bson:"protocol"`
}

type DNSRequest struct {
	Domain       string `json:"domain" bson:"domain"`
	QueryType    string `json:"query_type" bson:"query_type"`
	ResponseCode string `json:"response_code" bson:"response_code"`
	DNSSEC       bool   `json:"dnssec" bson:"dnssec"`
}
