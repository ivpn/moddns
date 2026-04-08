package requests

import "github.com/ivpn/dns/api/model"

type SubscriptionUpdates struct {
	Updates []model.SubscriptionUpdate `json:"updates" validate:"required,dive"`
}

