package model

// Security represents security settings
type Security struct {
	DNSSECSettings      DNSSECSettings      `json:"dnssec" bson:"dnssec" redis:"dnssec" binding:"required"`
	RebindingProtection RebindingProtection `json:"rebinding_protection" bson:"rebinding_protection" redis:"rebinding_protection"`
}

type DNSSECSettings struct {
	Enabled   bool `json:"enabled" bson:"enabled" redis:"enabled" binding:"required"`
	SendDoBit bool `json:"send_do_bit" bson:"send_do_bit" redis:"send_do_bit" binding:"required"`
}

// RebindingProtection holds the per-profile DNS rebinding protection toggle.
// When enabled, the proxy blocks answers where a public name resolves to a
// private/loopback/link-local IP. Default off (opt-in).
type RebindingProtection struct {
	Enabled bool `json:"enabled" bson:"enabled" redis:"enabled"`
}
