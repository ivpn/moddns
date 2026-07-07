package model

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Profile default-rule values (the fallback action when no rule matches). The
// custom-rule action/syntax vocabulary they reference lives in custom_rule.go.
const (
	DEFAULT_RULE_BLOCK = ACTION_BLOCK
	DEFAULT_RULE_ALLOW = ACTION_ALLOW
)

// ProfileSettings represents profile settings, it's internal model used in `profiles` collection
type ProfileSettings struct {
	ProfileId   string        `json:"profile_id" bson:"profile_id" redis:"profile_id" binding:"required"`
	Security    *Security     `json:"security" bson:"security" redis:"security" binding:"required"`
	Privacy     *Privacy      `json:"privacy" bson:"privacy" redis:"privacy" binding:"required"`
	CustomRules []*CustomRule `json:"custom_rules" bson:"custom_rules" redis:"-"`
	// CustomRuleGroups is the per-list group registry (denylist/allowlist).
	// Organizational metadata only; never synced to the proxy (redis:"-").
	CustomRuleGroups CustomRuleGroups    `json:"custom_rule_groups" bson:"custom_rule_groups" redis:"-"`
	Logs             *LogsSettings       `json:"logs" bson:"logs" redis:"-" binding:"required"`
	Statistics       *StatisticsSettings `json:"statistics" bson:"statistics" redis:"-" binding:"required"`
	Advanced         *Advanced           `json:"advanced" bson:"advanced" redis:"advanced" binding:"required"`
}

// MarshalJSON renders CustomRules as an empty JSON array ([]) instead of null
// when nil, so the API always returns a list. Storage (bson) is unchanged.
func (s ProfileSettings) MarshalJSON() ([]byte, error) {
	type alias ProfileSettings
	a := alias(s)
	if a.CustomRules == nil {
		a.CustomRules = []*CustomRule{}
	}
	return json.Marshal(a)
}

// NewSettings creates a new, empty settings object
func NewSettings() *ProfileSettings {
	return &ProfileSettings{
		Privacy: &Privacy{
			Blocklists:                make([]string, 0),
			Services:                  make([]string, 0),
			DefaultRule:               DEFAULT_RULE_ALLOW,
			BlocklistsSubdomainsRule:  ACTION_BLOCK,
			CustomRulesSubdomainsRule: CUSTOM_RULES_SUBDOMAINS_INCLUDE,
		},
		Security: &Security{
			DNSSECSettings: DNSSECSettings{
				Enabled:   true,
				SendDoBit: false,
			},
		},
		Logs: &LogsSettings{
			Enabled:       false,
			LogClientsIPs: false,
			LogDomains:    true,
			Retention:     RetentionOneDay,
		},
		Statistics: &StatisticsSettings{
			Enabled: false,
		},
		CustomRules:      make([]*CustomRule, 0),
		CustomRuleGroups: CustomRuleGroups{},
		Advanced: &Advanced{
			Recursor: RECURSOR_DEFAULT,
		},
	}
}

// StatisticsSettings represents statistics/analytics settings
type StatisticsSettings struct {
	Enabled bool `json:"enabled" bson:"enabled" redis:"enabled" binding:"required"`
}

type LogsSettings struct {
	Enabled       bool      `json:"enabled" bson:"enabled" redis:"enabled" binding:"required"`
	LogClientsIPs bool      `json:"log_clients_ips" bson:"log_clients_ips" redis:"log_clients_ips" binding:"required"`
	LogDomains    bool      `json:"log_domains" bson:"log_domains" redis:"log_domains" binding:"required"`
	Retention     Retention `json:"retention" bson:"retention" redis:"retention" binding:"required"`
}

type Retention string

var (
	ErrInvalidRetention = errors.New("invalid retention value")
)

func (r Retention) MarshalBinary() (data []byte, err error) {
	return []byte(fmt.Sprint(r)), nil
}

func NewRetention(retention string) (Retention, error) {
	switch retention {
	case "1h", "6h", "1d", "1w", "1m":
		return Retention(retention), nil
	default:
		return "", ErrInvalidRetention
	}
}
