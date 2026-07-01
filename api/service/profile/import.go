package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/idn"
	"github.com/ivpn/dns/api/internal/reauth"
	apivalidator "github.com/ivpn/dns/api/internal/validator"
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
// CreatedProfileIds and CreatedProfileNames are parallel slices of equal length
// in payload order; CreatedProfileNames[i] holds the *resolved* name (post
// collision-resolution per I24) so the UI can echo back what the user will
// actually see in the profile list. Mirrors spec rows I19, I19b, I20.
type ImportResult struct {
	CreatedProfileIds   []string `json:"createdProfileIds"`
	CreatedProfileNames []string `json:"createdProfileNames"`
	Warnings            []string `json:"warnings"`
}

// ErrImportNotImplemented is returned by Import until the implementation lands.
// Phase 2+ removes this.
var ErrImportNotImplemented = errors.New("import not implemented")

// maxCustomRulesPerProfile is the per-profile cap on imported custom rules.
// The DTO layer enforces the same limit (ExportedSettings.CustomRules max=1000),
// but the service applies a defensive check to remain safe when called without HTTP.
// specRef: S6, V10
const maxCustomRulesPerProfile = model.ExportedCustomRulesLimit

// maxCustomRuleGroupsPerList is the per-list defensive cap on imported groups.
// The DTO layer enforces the same limit (CustomRuleGroups.Block/Allow max), but
// the service caps too so it stays safe when called without HTTP. Groups aren't
// bounded by rule count (empty groups exist independently), so they need their own
// guard.
const maxCustomRuleGroupsPerList = model.ExportedCustomRuleGroupsLimit

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
// # Decision: best-effort in-process rollback (spec row I22)
//
// The MongoDB deployment does not use replica-set sessions (no WithTransaction
// usage found anywhere in api/db/mongodb/). We therefore implement rollback
// in-process: as each profile is successfully written we append its server-
// generated ProfileId to a Go slice (createdIds). On the first fatal error
// we call rollbackImportedProfiles which iterates that slice and best-effort
// deletes every entry from both the profile repository AND the Redis cache
// (the cache eviction matters because the proxy treats cached settings as
// live for one TTL — see import.go:454-479).
//
// # Failure mode: process death mid-import
//
// The rollback runs in the same goroutine as the import, so if the process
// dies before it gets a chance to execute (OOM kill, SIGKILL, container
// restart, hard request timeout, panic in an unrelated goroutine that takes
// down the runtime) any profiles already persisted to MongoDB will remain.
// They appear in the user's profile list and the user (or support) can
// delete them manually. The blast radius is bounded by the per-import cap
// (ServiceConfig.MaxProfiles, currently 100), and the import endpoint is
// rate-limited to 3 calls / 10 min per account, so the practical exposure
// is small. No security or data-loss impact — orphans are owned by the
// importing account, never leaked across accounts (specRef S3, all IDs
// regenerated server-side).
//
// If orphan profiles become a recurring user complaint, or if import volume
// grows substantially (e.g. admin-driven bulk migrations), revisit with a
// persisted batch-id tag and an off-process cleanup pass that reaps un-
// committed batches. That design adds a schema field, a commit step, and
// a new set of race-condition surfaces; today the cost outweighs the bound
// on orphan visibility.
//
// specRef: M4, M5, M6, I1, I8, I11, I16-I23, V1-V17, S2-S6
func (p *ProfileService) Import(
	ctx context.Context,
	accountId, mode string,
	payload *model.ExportEnvelope,
	currentPassword, reauthToken *string,
	mfa *model.MfaData,
) (*ImportResult, error) {
	if _, err := reauth.Verify(ctx, p.AccountRepository, p.MfaVerifier, reauth.Params{
		AccountId:        accountId,
		TokenType:        auth.TokenTypeReauthProfileImport,
		Password:         currentPassword,
		ReauthToken:      reauthToken,
		Mfa:              mfa,
		PersistOnConsume: true,
	}); err != nil {
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

	// specRef: I24 -- seed the taken-name set with every existing account profile
	// so name-collision resolution sees the current account state. The set is
	// updated in-place as each payload profile is allocated a (possibly renamed)
	// name, so two payload profiles with the same source name also collide
	// against each other.
	takenNames := make(map[string]struct{}, len(existingProfiles))
	for _, existing := range existingProfiles {
		takenNames[apivalidator.NormalizeName(existing.Name)] = struct{}{}
	}

	// createdIds and createdNames accumulate per-profile output in payload
	// order. On partial failure createdIds is used to roll back via deletion.
	// specRef: I19, I19b
	createdIds := make([]string, 0, len(payload.Profiles))
	createdNames := make([]string, 0, len(payload.Profiles))

	for _, ep := range payload.Profiles {
		// specRef: I24 -- normalise name: truncate to MaxProfileNameLen before
		// collision resolution so the persisted Profile.Name always fits within
		// the storage cap. Over-length names produce a non-fatal warning.
		if len([]rune(ep.Name)) > model.MaxProfileNameLen {
			ep.Name = fitNameWithSuffix(ep.Name, "", model.MaxProfileNameLen)
			warnings = append(warnings, "profile name truncated to 50 characters")
		}

		// specRef: I24 -- resolve name collisions before persisting.
		resolvedName, renameWarning := resolveImportName(ep.Name, takenNames)
		if renameWarning != "" {
			warnings = append(warnings, renameWarning)
		}
		takenNames[apivalidator.NormalizeName(resolvedName)] = struct{}{}

		profileId, profileWarnings, err := p.importOneProfile(ctx, accountId, ep, resolvedName)
		if err != nil {
			// specRef: I21, I22 -- rollback: delete every profile created so far in this batch.
			p.rollbackImportedProfiles(ctx, accountId, createdIds)
			return nil, err
		}
		createdIds = append(createdIds, profileId)
		createdNames = append(createdNames, apivalidator.NormalizeName(resolvedName))
		warnings = append(warnings, profileWarnings...)
	}

	return &ImportResult{
		CreatedProfileIds:   createdIds,
		CreatedProfileNames: createdNames,
		Warnings:            warnings,
	}, nil
}

// importOneProfile creates a single profile from a model.ExportedProfile entry.
// It returns the fresh ProfileId, any non-fatal warnings, and a fatal error (nil on success).
// All IDs are regenerated server-side; none are carried from the payload.
// resolvedName is the post-collision-resolution name (see resolveImportName); use
// it for both the persisted Name and any user-facing warning text so the user
// sees consistent naming end-to-end.
// specRef: S3, I19, I24, V8, V9, V11, V12, S5, V14
func (p *ProfileService) importOneProfile(
	ctx context.Context,
	accountId string,
	ep model.ExportedProfile,
	resolvedName string,
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
				resolvedName, maxCustomRulesPerProfile, len(ep.Settings.CustomRules)-maxCustomRulesPerProfile,
			))
		}
		validRules, warnings = p.validateAndMapRules(rulesInput, resolvedName, accountId, warnings)
	}

	// Defensive per-list cap on groups; the DTO layer enforces the same limit, but
	// guard here too since groups aren't bounded by rule count.
	settings.CustomRuleGroups.Block, warnings = capGroups(settings.CustomRuleGroups.Block, "denylist", resolvedName, warnings)
	settings.CustomRuleGroups.Allow, warnings = capGroups(settings.CustomRuleGroups.Allow, "allowlist", resolvedName, warnings)

	// Persist the profile document (without custom rules — those go via CreateCustomRules).
	newProfile := &model.Profile{
		ID:        primitive.NewObjectID(),
		ProfileId: freshProfileId,
		AccountId: accountId,
		Name:      apivalidator.NormalizeName(resolvedName),
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
		if err := p.Cache.AddCustomRules(ctx, freshProfileId, validRules); err != nil {
			return "", nil, err
		}
	}

	return freshProfileId, warnings, nil
}

