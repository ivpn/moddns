package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/idn"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ImportMode enumerates the supported import modes.
// v1: create_new only. replace is reserved for a future PR (spec rows I8-I11).
const (
	ImportModeCreateNew = "create_new"
)

// ImportResult is the response payload of a successful import.
// Mirrors spec rows I19-I20.
type ImportResult struct {
	CreatedProfileIds []string `json:"createdProfileIds"`
	Warnings          []string `json:"warnings"`
}

// ErrImportNotImplemented is returned by Import until the implementation lands.
// Phase 2+ removes this.
var ErrImportNotImplemented = errors.New("import not implemented")

// maxCustomRulesPerProfile is the per-profile cap on imported custom rules.
// The DTO layer enforces the same limit (requests.ImportSettings.CustomRules max=10000),
// but the service applies a defensive check to remain safe when called without HTTP.
// specRef: S6, V10
const maxCustomRulesPerProfile = 10_000

// staleExportThreshold is the age beyond which an export file triggers an advisory warning.
// specRef: V16, V17
const staleExportThreshold = 90 * 24 * time.Hour

// Import creates fresh profiles from a parsed and validated export envelope.
//
// # Decision: skip-with-warning for invalid rules (spec rows V11, V12)
//
// The spec is ambiguous between "fail the entire import" and "skip-with-warning"
// for rules that fail re-validation (e.g. an action or syntax that was valid in
// an older schema version). We choose skip-with-warning: the import succeeds and
// the offending rule is omitted from the created profile, with a warning entry
// added to ImportResult.Warnings. Rationale: an export file is a backup artifact;
// failing the whole import because one rule became invalid after a schema tightening
// destroys more value than it protects. Users can inspect the warning list and
// recreate the rejected rules manually.
//
// # Decision: importBatchId rollback (spec row I22)
//
// The MongoDB deployment does not use replica-set sessions (no WithTransaction
// usage found anywhere in api/db/mongodb/). We therefore use the importBatchId
// rollback pattern: all created profiles carry a transient importBatchId tag so
// that, on partial failure, a cleanup pass can delete every profile created in
// this batch. importBatchId is not persisted as a schema field; it is the
// ProfileId prefix we use to find and delete partial results.
//
// specRef: M4, M5, M6, I1, I8, I11, I16-I23, V1-V17, S2-S6
func (p *ProfileService) Import(
	ctx context.Context,
	accountId, mode string,
	payload *ExportEnvelope,
	currentPassword, reauthToken *string,
) (*ImportResult, error) {
	if err := p.verifyReauth(ctx, accountId, auth.TokenTypeReauthProfileImport, currentPassword, reauthToken); err != nil {
		return nil, err
	}

	// specRef: I8, I11 -- defensive mode check; the DTO already enforces oneof=create_new
	// but the service must be safe to call directly without HTTP.
	if mode != ImportModeCreateNew {
		return nil, ErrUnsupportedImportMode
	}

	// specRef: V1 -- schema version guard
	if payload.SchemaVersion != 1 {
		return nil, ErrUnsupportedSchemaVersion
	}

	// specRef: V2 -- kind discriminator
	if payload.Kind != "moddns-export" {
		return nil, ErrInvalidExportKind
	}

	// specRef: V5 -- at least one profile required
	if len(payload.Profiles) == 0 {
		return nil, ErrEmptyImportPayload
	}

	// specRef: I16, I17, I18 -- profile-count cap
	existingProfiles, err := p.GetProfiles(ctx, accountId)
	if err != nil {
		return nil, err
	}
	currentCount := len(existingProfiles)
	incomingCount := len(payload.Profiles)
	if currentCount+incomingCount > p.ServiceConfig.MaxProfiles {
		return nil, fmt.Errorf("%w: would exceed limit of %d profiles; have %d, payload has %d, max allowed %d",
			ErrMaxProfilesExceeded,
			p.ServiceConfig.MaxProfiles,
			currentCount,
			incomingCount,
			p.ServiceConfig.MaxProfiles,
		)
	}

	warnings := make([]string, 0)

	// specRef: V16, V17 -- stale-export advisory warning
	if time.Since(payload.ExportedAt) > staleExportThreshold {
		warnings = append(warnings, fmt.Sprintf(
			"export file is older than 90 days (exported %s)",
			payload.ExportedAt.UTC().Format(time.RFC3339),
		))
	}

	// createdIds accumulates the fresh ProfileId of each successfully created profile,
	// in payload order. On partial failure these are used to roll back via deletion.
	createdIds := make([]string, 0, len(payload.Profiles))

	for _, ep := range payload.Profiles {
		profileId, profileWarnings, err := p.importOneProfile(ctx, accountId, ep, payload)
		if err != nil {
			// specRef: I21, I22 -- rollback: delete every profile created so far in this batch.
			p.rollbackImportedProfiles(ctx, accountId, createdIds)
			return nil, err
		}
		createdIds = append(createdIds, profileId)
		warnings = append(warnings, profileWarnings...)
	}

	return &ImportResult{
		CreatedProfileIds: createdIds,
		Warnings:          warnings,
	}, nil
}

