package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

// ProfileRepository represents a profile repository
type ProfileRepository interface {
	CreateProfile(ctx context.Context, profile *model.Profile) error
	CreateCustomRules(ctx context.Context, profileId string, rules []*model.CustomRule) error
	RemoveCustomRules(ctx context.Context, profileId string, ruleIds []string) error
	UpdateCustomRule(ctx context.Context, profileId string, rule *model.CustomRule) error
	UpdateCustomRulesOrder(ctx context.Context, profileId string, idToOrder map[string]int) error
	SetCustomRuleGroups(ctx context.Context, profileId string, groups map[string]string) error
	ReassignCustomRuleGroup(ctx context.Context, profileId, from, to string) error
	EnableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error
	DisableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error
	EnableServices(ctx context.Context, profileId string, serviceIds []string) error
	DisableServices(ctx context.Context, profileId string, serviceIds []string) error
	GetProfileById(ctx context.Context, profileId string) (*model.Profile, error)
	GetProfilesByAccountId(ctx context.Context, accountId string) ([]model.Profile, error)
	Update(ctx context.Context, profileId string, profile *model.Profile) error
	UpdateSettings(ctx context.Context, profileId string, settings *model.ProfileSettings) error
	DeleteProfileById(ctx context.Context, profileId string) error
}
