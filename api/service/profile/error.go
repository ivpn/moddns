package profile

import (
	"errors"
	"fmt"

	"github.com/ivpn/dns/api/model"
)

var (
	ErrProfileNameEmpty         = errors.New("profile name cannot be empty")
	ErrFailedToDeleteProfile    = errors.New("failed to delete profile")
	ErrProfileNameAlreadyExists = errors.New("profile with this name already exists")
	ErrProfileNameCannotBeEmpty = errors.New("profile name cannot be empty")
	ErrDefaultRuleInvalid       = errors.New("default rule action is invalid. Allowed values: block, allow")
	ErrBlockSubdomainsInvalid        = errors.New("block subdomains value is invalid. Allowed values: block, allow")
	ErrCustomRulesSubdomainsInvalid = errors.New("custom_rules_subdomains value is invalid. Allowed values: include, exact")
	ErrBlocklistNotFound        = errors.New("blocklist not found")
	ErrBlocklistAlreadyEnabled  = errors.New("blocklist already enabled")
	ErrInvalidBlocklistValue    = errors.New("invalid blocklist value")
	ErrCustomRuleAlreadyExists  = errors.New("custom rule already exists")
	ErrLastProfileInAccount     = errors.New("cannot delete the last profile in the account")
	ErrRecursorInvalid          = fmt.Errorf("recursor value is invalid. Allowed values: %v", model.RECURSORS)
	ErrMaxProfilesLimitReached  = errors.New("maximum number of profiles reached")
	ErrQueryLogsRateLimited     = errors.New("query logs rate limited")
	ErrServiceNotFound          = errors.New("service not found")
	ErrServiceAlreadyEnabled    = errors.New("service already enabled")
	ErrInvalidServiceValue      = errors.New("invalid service value")
)