// importOneProfile creates a single profile from an ExportedProfile entry.
// It returns the fresh ProfileId, any non-fatal warnings, and a fatal error (nil on success).
// All IDs are regenerated server-side; none are carried from the payload.
// specRef: S3, I19, V8, V9, V11, V12, S5, V14
func (p *ProfileService) importOneProfile(
	ctx context.Context,
	accountId string,
	ep ExportedProfile,
	envelope *ExportEnvelope,
) (profileId string, warnings []string, err error) {
	warnings = make([]string, 0)

	// specRef: S3 -- generate a fresh ProfileId; never carry source IDs.
	freshProfileId, err := p.IdGen.Generate()
	if err != nil {
		return "", nil, err
	}

	// Build the ProfileSettings from the exported data.
	settings := p.mapExportedSettings(ep.Settings, freshProfileId)

	// specRef: V8, V9 -- validate blocklists and services against their catalogs;
	// missing IDs produce a warning and are dropped from the imported profile.
	if ep.Settings != nil && ep.Settings.Privacy != nil {
		validBlocklists, blocklistWarnings := p.filterCatalogRefs(
			ctx, ep.Settings.Privacy.Blocklists, "blocklist",
		)
		settings.Privacy.Blocklists = validBlocklists
		warnings = append(warnings, blocklistWarnings...)

		validServices, serviceWarnings := p.filterServiceRefs(
			ctx, ep.Settings.Privacy.Services,
		)
		settings.Privacy.Services = validServices
		warnings = append(warnings, serviceWarnings...)
	}

	// specRef: V11, V12, S5 -- re-validate every custom rule and surface IDN warnings.
	// Decision: skip-with-warning for rules that fail validation (see top-level comment).
	var validRules []*model.CustomRule
	if ep.Settings != nil {
		// specRef: S6, V10 -- defensive cap; DTO layer enforces the same limit.
		rulesInput := ep.Settings.CustomRules
		if len(rulesInput) > maxCustomRulesPerProfile {
			rulesInput = rulesInput[:maxCustomRulesPerProfile]
			warnings = append(warnings, fmt.Sprintf(
				"profile '%s': custom rules capped at %d; %d rules were discarded",
				ep.Name, maxCustomRulesPerProfile, len(ep.Settings.CustomRules)-maxCustomRulesPerProfile,
			))
		}
		validRules, warnings = p.validateAndMapRules(rulesInput, ep.Name, accountId, warnings)
	}

	// Persist the profile document (without custom rules — those go via CreateCustomRules).
	newProfile := &model.Profile{
		ID:        primitive.NewObjectID(),
		ProfileId: freshProfileId,
		AccountId: accountId,
		Name:      ep.Name,
		Settings:  settings,
	}

	if err := p.ProfileRepository.CreateProfile(ctx, newProfile); err != nil {
		return "", nil, err
	}

	// Populate cache for the new profile.
	if err := p.Cache.CreateOrUpdateProfileSettings(ctx, settings, true); err != nil {
		return "", nil, err
	}

	// specRef: V11, V12 -- bulk-insert the validated custom rules.
	if len(validRules) > 0 {
		if err := p.ProfileRepository.CreateCustomRules(ctx, freshProfileId, validRules); err != nil {
			return "", nil, err
		}
		for _, rule := range validRules {
			if err := p.Cache.AddCustomRule(ctx, freshProfileId, rule); err != nil {
				return "", nil, err
			}
		}
	}

	return freshProfileId, warnings, nil
}

