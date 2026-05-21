package profile

import (
	"errors"
	"fmt"

	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
)

// ProfileError tags an error as a client-class profile-service error. Mirrors
// the *PasskeyError pattern: any sentinel built via NewProfileError is mapped
// to HTTP 400 by api/api/errors.go via errors.As, so wrapped instances (e.g.
// fmt.Errorf("%w: ...", ErrMaxProfilesExceeded, ...)) are also caught.
//
// Use this type for sentinels that represent a client-fixable problem with the
// request — bad schema version, oversized batch, unsupported mode, etc. — and
// must surface as 400 with a typed error body. Don't use it for server-class
// failures (DB outages, internal panics) which should fall through to the
// default 500 mapping.
type ProfileError struct {
	Code    string
	Message error
}

// Error implements the error interface.
func (e *ProfileError) Error() string {
	return e.Message.Error()
}

// NewProfileError returns a new client-class profile-service error.
func NewProfileError(msg string) *ProfileError {
	return &ProfileError{
		Code:    "PROFILE_ERROR",
		Message: errors.New(msg),
	}
}

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

	// ErrReauthRequired aliases reauth.ErrMissingAuthMethod for legacy callers.
	// New code should reference reauth.ErrMissingAuthMethod directly.
	// specRef: M5
	ErrReauthRequired = reauth.ErrMissingAuthMethod

	// ErrReauthInvalid is the legacy conflated sentinel that the deleted
	// ProfileService.verifyReauth used to return for both wrong password and
	// invalid/expired tokens. Kept for backward compatibility with any caller
	// that errors.Is against it; new code uses reauth.ErrInvalidPassword /
	// reauth.ErrInvalidReauthToken / reauth.ErrReauthTokenExpired.
	// specRef: M6
	//
	// Deprecated: prefer the specific sentinel from package reauth.
	ErrReauthInvalid = errors.New("invalid reauth credential")

	// The seven export/import client-class sentinels below are typed as
	// *ProfileError so HandleError maps them to HTTP 400 via the errors.As
	// arm. The `errors.Is` semantics are preserved (pointer-equality on the
	// singleton sentinel, and errors.Is walks the wrap chain for
	// ErrMaxProfilesExceeded's fmt.Errorf("%w: ...", ...) augmentation).

	// ErrTooManyProfileIds is returned when the selected profile ID list exceeds MAX_PROFILES.
	// specRef: E10
	ErrTooManyProfileIds = NewProfileError("too many profile IDs requested")

	// ErrInvalidExportScope is returned when the scope field is not a recognised value.
	// specRef: E11
	ErrInvalidExportScope = NewProfileError("invalid export scope")

	// ErrUnsupportedImportMode is returned when mode is not ImportModeCreateNew.
	// specRef: I8, I9
	ErrUnsupportedImportMode = NewProfileError("unsupported import mode")

	// ErrUnsupportedSchemaVersion is returned when schemaVersion != 1.
	// specRef: V1
	ErrUnsupportedSchemaVersion = NewProfileError("unsupported schema version")

	// ErrInvalidExportKind is returned when kind != "moddns-export".
	// specRef: V2
	ErrInvalidExportKind = NewProfileError("invalid export kind")

	// ErrEmptyImportPayload is returned when the profiles array is empty.
	// specRef: V5
	ErrEmptyImportPayload = NewProfileError("import payload contains no profiles")

	// ErrMaxProfilesExceeded is returned when importing would push the account
	// over the ServiceConfig.MaxProfiles cap.
	// specRef: I16, I17, I18
	ErrMaxProfilesExceeded = NewProfileError("import would exceed maximum profile limit")
)
