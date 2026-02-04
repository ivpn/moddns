package requests

import (
	"github.com/ivpn/dns/api/model"
)

type CreateProfileCustomRuleBody struct {
	Action string `json:"action" validate:"required,oneof=block allow comment"`
	Value  string `json:"value" validate:"required,ipv4|ipv6|fqdn|fqdn_wildcard|asn"`
}

type CreateProfileCustomRulesBatchBody struct {
	Action string   `json:"action" validate:"required,oneof=block allow comment"`
	Values []string `json:"values" validate:"required,min=1,max=20,dive,required,ipv4|ipv6|fqdn|fqdn_wildcard|asn"`
}

type ProfileUpdates struct {
	Updates []model.ProfileUpdate `json:"updates" validate:"required,dive"`
}
