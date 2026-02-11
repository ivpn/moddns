package model

// Privacy struct holds the blocklists and other privacy-related settings
type Privacy struct {
	Blocklists                []string `json:"blocklists" bson:"blocklists" redis:"-"`
	DefaultRule               string   `json:"default_rule" bson:"default_rule" redis:"default_rule" validate:"required,oneof=block allow"`
	BlocklistsSubdomainsRule  string   `json:"blocklists_subdomains_rule" bson:"blocklists_subdomains_rule" redis:"blocklists_subdomains_rule" validate:"required,oneof=block allow"`
	CustomRulesSubdomainsRule string   `json:"custom_rules_subdomains_rule" bson:"custom_rules_subdomains_rule" redis:"custom_rules_subdomains_rule" validate:"omitempty,oneof=include exact"`
}
