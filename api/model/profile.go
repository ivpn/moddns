package model

import (
	"github.com/ivpn/dns/api/internal/idgen"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Profile represents a DNS profile
type Profile struct {
	ID        primitive.ObjectID `json:"id" bson:"_id" binding:"required"`
	ProfileId string             `json:"profile_id" bson:"profile_id" binding:"required"`
	AccountId string             `json:"account_id" bson:"account_id" binding:"required"`
	Name      string             `json:"name" validate:"required,max=50" binding:"required"`
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
	Path      string `json:"path" validate:"required,oneof=/name /settings/statistics/enabled /settings/logs/enabled /settings/logs/log_clients_ips /settings/logs/log_domains /settings/logs/retention /settings/privacy/default_rule /settings/privacy/subdomains_rule /settings/privacy/custom_rules_subdomains /settings/security/dnssec/enabled /settings/security/dnssec/send_do_bit /settings/advanced/recursor"`
	Value     any    `json:"value" validate:"required"`
}