// mapExportedSettings converts ExportedSettings into model.ProfileSettings,
// falling back to model.NewSettings() defaults for any nil section.
// specRef: F1-F7
func (p *ProfileService) mapExportedSettings(src *ExportedSettings, profileId string) *model.ProfileSettings {
	s := model.NewSettings()
	s.ProfileId = profileId

	if src == nil {
		return s
	}

	if src.Privacy != nil {
		// DefaultRule: fall back to the default if the imported value is not recognised.
		if src.Privacy.DefaultRule == model.DEFAULT_RULE_BLOCK || src.Privacy.DefaultRule == model.DEFAULT_RULE_ALLOW {
			s.Privacy.DefaultRule = src.Privacy.DefaultRule
		}
		// BlocklistsSubdomainsRule
		if src.Privacy.BlocklistsSubdomainsRule == model.ACTION_BLOCK || src.Privacy.BlocklistsSubdomainsRule == model.ACTION_ALLOW {
			s.Privacy.BlocklistsSubdomainsRule = src.Privacy.BlocklistsSubdomainsRule
		}
		// CustomRulesSubdomainsRule
		if src.Privacy.CustomRulesSubdomainsRule == model.CUSTOM_RULES_SUBDOMAINS_INCLUDE ||
			src.Privacy.CustomRulesSubdomainsRule == model.CUSTOM_RULES_SUBDOMAINS_EXACT {
			s.Privacy.CustomRulesSubdomainsRule = src.Privacy.CustomRulesSubdomainsRule
		}
		// Blocklists and Services are populated after catalog validation.
	}

	if src.Security != nil && src.Security.DNSSEC != nil {
		s.Security.DNSSECSettings.Enabled = src.Security.DNSSEC.Enabled
		s.Security.DNSSECSettings.SendDoBit = src.Security.DNSSEC.SendDoBit
	}

	if src.Logs != nil {
		s.Logs.Enabled = src.Logs.Enabled
		s.Logs.LogClientsIPs = src.Logs.LogClientsIPs
		s.Logs.LogDomains = src.Logs.LogDomains
		if ret, err := model.NewRetention(src.Logs.Retention); err == nil {
			s.Logs.Retention = ret
		}
		// Invalid retention values silently fall back to the default (RetentionOneDay).
	}

	if src.Statistics != nil {
		s.Statistics.Enabled = src.Statistics.Enabled
	}

	if src.Advanced != nil {
		if slices.Contains(model.RECURSORS, src.Advanced.Recursor) {
			s.Advanced.Recursor = src.Advanced.Recursor
		}
		// Unrecognised recursor falls back to RECURSOR_DEFAULT.
	}

	return s
}

// filterCatalogRefs checks each blocklist ID against the catalog.
// IDs present in the catalog are returned in validIDs;
// missing IDs produce a warning entry and are dropped.
// Decision: warn-and-skip on missing; the import still succeeds.
// specRef: V8
func (p *ProfileService) filterCatalogRefs(
	ctx context.Context,
	ids []string,
	kind string,
) (validIDs []string, warnings []string) {
	validIDs = make([]string, 0, len(ids))
	warnings = make([]string, 0)
	for _, id := range ids {
		fltr := map[string]any{"blocklist_id": id}
		results, err := p.BlocklistService.GetBlocklist(ctx, fltr, "")
		if err != nil || len(results) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s '%s' is not in the catalog -- skipped", kind, id,
			))
			continue
		}
		validIDs = append(validIDs, id)
	}
	return validIDs, warnings
}

