package model

import "time"

// This file holds the wire/file shape of the modDNS profile export envelope.
// Unlike most types in package `model`, ExportEnvelope and its nested types
// are NOT storage records — they carry no BSON or Redis tags and exist only
// to marshal/unmarshal the JSON file that POST /api/v1/profiles/export
// emits and POST /api/v1/profiles/import accepts.
//
// They live in `model` (not `service/profile`) so the DTO layer in
// `api/api/requests/` can reference them without pulling in a service
// dependency; this matches the precedent set by `requests.ProfileUpdates`
// which uses `[]model.ProfileUpdate`.
//
// Schema reference: docs/specs/account-export-import-behaviour.md (Section F).

// ExportEnvelope is the top-level shape of a modDNS profile export.
//
// The same type is used in two directions:
//   - Outbound (export): the service marshals it into the downloadable file.
//   - Inbound (import): the HTTP handler decodes the request body's `payload`
//     field into it, and s.Validator.ValidateRequest runs the `validate:` tags
//     recursively. Validation tags are inert during marshaling.
//
// The export file carries no embedded warning text. The export UI displays a
// sensitivity warning at the download point instead, keeping the import-side
// DisallowUnknownFields() allowlist strict.
//
// specRef: V1–V6, F1–F9
type ExportEnvelope struct {
	SchemaVersion int               `json:"schemaVersion" validate:"required,eq=1"`
	Kind          string            `json:"kind"          validate:"required,eq=moddns-export"`
	ExportedAt    time.Time         `json:"exportedAt"    validate:"required"`
	ExportedFrom  *ExportedFromInfo `json:"exportedFrom,omitempty"`
	Profiles      []ExportedProfile `json:"profiles"      validate:"required,min=1,max=100,dive"`
}

// ExportedFromInfo carries informational metadata about the exporting service.
// The contents are not trusted and not validated for semantics.
// specRef: V4
type ExportedFromInfo struct {
	Service    string `json:"service,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
}

// ExportedProfile represents a single profile in the envelope.
// specRef: V7–V15, F1–F9
type ExportedProfile struct {
	// Name of the profile. Names longer than 50 characters are truncated on
	// import (with a warning) rather than rejected, so the wire limit is 200
	// while the persisted profile name is capped at 50.
	Name     string            `json:"name"              validate:"required,max=200,safe_name"`
	Comment  string            `json:"comment,omitempty" validate:"omitempty,max=200,safe_name"`
	Settings *ExportedSettings `json:"settings"          validate:"required"`
}

type ExportedSettings struct {
	Privacy  *ExportedPrivacy  `json:"privacy,omitempty"`
	Security *ExportedSecurity `json:"security,omitempty"`
	// CustomRules holds the profile's custom filtering rules, capped at 1000 per profile.
	CustomRules []ExportedCustomRule `json:"customRules,omitempty" validate:"max=1000,dive"`
	// CustomRuleGroups maps a custom-rule group name to its optional note. Purely
	// organizational metadata that round-trips with the rules' `group` field.
	CustomRuleGroups map[string]string   `json:"customRuleGroups,omitempty" validate:"omitempty,max=1000"`
	Logs             *ExportedLogs       `json:"logs,omitempty"`
	Statistics       *ExportedStatistics `json:"statistics,omitempty"`
	Advanced         *ExportedAdvanced   `json:"advanced,omitempty"`
}

// ExportedPrivacy carries the privacy section of a profile.
// specRef: V8, V9, F1–F4
type ExportedPrivacy struct {
	Blocklists                []string `json:"blocklists,omitempty"                validate:"max=100,dive,required,max=64"`
	Services                  []string `json:"services,omitempty"                  validate:"max=100,dive,required,max=64"`
	DefaultRule               string   `json:"defaultRule,omitempty"               validate:"omitempty,oneof=block allow"`
	BlocklistsSubdomainsRule  string   `json:"blocklistsSubdomainsRule,omitempty"  validate:"omitempty,oneof=block allow"`
	CustomRulesSubdomainsRule string   `json:"customRulesSubdomainsRule,omitempty" validate:"omitempty,oneof=include exact"`
}

// ExportedSecurity carries the security section of a profile.
// specRef: F5
type ExportedSecurity struct {
	DNSSEC *ExportedDNSSEC `json:"dnssec,omitempty"`
}

// ExportedDNSSEC carries DNSSEC settings.
// specRef: F5
type ExportedDNSSEC struct {
	Enabled   bool `json:"enabled"`
	SendDoBit bool `json:"sendDoBit"`
}

// ExportedCustomRule represents a single user-authored filtering rule.
// Note: addedAt is not in v1 -- CustomRule model has no timestamp field.
// The rule's display `order` is intentionally NOT exported: it is positional and
// re-derived from the array index on import.
// specRef: V10–V14, F4
type ExportedCustomRule struct {
	Action string `json:"action"          validate:"required,oneof=block allow comment"`
	Value  string `json:"value"           validate:"required,max=255"`
	// Note is a free-text annotation. Free text (not safe_name) so users can write
	// arbitrary reminders; length-capped to match the model/PATCH validators.
	Note string `json:"note,omitempty"  validate:"omitempty,max=280"`
	// Group is the optional organizational label this rule belongs to.
	Group string `json:"group,omitempty" validate:"omitempty,max=64"`
}

// ExportedLogs carries the log settings for a profile.
// specRef: F6
type ExportedLogs struct {
	Enabled       bool   `json:"enabled"`
	LogClientsIPs bool   `json:"logClientsIPs"`
	LogDomains    bool   `json:"logDomains"`
	Retention     string `json:"retention,omitempty" validate:"omitempty,oneof=1h 6h 1d 1w 1m"`
}

// ExportedStatistics carries the statistics toggle for a profile.
// specRef: F6
type ExportedStatistics struct {
	Enabled bool `json:"enabled"`
}

// ExportedAdvanced carries advanced settings for a profile.
// Note: emission is suppressed by the export service (recursor is a
// staging-only control); this type stays so the DTO can tolerate-and-ignore
// the field for forward-compat with hand-edited files. See spec row F7.
// specRef: F7
type ExportedAdvanced struct {
	Recursor string `json:"recursor,omitempty" validate:"omitempty,oneof=sdns unbound"`
}
