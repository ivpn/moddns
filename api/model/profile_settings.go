package model

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ACTION_BLOCK         = "block"
	ACTION_ALLOW         = "allow"
	ACTION_COMMENT       = "comment"
	DEFAULT_RULE_BLOCK   = ACTION_BLOCK
	DEFAULT_RULE_ALLOW   = ACTION_ALLOW
	SYNTAX_IPV4          = "ip4_addr"
	SYNTAX_IPV4_WILDCARD = "ip4_wildcard"
	SYNTAX_IPV4_CIDR     = "ip4_cidr"
	SYNTAX_IPV6          = "ip6"
	SYNTAX_IPV6_WILDCARD = "ip6_wildcard"
	SYNTAX_IPV6_CIDR     = "ip6_cidr"
	SYNTAX_FQDN          = "fqdn"
	SYNTAX_FQDN_WILDCARD = "fqdn_wildcard"
	SYNTAX_ASN           = "asn"
	SYNTAX_UNKNOWN       = "unknown_syntax"
)

var (
	ErrInvalidCustomRuleAction = errors.New("invalid custom rule action type")
	ErrInvalidCustomRuleSyntax = errors.New("invalid custom rule value syntax")
)

// ProfileSettings represents profile settings, it's internal model used in `profiles` collection
type ProfileSettings struct {
	ProfileId   string              `json:"profile_id" bson:"profile_id" redis:"profile_id" binding:"required"`
	Security    *Security           `json:"security" bson:"security" redis:"security" binding:"required"`
	Privacy     *Privacy            `json:"privacy" bson:"privacy" redis:"privacy" binding:"required"`
	CustomRules []*CustomRule       `json:"custom_rules" bson:"custom_rules" redis:"-"`
	Logs        *LogsSettings       `json:"logs" bson:"logs" redis:"-" binding:"required"`
	Statistics  *StatisticsSettings `json:"statistics" bson:"statistics" redis:"-" binding:"required"`
	Advanced    *Advanced           `json:"advanced" bson:"advanced" redis:"advanced" binding:"required"`
}

// NewSettings creates a new, empty settings object
func NewSettings() *ProfileSettings {
	return &ProfileSettings{
		Privacy: &Privacy{
			Blocklists: make([]string, 0),
			Services: &ServicesSettings{
				Blocked: make([]string, 0),
			},
			DefaultRule:    DEFAULT_RULE_ALLOW,
			SubdomainsRule: ACTION_BLOCK,
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
		CustomRules: make([]*CustomRule, 0),
		Advanced: &Advanced{
			Recursor: RECURSOR_DEFAULT,
		},
	}
}

// StatisticsSettings represents statistics/analytics settings
type StatisticsSettings struct {
	Enabled bool `json:"enabled" bson:"enabled" redis:"enabled" binding:"required"`
}

// CustomRule represents a custom rule
type CustomRule struct {
	ID     primitive.ObjectID `json:"id" bson:"_id" redis:"-" binding:"required"`
	Action CustomRuleAction   `json:"action" bson:"action" redis:"action" binding:"required"`
	Value  string             `json:"value" bson:"value" redis:"value" binding:"required"`
	Syntax CustomRuleSyntax   `json:"-" bson:"syntax" redis:"syntax" binding:"required"`
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
