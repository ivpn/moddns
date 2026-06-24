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

// UpdateProfileCustomRuleBody is the partial-update payload for a single custom
// rule. All fields are optional; a nil pointer leaves the field unchanged, while
// a present value (including an empty string) is applied.
type UpdateProfileCustomRuleBody struct {
	Action *string `json:"action" validate:"omitempty,oneof=block allow comment"`
	Value  *string `json:"value" validate:"omitempty,ipv4|ipv6|fqdn|fqdn_wildcard|asn"`
	Note   *string `json:"note" validate:"omitempty,max=280"`
	Group  *string `json:"group" validate:"omitempty,max=64"`
	Order  *int    `json:"order" validate:"omitempty,min=0"`
}

// ReorderProfileCustomRulesBody carries the complete ordered list of rule IDs for
// a profile; the rule at index 0 sorts first.
type ReorderProfileCustomRulesBody struct {
	Order []string `json:"order" validate:"required,min=1,max=10000,dive,required"`
}

// SetCustomRuleGroupsBody upserts group notes. A null value for a key deletes
// that group's note.
type SetCustomRuleGroupsBody struct {
	Groups map[string]*string `json:"groups" validate:"required"`
}

type ProfileUpdates struct {
	Updates []model.ProfileUpdate `json:"updates" validate:"required,dive"`
}
