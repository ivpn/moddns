package requests

import "github.com/ivpn/dns/api/model"

type SubscriptionUpdates struct {
	Updates []model.SubscriptionUpdate `json:"updates" validate:"required,dive"`
}

// SubscriptionUpdateReq represents a request to resync a subscription via PASession.
type SubscriptionUpdateReq struct {
	ID    string `json:"id" validate:"required,uuid4"`
	SubID string `json:"subid" validate:"required,uuid4"`
}