// mapExportedSettings converts model.ExportedSettings into model.ProfileSettings,
// falling back to model.NewSettings() defaults for any nil section.
// specRef: F1-F7
func (p *ProfileService) mapExportedSettings(src *model.ExportedSettings, profileId string) *model.ProfileSettings {
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

	// Per-list custom rule groups round-trip as-is. Groups that end up with no
	// member rule (e.g. their rules were skipped) are harmless.
	if src.CustomRuleGroups != nil {
		s.CustomRuleGroups = src.CustomRuleGroups.Clone()
	}

	// Advanced section — specRef: F7
	// Silently ignored on import. The recursor is a staging-only control;
	// imported profiles always inherit RECURSOR_DEFAULT from model.NewSettings(),
	// matching the regular create-profile path. model.ExportedAdvanced is still
	// tolerated by the DTO decoder so old or hand-edited files import cleanly.
	_ = src.Advanced

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
	rules []model.ExportedCustomRule,
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
			Note:   r.Note,
			Group:  r.Group,
			// Order is re-derived from payload position; export omits it. Using the
			// source index (not len(valid)) keeps spacing stable even when an
			// earlier rule is skipped, and the values are still strictly increasing.
			Order: i,
		})
	}

	return valid, warnings
}

// capGroups truncates a per-list group slice to maxCustomRuleGroupsPerList,
// appending a warning when entries are discarded. Defensive mirror of the rules
// cap; the DTO validator rejects over-limit payloads on the HTTP path.
func capGroups(list []model.CustomRuleGroup, listName, profileName string, warnings []string) ([]model.CustomRuleGroup, []string) {
	if len(list) > maxCustomRuleGroupsPerList {
		discarded := len(list) - maxCustomRuleGroupsPerList
		list = list[:maxCustomRuleGroupsPerList]
		warnings = append(warnings, fmt.Sprintf(
			"profile '%s': %s groups capped at %d; %d were discarded",
			profileName, listName, maxCustomRuleGroupsPerList, discarded,
		))
	}
	return list, warnings
}

