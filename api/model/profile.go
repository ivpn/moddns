package model

import (
	"github.com/ivpn/dns/api/internal/idgen"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaxProfileNameLen is the canonical maximum length (in characters) for a
// profile name. Mirrors the `max=50` literal in the Profile.Name struct tag
// below, in the corresponding ExportedProfile.Name tag in export.go, and in
// the createProfileBody.Name tag in api/api/profiles.go. The reflection-based
// regression tests in profile_test.go and api/api/profiles_test.go assert
// the tags and this const stay aligned — do not change one without the other.
const MaxProfileNameLen = 50

// MaxCustomRulesPerProfile is the hard upper ceiling on how many custom rules a
// single profile may hold. It is a high abuse/resource guard (protecting Redis
// memory and the proxy's in-memory rule cache), not a product-facing limit —
// real users never reach it. Enforced by the create path
// (service/profile/custom_rules.go).
const MaxCustomRulesPerProfile = 10000

// ExportedCustomRulesLimit is the maximum number of custom rules emitted per
// profile in an export, and therefore the per-profile cap accepted on import.
// Exports beyond this are truncated (oldest-first) so an export always re-imports
// cleanly regardless of how many rules a profile accumulated. Mirrors the
// `max` literal in the ExportedSettings.CustomRules tag in export.go; the
// regression test in profile_test.go keeps them aligned — change both together.
const ExportedCustomRulesLimit = 1000

// ExportedCustomRuleGroupsLimit is the maximum number of custom-rule groups emitted
// per list (denylist/allowlist) in an export, and the per-list cap accepted on
// import. Groups are not bounded by rule count (empty groups exist in the registry
// independently of any rule), so they need their own modest guard against
// empty-group bloat — real users have a handful. Mirrors the `max` literal in the
// CustomRuleGroups.Block/Allow tags; the regression test in profile_test.go keeps
// them aligned — change both together.
const ExportedCustomRuleGroupsLimit = 100

// Profile represents a DNS profile
type Profile struct {
	ID        primitive.ObjectID `json:"id" bson:"_id" binding:"required"`
	ProfileId string             `json:"profile_id" bson:"profile_id" binding:"required"`
	AccountId string             `json:"account_id" bson:"account_id" binding:"required"`
	Name      string             `json:"name" validate:"required,max=50,safe_name" binding:"required"`
	Settings  *ProfileSettings   `json:"settings" bson:"settings" binding:"required"`
}

// New creates a new profile
func NewProfile(idGen idgen.Generator, name, accountId string) (*Profile, error) {
	profileId, err := idGen.Generate()
	if err != nil {
		return nil, err
	}
	return &Profile{
		ID:        primitive.NewObjectID(),
		ProfileId: profileId,
		AccountId: accountId,
		Name:      name,
	}, nil
}

// ProfileUpdate represents profile settings update
// RFC6902 JSON Patch format is used
type ProfileUpdate struct {
	Operation string `json:"operation" validate:"required,oneof=remove add replace move copy"`
	Path      string `json:"path" validate:"required,oneof=/name /settings/statistics/enabled /settings/logs/enabled /settings/logs/log_clients_ips /settings/logs/log_domains /settings/logs/retention /settings/privacy/default_rule /settings/privacy/blocklists_subdomains_rule /settings/privacy/custom_rules_subdomains_rule /settings/security/dnssec/enabled /settings/security/dnssec/send_do_bit /settings/advanced/recursor"`
	Value     any    `json:"value" validate:"required"`
}
