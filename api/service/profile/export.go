package profile

import (
	"context"
	"errors"
	"sort"
	"time"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/version"
	"github.com/ivpn/dns/api/model"
)

// The envelope types (ExportEnvelope, ExportedProfile, …) live in package
// `model` (see api/model/export.go) so the DTO layer in api/api/requests/
// can reference them without depending on this service package. Use the
// model.* prefix throughout this file.

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
	mfa *model.MfaData,
) (*model.ExportEnvelope, error) {
	if _, err := reauth.Verify(ctx, p.AccountRepository, p.MfaVerifier, reauth.Params{
		AccountId:        accountId,
		TokenType:        auth.TokenTypeReauthProfileExport,
		Password:         currentPassword,
		ReauthToken:      reauthToken,
		Mfa:              mfa,
		PersistOnConsume: true,
	}); err != nil {
		return nil, err
	}

	profiles, err := p.enumerateExportProfiles(ctx, accountId, scope, profileIds)
	if err != nil {
		return nil, err
	}

	exportedProfiles := make([]model.ExportedProfile, 0, len(profiles))
	for i := range profiles {
		exportedProfiles = append(exportedProfiles, exportProfile(&profiles[i]))
	}

	envelope := &model.ExportEnvelope{
		SchemaVersion: 1,
		Kind:          "moddns-export",
		ExportedAt:    time.Now().UTC(),
		ExportedFrom: &model.ExportedFromInfo{
			Service:    "modDNS",
			AppVersion: version.Version,
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
func exportProfile(p *model.Profile) model.ExportedProfile {
	ep := model.ExportedProfile{
		Name:     p.Name,
		Settings: exportSettings(p.Settings),
	}
	return ep
}

// exportSettings builds the ExportedSettings from a ProfileSettings record.
// A nil Settings pointer results in an empty (but non-nil) ExportedSettings so
// that the envelope is always structurally valid.
func exportSettings(s *model.ProfileSettings) *model.ExportedSettings {
	es := &model.ExportedSettings{}

	if s == nil {
		return es
	}

	// Privacy section — specRef: F1, F2, F3, F4, F5
	if s.Privacy != nil {
		es.Privacy = &model.ExportedPrivacy{
			DefaultRule:               s.Privacy.DefaultRule,
			BlocklistsSubdomainsRule:  s.Privacy.BlocklistsSubdomainsRule,
			CustomRulesSubdomainsRule: s.Privacy.CustomRulesSubdomainsRule,
			Blocklists:                s.Privacy.Blocklists,
			Services:                  s.Privacy.Services,
		}
	}

	// Security section — specRef: F6
	if s.Security != nil {
		es.Security = &model.ExportedSecurity{
			DNSSEC: &model.ExportedDNSSEC{
				Enabled:   s.Security.DNSSECSettings.Enabled,
				SendDoBit: s.Security.DNSSECSettings.SendDoBit,
			},
		}
	}

	// Custom rules — specRef: F7; internal ID and positional order stripped per F9.
	// Rules are emitted in display order so the import side can re-derive `order`
	// from the array index.
	if len(s.CustomRules) > 0 {
		ordered := make([]*model.CustomRule, 0, len(s.CustomRules))
		for _, r := range s.CustomRules {
			if r != nil {
				ordered = append(ordered, r)
			}
		}
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })

		rules := make([]model.ExportedCustomRule, 0, len(ordered))
		for _, r := range ordered {
			rules = append(rules, model.ExportedCustomRule{
				Action: string(r.Action),
				Value:  r.Value,
				Note:   r.Note,
				Group:  r.Group,
			})
		}
		es.CustomRules = rules
	}

	// Per-list groups round-trip alongside the rules' group labels.
	if len(s.CustomRuleGroups.Block) > 0 || len(s.CustomRuleGroups.Allow) > 0 {
		groups := s.CustomRuleGroups.Clone()
		es.CustomRuleGroups = &groups
	}

	// Logs section — specRef: F3 (logs sub-fields)
	if s.Logs != nil {
		es.Logs = &model.ExportedLogs{
			Enabled:       s.Logs.Enabled,
			LogClientsIPs: s.Logs.LogClientsIPs,
			LogDomains:    s.Logs.LogDomains,
			Retention:     string(s.Logs.Retention),
		}
	}

	// Statistics section
	if s.Statistics != nil {
		es.Statistics = &model.ExportedStatistics{
			Enabled: s.Statistics.Enabled,
		}
	}

	// Advanced section — specRef: F7
	// Deliberately not emitted. The recursor is a staging-only control and
	// must not leak into other environments via export files. ExportedAdvanced
	// stays as a type so the import DTO can silently accept-and-ignore the
	// field for forward-compat with hand-edited files.

	return es
}
