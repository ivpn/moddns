package model

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ACTION_BLOCK                    = "block"
	ACTION_ALLOW                    = "allow"
	ACTION_COMMENT                  = "comment"
	DEFAULT_RULE_BLOCK              = ACTION_BLOCK
	DEFAULT_RULE_ALLOW              = ACTION_ALLOW
	CUSTOM_RULES_SUBDOMAINS_INCLUDE = "include"
	CUSTOM_RULES_SUBDOMAINS_EXACT   = "exact"
	SYNTAX_IPV4                     = "ip4_addr"
	SYNTAX_IPV4_WILDCARD            = "ip4_wildcard"
	SYNTAX_IPV4_CIDR                = "ip4_cidr"
	SYNTAX_IPV6                     = "ip6"
	SYNTAX_IPV6_WILDCARD            = "ip6_wildcard"
	SYNTAX_IPV6_CIDR                = "ip6_cidr"
	SYNTAX_FQDN                     = "fqdn"
	SYNTAX_FQDN_WILDCARD            = "fqdn_wildcard"
	SYNTAX_ASN                      = "asn"
	SYNTAX_UNKNOWN                  = "unknown_syntax"
)

var (
	ErrInvalidCustomRuleAction = errors.New("invalid custom rule action type")
	ErrInvalidCustomRuleSyntax = errors.New("invalid custom rule value syntax")
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

// CustomRuleGroup is one organizational group within a list. Modeled as a struct
// (rather than a name→note map entry) so future per-group attributes — colour,
// icon, display order, etc. — can be added without a shape change.
type CustomRuleGroup struct {
	Name    string `json:"name" bson:"name" validate:"required,max=64"`
	Comment string `json:"comment,omitempty" bson:"comment,omitempty" validate:"omitempty,max=80"`
}

// CustomRuleGroups is the per-list group registry: groups are scoped to the
// denylist (Block) or allowlist (Allow), so the same name in each list is
// independent. Organizational metadata only; never synced to the proxy.
type CustomRuleGroups struct {
	Block []CustomRuleGroup `json:"block,omitempty" bson:"block,omitempty" validate:"omitempty,max=100,dive"`
	Allow []CustomRuleGroup `json:"allow,omitempty" bson:"allow,omitempty" validate:"omitempty,max=100,dive"`
}

// list returns the group slice for an action ("" if the action is unknown).
func (g *CustomRuleGroups) list(action string) []CustomRuleGroup {
	switch action {
	case ACTION_BLOCK:
		return g.Block
	case ACTION_ALLOW:
		return g.Allow
	}
	return nil
}

// assign writes the group slice back for an action, normalizing empty to nil so
// the stored/serialized shape stays clean (omitempty drops it).
func (g *CustomRuleGroups) assign(action string, list []CustomRuleGroup) {
	if len(list) == 0 {
		list = nil
	}
	switch action {
	case ACTION_BLOCK:
		g.Block = list
	case ACTION_ALLOW:
		g.Allow = list
	}
}

// Upsert sets a group's comment in the given list, creating the group if absent.
func (g *CustomRuleGroups) Upsert(action, name, comment string) {
	list := g.list(action)
	for i := range list {
		if list[i].Name == name {
			list[i].Comment = comment
			g.assign(action, list)
			return
		}
	}
	g.assign(action, append(list, CustomRuleGroup{Name: name, Comment: comment}))
}

// Remove drops a group from the given list (its rules are reassigned separately).
func (g *CustomRuleGroups) Remove(action, name string) {
	list := g.list(action)
	out := make([]CustomRuleGroup, 0, len(list))
	for _, grp := range list {
		if grp.Name != name {
			out = append(out, grp)
		}
	}
	g.assign(action, out)
}

// Rename renames `from`→`to` in the given list. If `to` already exists the move
// merges into it (its comment is kept); otherwise `from`'s comment is carried over.
func (g *CustomRuleGroups) Rename(action, from, to string) {
	if from == to {
		return
	}
	list := g.list(action)
	var fromComment string
	hadFrom, hasTo := false, false
	out := make([]CustomRuleGroup, 0, len(list))
	for _, grp := range list {
		switch grp.Name {
		case from:
			fromComment, hadFrom = grp.Comment, true
		case to:
			hasTo = true
			out = append(out, grp)
		default:
			out = append(out, grp)
		}
	}
	if !hasTo {
		comment := ""
		if hadFrom {
			comment = fromComment
		}
		out = append(out, CustomRuleGroup{Name: to, Comment: comment})
	}
	g.assign(action, out)
}

// Clone returns a deep copy so service ops can mutate without touching the loaded profile.
func (g CustomRuleGroups) Clone() CustomRuleGroups {
	cp := CustomRuleGroups{}
	if g.Block != nil {
		cp.Block = append([]CustomRuleGroup(nil), g.Block...)
	}
	if g.Allow != nil {
		cp.Allow = append([]CustomRuleGroup(nil), g.Allow...)
	}
	return cp
}

// CustomRule represents a custom rule.
//
// Note, Group and Order are organizational metadata used only by the API and
// frontend. They carry `redis:"-"` so they never reach the proxy hash, which
// reads exactly {action, value, syntax}. Order is a dense per-profile display
// index (0..N-1); it does NOT affect filtering precedence (precedence stays
// action-based in the proxy).
type CustomRule struct {
	ID     primitive.ObjectID `json:"id" bson:"_id" redis:"-" binding:"required"`
	Action CustomRuleAction   `json:"action" bson:"action" redis:"action" binding:"required"`
	Value  string             `json:"value" bson:"value" redis:"value" binding:"required"`
	Syntax CustomRuleSyntax   `json:"-" bson:"syntax" redis:"syntax" binding:"required"`
	Note   string             `json:"note" bson:"note" redis:"-"`
	Group  string             `json:"group" bson:"group" redis:"-"`
	Order  int                `json:"order" bson:"order" redis:"-"`
}

// CustomRuleAction represents a custom rule action type
type CustomRuleAction string

func (p CustomRuleAction) MarshalBinary() (data []byte, err error) {
	return fmt.Append(nil, p), nil
}

func NewCustomRuleAction(action string) (CustomRuleAction, error) {
	switch action {
	case ACTION_BLOCK:
		return ACTION_BLOCK, nil
	case ACTION_ALLOW:
		return ACTION_ALLOW, nil
	case ACTION_COMMENT:
		return ACTION_COMMENT, nil
	default:
		return "", ErrInvalidCustomRuleAction
	}
}

// CustomRuleSyntax represents a custom rule action syntax
type CustomRuleSyntax string

func (p CustomRuleSyntax) MarshalBinary() (data []byte, err error) {
	return []byte(fmt.Sprint(p)), nil
}

var (
	validations = []string{"fqdn", "ip4_addr", "ip6_addr", "fqdn_wildcard", "asn"}
)

func NewCustomRuleSyntax(vldtr *validator.Validate, value string) (CustomRuleSyntax, error) {
	for _, validation := range validations {
		if err := vldtr.Var(value, validation); err == nil {
			return CustomRuleSyntax(validation), nil
		}
	}
	return SYNTAX_UNKNOWN, ErrInvalidCustomRuleSyntax
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
