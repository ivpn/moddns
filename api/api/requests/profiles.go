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
	Note   *string `json:"note" validate:"omitempty,max=80"`
	Group  *string `json:"group" validate:"omitempty,max=64"`
	Order  *int    `json:"order" validate:"omitempty,min=0"`
}

// ReorderProfileCustomRulesBody carries the complete ordered list of rule IDs for
// a profile; the rule at index 0 sorts first.
type ReorderProfileCustomRulesBody struct {
	Order []string `json:"order" validate:"required,min=1,max=10000,dive,required"`
}

// CustomRuleGroupUpdate is a single JSON-Patch-style operation on the custom-rule
// group registry, mirroring the shape of model.ProfileUpdate. Group names travel
// in the JSON-Pointer `path`/`from` fields (RFC6901, `~1`=/, `~0`=~) so they never
// appear in the URL. Unlike ProfileUpdate, `path` is open (dynamic group names) and
// validated by format; the decoded name length is checked in the service.
//   - add | replace : set/clear a group's note (value); creates the group.
//   - remove        : delete the group (member rules → Ungrouped, note dropped).
//   - move          : rename `from` → `path` (reassign member rules, move the note).
type CustomRuleGroupUpdate struct {
	Operation string `json:"operation" validate:"required,oneof=add replace remove move"`
	// Action scopes the op to one list ("block" = denylist, "allow" = allowlist);
	// groups are per-list.
	Action string  `json:"action" validate:"required,oneof=block allow"`
	Path   string  `json:"path" validate:"required,startswith=/,max=130"`
	From   string  `json:"from" validate:"omitempty,startswith=/,max=130"`
	Value  *string `json:"value" validate:"omitempty,max=80"`
}

// CustomRuleGroupUpdates is the body of PATCH /custom_rule_groups.
type CustomRuleGroupUpdates struct {
	Updates []CustomRuleGroupUpdate `json:"updates" validate:"required,min=1,max=50,dive"`
}

type ProfileUpdates struct {
	Updates []model.ProfileUpdate `json:"updates" validate:"required,dive"`
}
