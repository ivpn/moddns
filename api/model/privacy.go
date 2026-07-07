package model

import "encoding/json"

// Privacy struct holds the blocklists and other privacy-related settings
type Privacy struct {
	Blocklists                []string `json:"blocklists" bson:"blocklists,omitempty" redis:"-"`
	Services                  []string `json:"services" bson:"services,omitempty" redis:"-"`
	DefaultRule               string   `json:"default_rule" bson:"default_rule" redis:"default_rule" validate:"required,oneof=block allow"`
	BlocklistsSubdomainsRule  string   `json:"blocklists_subdomains_rule" bson:"blocklists_subdomains_rule" redis:"blocklists_subdomains_rule" validate:"required,oneof=block allow"`
	CustomRulesSubdomainsRule string   `json:"custom_rules_subdomains_rule" bson:"custom_rules_subdomains_rule" redis:"custom_rules_subdomains_rule" validate:"omitempty,oneof=include exact"`
}

// MarshalJSON renders Blocklists and Services as empty JSON arrays ([]) instead of
// null when they are nil, so the API always returns a list for these fields even
// when the profile has none enabled. A nil []string otherwise marshals to `null`,
// which reads poorly for clients. This only affects the JSON representation; the
// stored BSON value (empty or, with omitempty, absent) is unchanged.
func (p Privacy) MarshalJSON() ([]byte, error) {
	type alias Privacy
	a := alias(p)
	if a.Blocklists == nil {
		a.Blocklists = []string{}
	}
	if a.Services == nil {
		a.Services = []string{}
	}
	return json.Marshal(a)
}
