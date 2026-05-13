package profile

import (
	"errors"
	"fmt"

	"github.com/ivpn/dns/api/model"
)

var (
	ErrProfileNameEmpty             = errors.New("profile name cannot be empty")
	ErrProfileNameInvalid           = errors.New("profile name contains invalid characters")
	ErrFailedToDeleteProfile        = errors.New("failed to delete profile")
	ErrProfileNameAlreadyExists     = errors.New("profile with this name already exists")
	ErrProfileNameCannotBeEmpty     = errors.New("profile name cannot be empty")
	ErrDefaultRuleInvalid           = errors.New("default rule action is invalid. Allowed values: block, allow")
	ErrBlocklistsSubdomainsInvalid  = errors.New("blocklists_subdomains_rule value is invalid. Allowed values: block, allow")
	ErrCustomRulesSubdomainsInvalid = errors.New("custom_rules_subdomains_rule value is invalid. Allowed values: include, exact")
	ErrBlocklistNotFound            = errors.New("blocklist not found")
	ErrBlocklistAlreadyEnabled      = errors.New("blocklist already enabled")
	ErrInvalidBlocklistValue        = errors.New("invalid blocklist value")
	ErrCustomRuleAlreadyExists      = errors.New("custom rule already exists")
	ErrLastProfileInAccount         = errors.New("cannot delete the last profile in the account")
	ErrRecursorInvalid              = fmt.Errorf("recursor value is invalid. Allowed values: %v", model.RECURSORS)
	ErrMaxProfilesLimitReached      = errors.New("maximum number of profiles reached")
	ErrQueryLogsRateLimited         = errors.New("query logs rate limited")
	ErrServiceNotFound              = errors.New("service not found")
	ErrServiceAlreadyEnabled        = errors.New("service already enabled")
	ErrInvalidServiceValue          = errors.New("invalid service value")

	// ErrReauthRequired is returned when neither currentPassword nor reauthToken is provided.
	// specRef: M5
	ErrReauthRequired = errors.New("missing current_password or reauth_token")

	// ErrReauthInvalid is returned when the provided credential does not match the account record.
	// specRef: M6
	ErrReauthInvalid = errors.New("invalid reauth credential")

	// ErrTooManyProfileIds is returned when the selected profile ID list exceeds MAX_PROFILES.
	// specRef: E10
	ErrTooManyProfileIds = errors.New("too many profile IDs requested")

	// ErrInvalidExportScope is returned when the scope field is not a recognised value.
	// specRef: E11
	ErrInvalidExportScope = errors.New("invalid export scope")

	// ErrUnsupportedImportMode is returned when mode is not ImportModeCreateNew.
	// specRef: I8, I9
	ErrUnsupportedImportMode = errors.New("unsupported import mode")

	// ErrUnsupportedSchemaVersion is returned when schemaVersion != 1.
	// specRef: V1
	ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")

	// ErrInvalidExportKind is returned when kind != "moddns-export".
	// specRef: V2
	ErrInvalidExportKind = errors.New("invalid export kind")

	// ErrEmptyImportPayload is returned when the profiles array is empty.
	// specRef: V5
	ErrEmptyImportPayload = errors.New("import payload contains no profiles")

	// ErrMaxProfilesExceeded is returned when importing would push the account
	// over the ServiceConfig.MaxProfiles cap.
	// specRef: I16, I17, I18
	ErrMaxProfilesExceeded = errors.New("import would exceed maximum profile limit")
)
