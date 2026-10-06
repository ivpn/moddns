package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

// FieldSet sets one stored field, named by its dotted document path.
type FieldSet struct {
	Field string
	Value any
}

// ProfileFieldsUpdate is applied to one profile as a single atomic update.
type ProfileFieldsUpdate struct {
	Set []FieldSet
	// EnabledAtNow becomes settings.statistics.enabled_at when Set turns statistics on;
	// enabled_at is derived from the stored value being replaced (api-endpoint-behaviour.md G22).
	EnabledAtNow time.Time
	// AppendCustomRules are appended unless a stored rule has the same value.
	AppendCustomRules []*model.CustomRule
}

// ProfileRepository represents a profile repository
type ProfileRepository interface {
	CreateProfile(ctx context.Context, profile *model.Profile) error
	CreateCustomRules(ctx context.Context, profileId string, rules []*model.CustomRule) error
	RemoveCustomRules(ctx context.Context, profileId string, ruleIds []string) error
	UpdateCustomRule(ctx context.Context, profileId string, rule *model.CustomRule) error
	UpdateCustomRulesOrder(ctx context.Context, profileId string, idToOrder map[string]int) error
	SetCustomRuleGroups(ctx context.Context, profileId string, groups model.CustomRuleGroups) error
	ReassignCustomRuleGroup(ctx context.Context, profileId, action, from, to string) error
	EnableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error
	DisableBlocklists(ctx context.Context, profileId string, blocklistIds []string) error
	EnableServices(ctx context.Context, profileId string, serviceIds []string) error
	DisableServices(ctx context.Context, profileId string, serviceIds []string) error
	GetProfileById(ctx context.Context, profileId string) (*model.Profile, error)
	// GetProfilesStatisticsSettings reads only settings.statistics of the given profiles from the
	// primary. Profiles that do not exist are absent from the result; a profile with no statistics
	// block maps to nil.
	GetProfilesStatisticsSettings(ctx context.Context, profileIds []string) (map[string]*model.StatisticsSettings, error)
	// GetProfilesLogsEnabled reads only settings.logs.enabled of the given profiles from the
	// primary. Profiles that do not exist are absent; a missing logs block maps to false.
	GetProfilesLogsEnabled(ctx context.Context, profileIds []string) (map[string]bool, error)
	GetProfilesByAccountId(ctx context.Context, accountId string) ([]model.Profile, error)
	// UpdateFields applies upd atomically and returns the profile as stored immediately before
	// the update and as re-read from the primary after it.
	UpdateFields(ctx context.Context, profileId string, upd ProfileFieldsUpdate) (before, after *model.Profile, err error)
	UpdateSettings(ctx context.Context, profileId string, settings *model.ProfileSettings) error
	DeleteProfileById(ctx context.Context, profileId string) error
}
