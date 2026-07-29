package profile_test

// Export-completeness guardrails.
//
// Why: adding `security.rebinding_protection` to the settings model did not trip
// any export/import test — the DTO enumerates fields explicitly and the golden
// test only allowlists the DTO, so a model field that never reaches the DTO is
// invisible to it. A plain round-trip test can't catch that either: a field
// missing from both sides round-trips "successfully". The only reliable oracle
// for omissions is parity against the storage model itself.
//
// TestExportParity_EveryModelFieldDecided therefore walks model.Profile
// via reflection and demands every leaf field be either mapped into the export
// envelope or listed in exportExclusions with a spec-row-citing reason. A new
// model field fails this test until the developer makes that decision explicit.
//
// TestExport_Import_Export_RoundTripEquality closes the other direction
// (exported but forgotten on import): the envelope of an imported profile must
// equal the envelope it was imported from.
//
// specRef: S14

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// exportedFields lists every model.Profile leaf that is mapped into the
// export envelope (see service/profile/export.go). Keep in sync with the DTO —
// the parity test fails on any drift in either direction.
var exportedFields = map[string]string{
	"Name": "name (F1)",
	"Settings.Security.DNSSECSettings.Enabled":      "security.dnssec.enabled (F5)",
	"Settings.Security.DNSSECSettings.SendDoBit":    "security.dnssec.sendDoBit (F5)",
	"Settings.Security.RebindingProtection.Enabled": "security.rebindingProtection.enabled (F8)",
	"Settings.Privacy.Blocklists":                   "privacy.blocklists (F2)",
	"Settings.Privacy.Services":                     "privacy.services (F3)",
	"Settings.Privacy.DefaultRule":                  "privacy.defaultRule (F1)",
	"Settings.Privacy.BlocklistsSubdomainsRule":     "privacy.blocklistsSubdomainsRule (F1)",
	"Settings.Privacy.CustomRulesSubdomainsRule":    "privacy.customRulesSubdomainsRule (F1)",
	"Settings.CustomRules.Action":                   "customRules[].action (F4)",
	"Settings.CustomRules.Value":                    "customRules[].value (F4)",
	"Settings.CustomRules.Note":                     "customRules[].note (V11)",
	"Settings.CustomRules.Group":                    "customRules[].group (V12)",
	"Settings.CustomRuleGroups.Block.Name":          "customRuleGroups.block[].name (V12)",
	"Settings.CustomRuleGroups.Block.Comment":       "customRuleGroups.block[].comment (V12)",
	"Settings.CustomRuleGroups.Allow.Name":          "customRuleGroups.allow[].name (V12)",
	"Settings.CustomRuleGroups.Allow.Comment":       "customRuleGroups.allow[].comment (V12)",
	"Settings.Logs.Enabled":                         "logs.enabled (F6)",
	"Settings.Logs.LogClientsIPs":                   "logs.logClientsIPs (F6)",
	"Settings.Logs.LogDomains":                      "logs.logDomains (F6)",
	"Settings.Logs.Retention":                       "logs.retention (F6)",
	"Settings.Statistics.Enabled":                   "statistics.enabled (F6)",
}

// exportExclusions lists every model.Profile leaf that is deliberately
// NOT exported. Each entry must cite the spec row (or rationale) recording that
// decision — this is what makes the omission auditable instead of accidental.
var exportExclusions = map[string]string{
	"ID":                          "F9 — internal Mongo id; never exported",
	"ProfileId":                   "F9 — internal id; regenerated on import",
	"AccountId":                   "account-scoped; must never appear in an export (golden-envelope PII guard)",
	"Settings.ProfileId":          "F9 — internal id; regenerated on import",
	"Settings.CustomRules.ID":     "F9 — internal id; regenerated on import",
	"Settings.CustomRules.Syntax": "derived from Value at parse time; re-derived on import",
	"Settings.CustomRules.Order":  "F9 — positional; re-derived from array index on import",
	"Settings.Advanced.Recursor":  "F7 — staging-only control, deliberately not exported",
}

// collectLeafPaths walks t depth-first and appends dot-separated paths of every
// leaf field. Pointers and slice/array elements are traversed transparently;
// named non-struct types (Retention, CustomRuleAction, primitive.ObjectID) are
// leaves.
func collectLeafPaths(t reflect.Type, prefix string, out *[]string) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		elem := t.Elem()
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			collectLeafPaths(elem, prefix, out)
			return
		}
		*out = append(*out, prefix)
		return
	}
	if t.Kind() != reflect.Struct {
		*out = append(*out, prefix)
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		p := f.Name
		if prefix != "" {
			p = prefix + "." + f.Name
		}
		collectLeafPaths(f.Type, p, out)
	}
}

// specRef: S14 — every settings-model field must be exported or spec-row-excluded.
func TestExportParity_EveryModelFieldDecided(t *testing.T) {
	var actual []string
	collectLeafPaths(reflect.TypeOf(model.Profile{}), "", &actual)
	sort.Strings(actual)

	actualSet := make(map[string]struct{}, len(actual))
	for _, p := range actual {
		actualSet[p] = struct{}{}
	}

	// Every model leaf needs a decision: exported or excluded-with-reason.
	for _, p := range actual {
		_, exported := exportedFields[p]
		_, excluded := exportExclusions[p]
		assert.Truef(t, exported || excluded,
			"new settings field %q has no export decision: map it into the export "+
				"envelope (export.go + import.go + exportedFields) or add it to "+
				"exportExclusions citing a docs/specs/account-export-import-behaviour.md row",
			p)
		assert.Falsef(t, exported && excluded,
			"settings field %q is listed as both exported and excluded", p)
	}

	// No stale registry entries: everything listed must still exist in the model.
	for p := range exportedFields {
		_, ok := actualSet[p]
		assert.Truef(t, ok, "exportedFields entry %q no longer exists in model.Profile", p)
	}
	for p := range exportExclusions {
		_, ok := actualSet[p]
		assert.Truef(t, ok, "exportExclusions entry %q no longer exists in model.Profile", p)
	}
}

