package model

type ServicesSettings struct {
	Blocked []string `json:"blocked" bson:"blocked" redis:"-"`
}

// Privacy struct holds the blocklists and other privacy-related settings
type Privacy struct {
	Blocklists              []string          `json:"blocklists" bson:"blocklists" redis:"-"`
	Services                *ServicesSettings `json:"services" bson:"services" redis:"-"`
	DefaultRule             string            `json:"default_rule" bson:"default_rule" redis:"default_rule" validate:"required,oneof=block allow"`
	SubdomainsRule          string            `json:"subdomains_rule" bson:"subdomains_rule" redis:"subdomains_rule" validate:"required,oneof=block allow"`
	CustomRulesSubdomains   string            `json:"custom_rules_subdomains" bson:"custom_rules_subdomains" redis:"custom_rules_subdomains" validate:"omitempty,oneof=include exact"`
}
