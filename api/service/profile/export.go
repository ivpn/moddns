package profile

import (
	"context"
	"errors"
	"time"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// ExportEnvelope is the top-level shape of a modDNS profile export.
// Mirrors the schema defined in docs/specs/account-export-import-behaviour.md
// (Section F).
//
// The export file carries no embedded warning text. The export UI displays a
// sensitivity warning at the download point instead, keeping the import-side
// DisallowUnknownFields() allowlist strict.
type ExportEnvelope struct {
	SchemaVersion int               `json:"schemaVersion"`
	Kind          string            `json:"kind"`
	ExportedAt    time.Time         `json:"exportedAt"`
	ExportedFrom  *ExportedFromInfo `json:"exportedFrom,omitempty"`
	Profiles      []ExportedProfile `json:"profiles"`
}

type ExportedFromInfo struct {
	Service    string `json:"service,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
}

type ExportedProfile struct {
	Name     string            `json:"name"`
	Comment  string            `json:"comment,omitempty"`
	Settings *ExportedSettings `json:"settings"`
}

type ExportedSettings struct {
	Privacy     *ExportedPrivacy     `json:"privacy,omitempty"`
	Security    *ExportedSecurity    `json:"security,omitempty"`
	CustomRules []ExportedCustomRule `json:"customRules,omitempty"`
	Logs        *ExportedLogs        `json:"logs,omitempty"`
	Statistics  *ExportedStatistics  `json:"statistics,omitempty"`
	Advanced    *ExportedAdvanced    `json:"advanced,omitempty"`
}

type ExportedPrivacy struct {
	Blocklists                []string `json:"blocklists,omitempty"`
	Services                  []string `json:"services,omitempty"`
	DefaultRule               string   `json:"defaultRule,omitempty"`
	BlocklistsSubdomainsRule  string   `json:"blocklistsSubdomainsRule,omitempty"`
	CustomRulesSubdomainsRule string   `json:"customRulesSubdomainsRule,omitempty"`
}

type ExportedSecurity struct {
	DNSSEC *ExportedDNSSEC `json:"dnssec,omitempty"`
}

type ExportedDNSSEC struct {
	Enabled   bool `json:"enabled"`
	SendDoBit bool `json:"sendDoBit"`
}

type ExportedCustomRule struct {
	Action  string `json:"action"`
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"`
}

type ExportedLogs struct {
	Enabled       bool   `json:"enabled"`
	LogClientsIPs bool   `json:"logClientsIPs"`
	LogDomains    bool   `json:"logDomains"`
	Retention     string `json:"retention,omitempty"`
}

type ExportedStatistics struct {
	Enabled bool `json:"enabled"`
}

type ExportedAdvanced struct {
	Recursor string `json:"recursor,omitempty"`
}

// ExportScope enumerates the supported selection scopes for Export.
// Mirrors spec rows E5-E8.
const (
	ExportScopeAll      = "all"
	ExportScopeSelected = "selected"
)

// Export produces a profile export envelope for the caller.
//
// Scope semantics (spec rows E5-E11):
//   - scope=ExportScopeAll      -> profileIds must be empty; export every profile owned by accountId
//   - scope=ExportScopeSelected -> profileIds must be non-empty; export those profiles after ownership check
//
// Reauth is verified inline before any other work.
// specRef: M4, M5, M6, E3, E5, E7, E9, E10
func (p *ProfileService) Export(
	ctx context.Context,
	accountId, scope string,
	profileIds []string,
	currentPassword, reauthToken *string,
) (*ExportEnvelope, error) {
	if err := p.verifyReauth(ctx, accountId, auth.TokenTypeReauthProfileExport, currentPassword, reauthToken); err != nil {
		return nil, err
	}

	profiles, err := p.enumerateExportProfiles(ctx, accountId, scope, profileIds)
	if err != nil {
		return nil, err
	}

	exportedProfiles := make([]ExportedProfile, 0, len(profiles))
	for i := range profiles {
		exportedProfiles = append(exportedProfiles, exportProfile(&profiles[i]))
	}

	envelope := &ExportEnvelope{
		SchemaVersion: 1,
		Kind:          "moddns-export",
		ExportedAt:    time.Now().UTC(),
		ExportedFrom: &ExportedFromInfo{
			Service: "modDNS",
			// TODO: wire app version when available (no Version field on ServerConfig or ServiceConfig today)
			AppVersion: "",
		},
		Profiles: exportedProfiles,
	}

	return envelope, nil
}

// enumerateExportProfiles fetches the profiles to be exported based on the requested scope.
// specRef: E5, E7, E9, E10, E11
func (p *ProfileService) enumerateExportProfiles(
	ctx context.Context,
	accountId, scope string,
	profileIds []string,
) ([]model.Profile, error) {
	switch scope {
	case ExportScopeAll:
		// specRef: E5 — export every profile owned by this account
		profiles, err := p.GetProfiles(ctx, accountId)
		if err != nil {
			return nil, err
		}
		return profiles, nil

	case ExportScopeSelected:
		// specRef: E10 — cap the selection at the configured maximum
		if len(profileIds) > p.ServiceConfig.MaxProfiles {
			return nil, ErrTooManyProfileIds
		}

		// specRef: E7, E9 — validate ownership of every requested profile ID
		profiles := make([]model.Profile, 0, len(profileIds))
		for _, profileId := range profileIds {
			profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
			if err != nil {
				if errors.Is(err, dbErrors.ErrProfileNotFound) {
					// specRef: E9 — do not distinguish "other account's profile" from "does not exist"
					return nil, dbErrors.ErrProfileNotFound
				}
				return nil, err
			}
			profiles = append(profiles, *profile)
		}
		return profiles, nil

	default:
		// specRef: E11 — unknown scope values are rejected
		// The DTO validator rejects these before reaching the service, but we
		// defend in depth here so the function never silently returns garbage.
		return nil, ErrInvalidExportScope
	}
}

// exportProfile transforms a model.Profile into an ExportedProfile envelope entry.
// Internal identifiers (ID, ProfileId, AccountId) are deliberately omitted.
// specRef: F1-F9
func exportProfile(p *model.Profile) ExportedProfile {
	ep := ExportedProfile{
		Name:     p.Name,
		Settings: exportSettings(p.Settings),
	}
	return ep
}

// exportSettings builds the ExportedSettings from a ProfileSettings record.
// A nil Settings pointer results in an empty (but non-nil) ExportedSettings so
// that the envelope is always structurally valid.
func exportSettings(s *model.ProfileSettings) *ExportedSettings {
	es := &ExportedSettings{}

	if s == nil {
		return es
	}

	// Privacy section — specRef: F1, F2, F3, F4, F5
	if s.Privacy != nil {
		es.Privacy = &ExportedPrivacy{
			DefaultRule:               s.Privacy.DefaultRule,
			BlocklistsSubdomainsRule:  s.Privacy.BlocklistsSubdomainsRule,
			CustomRulesSubdomainsRule: s.Privacy.CustomRulesSubdomainsRule,
			Blocklists:                s.Privacy.Blocklists,
			Services:                  s.Privacy.Services,
		}
	}

	// Security section — specRef: F6
	if s.Security != nil {
		es.Security = &ExportedSecurity{
			DNSSEC: &ExportedDNSSEC{
				Enabled:   s.Security.DNSSECSettings.Enabled,
				SendDoBit: s.Security.DNSSECSettings.SendDoBit,
			},
		}
	}

	// Custom rules — specRef: F7; internal ID field stripped per F9
	if len(s.CustomRules) > 0 {
		rules := make([]ExportedCustomRule, 0, len(s.CustomRules))
		for _, r := range s.CustomRules {
			if r == nil {
				continue
			}
			rules = append(rules, ExportedCustomRule{
				Action: string(r.Action),
				Value:  r.Value,
				// Comment and AddedAt are not present in the model today — omit.
			})
		}
		es.CustomRules = rules
	}

	// Logs section — specRef: F3 (logs sub-fields)
	if s.Logs != nil {
		es.Logs = &ExportedLogs{
			Enabled:       s.Logs.Enabled,
			LogClientsIPs: s.Logs.LogClientsIPs,
			LogDomains:    s.Logs.LogDomains,
			Retention:     string(s.Logs.Retention),
		}
	}

	// Statistics section
	if s.Statistics != nil {
		es.Statistics = &ExportedStatistics{
			Enabled: s.Statistics.Enabled,
		}
	}

	// Advanced section — specRef: F7 (recursor)
	if s.Advanced != nil {
		es.Advanced = &ExportedAdvanced{
			Recursor: s.Advanced.Recursor,
		}
	}

	return es
}