// valueAtPath resolves a dot-separated leaf path against v, descending into the
// first element of any slice on the way. Returns an invalid Value if a nil
// pointer or empty slice blocks the path.
func valueAtPath(v reflect.Value, path string) reflect.Value {
	for _, part := range strings.Split(path, ".") {
		for v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
			if v.Len() == 0 {
				return reflect.Value{}
			}
			v = v.Index(0)
			for v.Kind() == reflect.Ptr {
				if v.IsNil() {
					return reflect.Value{}
				}
				v = v.Elem()
			}
		}
		v = v.FieldByName(part)
		if !v.IsValid() {
			return reflect.Value{}
		}
	}
	return v
}

// specRef: S14 — the shared full-profile fixture must exercise every exported
// field, otherwise the golden-envelope and round-trip tests are blind to it.
// Bool leaves are skipped: the exported DTO serializes bools without omitempty,
// so their presence in the envelope does not depend on the fixture's value.
func TestExportParity_FullProfileFixtureExercisesAllExportedFields(t *testing.T) {
	prof := reflect.ValueOf(*fullProfile("acct-parity"))

	for path := range exportedFields {
		v := valueAtPath(prof, path)
		require.Truef(t, v.IsValid(), "fullProfile does not populate %q (nil pointer or empty slice on the path)", path)
		if v.Kind() == reflect.Bool {
			continue
		}
		assert.Falsef(t, v.IsZero(),
			"fullProfile leaves exported field %q zero-valued; populate it so the "+
				"golden and round-trip tests actually exercise it", path)
	}
}

// specRef: S14 — export → import → export must be lossless for everything the
// envelope carries. Catches fields that are exported but not applied on import,
// which the parity test alone cannot see.
func TestExport_Import_Export_RoundTripEquality(t *testing.T) {
	const accountId = "acct-rt"
	src := fullProfile(accountId)

	// First export.
	exportRepo := mocks.NewProfileRepository(t)
	exportAccounts := mocks.NewAccountRepository(t)
	exportRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{*src}, nil)
	exportAccounts.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)
	exportSvc := newExportSvc(t, exportRepo, exportAccounts, config.ServiceConfig{MaxProfiles: 100})

	env1, err := exportSvc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil, nil)
	require.NoError(t, err)
	require.Len(t, env1.Profiles, 1)

	// Import into a fresh account, capturing what would be persisted.
	imp := newImportTestEnv(t, "secret", 100)
	imp.svc.ServicesCatalog = newStaticCatalog("svc-a")

	var capturedProfile *model.Profile
	var capturedRules []*model.CustomRule
	imp.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct-b").
		Return([]model.Profile{}, nil).Once()
	imp.idGen.On("Generate").Return("fresh-rt-id", nil).Once()
	imp.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		capturedProfile = p
		return true
	})).Return(nil).Once()
	imp.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	imp.profileRepo.On("CreateCustomRules", mock.Anything, "fresh-rt-id",
		mock.MatchedBy(func(rules []*model.CustomRule) bool {
			capturedRules = rules
			return true
		})).Return(nil).Once()
	imp.cache.On("AddCustomRules", mock.Anything, "fresh-rt-id",
		mock.AnythingOfType("[]*model.CustomRule")).Return(nil).Once()
	for _, blID := range src.Settings.Privacy.Blocklists {
		imp.blocklistRepo.On("Get", mock.Anything,
			map[string]any{"blocklist_id": blID}, "updated").
			Return([]*model.Blocklist{{BlocklistID: blID}}, nil).Once()
	}

	result, err := imp.svc.Import(context.Background(), "acct-b",
		profile.ImportModeCreateNew, env1, ptr("secret"), nil, nil)
	require.NoError(t, err)
	require.Empty(t, result.Warnings, "round-trip import must be warning-free")
	require.NotNil(t, capturedProfile)

	// Re-export the imported profile.
	reassembled := *capturedProfile
	require.NotNil(t, reassembled.Settings)
	reassembled.Settings.CustomRules = capturedRules

	reexportRepo := mocks.NewProfileRepository(t)
	reexportAccounts := mocks.NewAccountRepository(t)
	reexportRepo.On("GetProfilesByAccountId", context.Background(), "acct-b").Return([]model.Profile{reassembled}, nil)
	reexportAccounts.On("GetAccountById", context.Background(), "acct-b").Return(authorisedAccount(t), nil)
	reexportSvc := newExportSvc(t, reexportRepo, reexportAccounts, config.ServiceConfig{MaxProfiles: 100})

	env2, err := reexportSvc.Export(context.Background(), "acct-b", profile.ExportScopeAll, nil, ptrStr("testpw"), nil, nil)
	require.NoError(t, err)
	require.Len(t, env2.Profiles, 1)

	// Envelope metadata (timestamps, source info) legitimately differs; the
	// profile payloads must not.
	json1, err := json.Marshal(env1.Profiles[0])
	require.NoError(t, err)
	json2, err := json.Marshal(env2.Profiles[0])
	require.NoError(t, err)
	assert.JSONEq(t, string(json1), string(json2),
		"export → import → export lost or mutated data; a field is exported but not applied on import (or vice versa)")
}