// rollbackImportedProfiles deletes every profile in profileIds. This is the
// in-process rollback path (spec row I22) — the only rollback path; there is
// no persisted-tag cleanup pass behind it. Errors are logged at Warn level
// but not returned; the caller has already encountered a fatal error and
// this is best-effort cleanup. See the # Failure mode section in Import()
// for what happens when the process dies before this function runs.
//
// Mirror the cleanup of the regular DeleteProfile path (service.go:171-173):
// importOneProfile populates Redis via Cache.CreateOrUpdateProfileSettings and
// Cache.AddCustomRules, so the rollback must also evict the cached settings
// for every profileId in the batch. Without this, the proxy would treat the
// rolled-back profile as live until the 30-second TTL expires, and a fresh
// import that happens to reuse a profileId could be shadowed by the stale
// cache entry.
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
		if err := p.Cache.DeleteProfileSettings(ctx, pid); err != nil {
			log.Warn().
				Str("account_id", accountId).
				Str("profile_id", pid).
				Err(err).
				Msg("import rollback: failed to evict cached profile settings")
		}
	}
}

// resolveImportName returns a name that does not collide with any entry in
// takenNames. If original is already free, it is returned unchanged with an
// empty warning. Otherwise the resolver tries "{original} (imported)" and then
// "{original} (imported 2)", "(imported 3)", … capping the final name at
// model.MaxProfileNameLen by trimming the original portion (suffix is always
// preserved so the rename remains visible to the user).
//
// The takenNames keys are NFC-normalized (apivalidator.NormalizeName) so that
// visually-equivalent names ("Café" vs "Café") collide as expected.
//
// specRef: I24
func resolveImportName(original string, takenNames map[string]struct{}) (resolved, warning string) {
	if _, exists := takenNames[apivalidator.NormalizeName(original)]; !exists {
		return original, ""
	}

	// First retry: simple " (imported)" suffix.
	candidate := fitNameWithSuffix(original, " (imported)", model.MaxProfileNameLen)
	if _, exists := takenNames[apivalidator.NormalizeName(candidate)]; !exists {
		return candidate, fmt.Sprintf(
			"profile '%s' renamed to '%s' to avoid name collision",
			original, candidate,
		)
	}

	// Counter retries. Bound the loop at MaxProfiles (100 per batch) + an existing
	// account's MaxProfiles (100) — worst-case 200 collisions, still well under
	// the 1000-iteration cap.
	for n := 2; n < 1000; n++ {
		candidate = fitNameWithSuffix(original, fmt.Sprintf(" (imported %d)", n), model.MaxProfileNameLen)
		if _, exists := takenNames[apivalidator.NormalizeName(candidate)]; !exists {
			return candidate, fmt.Sprintf(
				"profile '%s' renamed to '%s' to avoid name collision",
				original, candidate,
			)
		}
	}

	// Defensive fallback: 1000 collisions for the same source name is far past
	// any realistic account state. Fall back to a ProfileId-style suffix; the
	// safe_name and length validators still pass.
	candidate = fitNameWithSuffix(original, fmt.Sprintf(" (imported %d)", time.Now().UTC().UnixNano()), model.MaxProfileNameLen)
	return candidate, fmt.Sprintf(
		"profile '%s' renamed to '%s' to avoid name collision",
		original, candidate,
	)
}

// fitNameWithSuffix concatenates base + suffix, trimming base if needed to keep
// the result within max runes. Suffix is preserved in full so the rename
// remains legible. Operates on runes, not bytes, to handle multi-byte UTF-8
// names correctly.
func fitNameWithSuffix(base, suffix string, max int) string {
	baseRunes := []rune(base)
	suffixRunes := []rune(suffix)
	if len(baseRunes)+len(suffixRunes) <= max {
		return base + suffix
	}
	keep := max - len(suffixRunes)
	if keep < 0 {
		keep = 0
	}
	return string(baseRunes[:keep]) + suffix
}