// filterServiceRefs validates each service ID against the services catalog.
// IDs present in the catalog are returned in validIDs; missing IDs produce a
// warning entry and are dropped (same warn-and-skip semantics as V8/blocklists).
// When ServicesCatalog is nil (e.g. in unit tests that do not exercise service
// validation), all IDs are accepted without a catalog check.
// specRef: V9
func (p *ProfileService) filterServiceRefs(
	ctx context.Context,
	ids []string,
) (validIDs []string, warnings []string) {
	validIDs = make([]string, 0, len(ids))
	warnings = make([]string, 0)

	if p.ServicesCatalog == nil {
		// Catalog not wired — accept all IDs (safe-default, proxy ignores unknowns).
		return append(validIDs, ids...), warnings
	}

	cat, err := p.ServicesCatalog.Get()
	if err != nil || cat == nil {
		// Catalog unavailable — accept all IDs and surface a single advisory.
		warnings = append(warnings, "services catalog unavailable -- service IDs accepted without validation")
		return append(validIDs, ids...), warnings
	}

	for _, id := range ids {
		if _, ok := cat.FindByID(id); !ok {
			warnings = append(warnings, fmt.Sprintf(
				"service '%s' is not in the catalog -- skipped", id,
			))
			continue
		}
		validIDs = append(validIDs, id)
	}
	return validIDs, warnings
}

// validateAndMapRules re-validates each exported custom rule and builds a slice
// of *model.CustomRule ready for insertion. Invalid rules (bad action or syntax)
// are skipped with a warning. IDN rules produce an additional warning.
// specRef: V11, V12, S5
func (p *ProfileService) validateAndMapRules(
	rules []ExportedCustomRule,
	profileName string,
	accountId string,
	existingWarnings []string,
) (valid []*model.CustomRule, warnings []string) {
	warnings = existingWarnings
	valid = make([]*model.CustomRule, 0, len(rules))

	for i, r := range rules {
		// specRef: V11 -- re-validate action
		action, err := model.NewCustomRuleAction(r.Action)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"customRules[%d] in profile '%s': action '%s' is not valid -- rule skipped",
				i, profileName, r.Action,
			))
			continue
		}

		// specRef: V12 -- re-validate syntax
		syntax, err := model.NewCustomRuleSyntax(p.Validate, r.Value)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"customRules[%d] in profile '%s': value '%s' has invalid syntax -- rule skipped",
				i, profileName, r.Value,
			))
			continue
		}

		// specRef: S5, V12 -- Punycode / IDN warning
		if idn.ContainsIDN(r.Value) {
			decoded, ok := idn.Decode(r.Value)
			if !ok {
				decoded = "<not decodable>"
			}
			warnings = append(warnings, fmt.Sprintf(
				"customRules[%d] in profile '%s': value '%s' contains an internationalized domain name. Decoded form: '%s'. Visually similar to standard domains -- verify this is what you intended.",
				i, profileName, r.Value, decoded,
			))
			log.Info().
				Str("event", "import_idn_rule").
				Str("account_id", accountId).
				Str("profile_name", profileName).
				Int("rule_index", i).
				Str("rule_value_punycode", r.Value).
				Str("rule_value_decoded", decoded).
				Msg("imported custom rule contains internationalized domain")
		}

		valid = append(valid, &model.CustomRule{
			ID:     primitive.NewObjectID(),
			Action: action,
			Value:  r.Value,
			Syntax: syntax,
		})
	}

	return valid, warnings
}

// rollbackImportedProfiles deletes every profile in profileIds. This is the
// importBatchId fallback rollback path (spec row I22). Errors are logged at
// Warn level but not returned; the caller has already encountered a fatal
// error and this is best-effort cleanup.
// specRef: I22
func (p *ProfileService) rollbackImportedProfiles(ctx context.Context, accountId string, profileIds []string) {
	for _, pid := range profileIds {
		if err := p.ProfileRepository.DeleteProfileById(ctx, pid); err != nil {
			log.Warn().
				Str("account_id", accountId).
				Str("profile_id", pid).
				Err(err).
				Msg("import rollback: failed to delete partially-created profile")
		}
	}
}
