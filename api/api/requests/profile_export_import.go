package requests

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidScopeSelection is returned when the scope and profileIds fields are
// mutually inconsistent (spec rows E6, E8).
var ErrInvalidScopeSelection = errors.New("profileIds must be empty when scope=all")

// ExportRequest is the request body for POST /api/v1/profiles/export.
// Exactly one of CurrentPassword or ReauthToken must be provided (spec rows E3, M4).
// specRef: E2, E5–E11
type ExportRequest struct {
	Scope           string   `json:"scope"                        validate:"required,oneof=all selected"`
	ProfileIds      []string `json:"profileIds,omitempty"`
	CurrentPassword *string  `json:"current_password,omitempty"   validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string  `json:"reauth_token,omitempty"       validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}

// Validate enforces the cross-field invariant between Scope and ProfileIds.
// specRef: E6, E8
func (r *ExportRequest) Validate() error {
	if r.Scope == "all" && len(r.ProfileIds) > 0 {
		return fmt.Errorf("scope=all: %w", ErrInvalidScopeSelection)
	}
	if r.Scope == "selected" && len(r.ProfileIds) == 0 {
		return fmt.Errorf("scope=selected requires at least one profileId: %w", ErrInvalidScopeSelection)
	}
	return nil
}

// ImportRequest is the request body for POST /api/v1/profiles/import.
// Exactly one of CurrentPassword or ReauthToken must be provided (spec rows I2, M4).
// specRef: I1, I8–I11
type ImportRequest struct {
	Mode            string         `json:"mode"                         validate:"required,oneof=create_new"`
	Payload         *ImportPayload `json:"payload"                      validate:"required"`
	CurrentPassword *string        `json:"current_password,omitempty"   validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string        `json:"reauth_token,omitempty"       validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}

// ImportPayload is the top-level envelope of the export file embedded in the
// import request body.
// specRef: V1–V6
type ImportPayload struct {
	SchemaVersion int               `json:"schemaVersion" validate:"required,eq=1"`
	Kind          string            `json:"kind"          validate:"required,eq=moddns-export"`
	ExportedAt    time.Time         `json:"exportedAt"    validate:"required"`
	ExportedFrom  *ExportedFromInfo `json:"exportedFrom,omitempty"`
	Profiles      []ImportProfile   `json:"profiles"      validate:"required,min=1,max=10,dive"`
}

// ExportedFromInfo carries informational metadata about the exporting service.
// The contents are not trusted and not validated for semantics.
// specRef: V4
type ExportedFromInfo struct {
	Service    string `json:"service,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
}

// ImportProfile represents a single profile in the import payload.
// specRef: V7–V15
type ImportProfile struct {
	Name     string          `json:"name"              validate:"required,max=50,safe_name"`
	Comment  string          `json:"comment,omitempty" validate:"omitempty,max=200,safe_name"`
	Settings *ImportSettings `json:"settings"          validate:"required"`
}

// ImportSettings groups all per-profile setting sections.
type ImportSettings struct {
	Privacy     *ImportPrivacy     `json:"privacy,omitempty"`
	Security    *ImportSecurity    `json:"security,omitempty"`
	CustomRules []ImportCustomRule `json:"customRules,omitempty" validate:"max=10000,dive"`
	Logs        *ImportLogs        `json:"logs,omitempty"`
	Statistics  *ImportStatistics  `json:"statistics,omitempty"`
	Advanced    *ImportAdvanced    `json:"advanced,omitempty"`
}

// ImportPrivacy carries the privacy section of a profile.
// specRef: V8, V9
type ImportPrivacy struct {
	Blocklists                []string `json:"blocklists,omitempty"               validate:"max=100,dive,required,max=64"`
	Services                  []string `json:"services,omitempty"                 validate:"max=100,dive,required,max=64"`
	DefaultRule               string   `json:"defaultRule,omitempty"              validate:"omitempty,oneof=block allow"`
	BlocklistsSubdomainsRule  string   `json:"blocklistsSubdomainsRule,omitempty" validate:"omitempty,oneof=block allow"`
	CustomRulesSubdomainsRule string   `json:"customRulesSubdomainsRule,omitempty" validate:"omitempty,oneof=include exact"`
}

// ImportSecurity carries the security section of a profile.
// specRef: F5
type ImportSecurity struct {
	DNSSEC *ImportDNSSEC `json:"dnssec,omitempty"`
}

// ImportDNSSEC carries DNSSEC settings.
// specRef: F5
type ImportDNSSEC struct {
	Enabled   bool `json:"enabled"`
	SendDoBit bool `json:"sendDoBit"`
}

// ImportCustomRule represents a single user-authored filtering rule.
// Note: addedAt is not in v1 -- CustomRule model has no timestamp field.
// specRef: V10–V14, F4
type ImportCustomRule struct {
	Action  string `json:"action"            validate:"required,oneof=block allow comment"`
	Value   string `json:"value"             validate:"required,max=255"`
	Comment string `json:"comment,omitempty" validate:"omitempty,max=200,safe_name"`
}

// ImportLogs carries the log settings for a profile.
// specRef: F6
type ImportLogs struct {
	Enabled       bool   `json:"enabled"`
	LogClientsIPs bool   `json:"logClientsIPs"`
	LogDomains    bool   `json:"logDomains"`
	Retention     string `json:"retention,omitempty" validate:"omitempty,oneof=1h 6h 1d 1w 1m"`
}

// ImportStatistics carries the statistics toggle for a profile.
// specRef: F6
type ImportStatistics struct {
	Enabled bool `json:"enabled"`
}

// ImportAdvanced carries advanced settings for a profile.
// specRef: F7
type ImportAdvanced struct {
	Recursor string `json:"recursor,omitempty" validate:"omitempty,oneof=sdns unbound"`
}
