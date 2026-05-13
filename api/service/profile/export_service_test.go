package profile_test

// export_service_test.go — unit tests for ProfileService.Export business logic.
//
// Choice: Option B (black-box, package profile_test) is required here because
// the white-box package (package profile) cannot import api/mocks without
// creating an import cycle (mocks/profile_servicer.go imports
// api/service/profile). All assertions are made through the public Export()
// method. Private helpers are covered indirectly.
//
// specRef annotations trace to docs/specs/account-export-import-behaviour.md.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
)

// ---- helpers ----------------------------------------------------------------

// ptrStr returns a pointer to the string value.
func ptrStr(s string) *string { return &s }

// bcryptExportHash creates a bcrypt hash of the given password for test accounts.
func bcryptExportHash(t *testing.T, password string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(b)
}

// authorisedAccount returns an *model.Account whose password matches "testpw".
func authorisedAccount(t *testing.T) *model.Account {
	t.Helper()
	hash := bcryptExportHash(t, "testpw")
	return &model.Account{Password: &hash}
}

// newExportSvc builds a minimal ProfileService wired to the given mocks.
func newExportSvc(
	t *testing.T,
	profileRepo *mocks.ProfileRepository,
	accountRepo *mocks.AccountRepository,
	svcCfg config.ServiceConfig,
) *profile.ProfileService {
	t.Helper()
	return &profile.ProfileService{
		ProfileRepository: profileRepo,
		AccountRepository: accountRepo,
		ServiceConfig:     svcCfg,
	}
}

// minimalProfile builds a model.Profile with Settings initialised by model.NewSettings().
func minimalProfile(name, accountId string) *model.Profile {
	return &model.Profile{
		ID:        primitive.NewObjectID(),
		ProfileId: "prf-" + name,
		AccountId: accountId,
		Name:      name,
		Settings:  model.NewSettings(),
	}
}

// fullProfile returns a profile whose every settings sub-section is populated,
// suitable for asserting the complete field-mapping table (spec Section F).
func fullProfile(accountId string) *model.Profile {
	p := minimalProfile("Full Profile", accountId)
	p.Settings.Privacy = &model.Privacy{
		Blocklists:                []string{"bl-1", "bl-2"},
		Services:                  []string{"svc-a"},
		DefaultRule:               "allow",
		BlocklistsSubdomainsRule:  "block",
		CustomRulesSubdomainsRule: "include",
	}
	p.Settings.Security = &model.Security{
		DNSSECSettings: model.DNSSECSettings{Enabled: true, SendDoBit: true},
	}
	p.Settings.CustomRules = []*model.CustomRule{
		{ID: primitive.NewObjectID(), Action: "block", Value: "ads.example.com"},
		{ID: primitive.NewObjectID(), Action: "allow", Value: "safe.example.com"},
	}
	p.Settings.Logs = &model.LogsSettings{
		Enabled:       true,
		LogClientsIPs: true,
		LogDomains:    false,
		Retention:     "1w",
	}
	p.Settings.Statistics = &model.StatisticsSettings{Enabled: true}
	p.Settings.Advanced = &model.Advanced{Recursor: "unbound"}
	return p
}

// ---- Export scope tests ----------------------------------------------------

// specRef:"E5"
func TestExport_ScopeAll_ReturnsAllOwnedProfiles(t *testing.T) {
	const accountId = "acct-all"

	profiles := []model.Profile{
		*minimalProfile("Alpha", accountId),
		*minimalProfile("Beta", accountId),
		*minimalProfile("Gamma", accountId),
	}

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return(profiles, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	require.NoError(t, err)
	require.NotNil(t, env)
	require.Len(t, env.Profiles, 3)
	assert.Equal(t, "Alpha", env.Profiles[0].Name)
	assert.Equal(t, "Beta", env.Profiles[1].Name)
	assert.Equal(t, "Gamma", env.Profiles[2].Name)
}

// specRef:"E7"
func TestExport_ScopeSelected_ReturnsRequestedProfiles(t *testing.T) {
	const accountId = "acct-sel"

	p1 := minimalProfile("Profile-1", accountId)
	p1.ProfileId = "prf-1"

	p3 := minimalProfile("Profile-3", accountId)
	p3.ProfileId = "prf-3"

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfileById", context.Background(), "prf-1").Return(p1, nil)
	profileRepo.On("GetProfileById", context.Background(), "prf-3").Return(p3, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeSelected, []string{"prf-1", "prf-3"}, ptrStr("testpw"), nil)
	require.NoError(t, err)
	require.NotNil(t, env)
	require.Len(t, env.Profiles, 2)
	assert.Equal(t, "Profile-1", env.Profiles[0].Name)
	assert.Equal(t, "Profile-3", env.Profiles[1].Name)
}

// specRef:"E9" — profile exists but belongs to a different account
func TestExport_ScopeSelected_ForeignProfileId_ReturnsNotFound(t *testing.T) {
	const accountId = "acct-owner"
	const foreignId = "prf-foreign"

	foreignProfile := minimalProfile("Enemy Profile", "acct-other")
	foreignProfile.ProfileId = foreignId

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	// validateProfileIdAffiliation finds the profile then rejects because
	// AccountId does not match — it returns dbErrors.ErrProfileNotFound.
	profileRepo.On("GetProfileById", context.Background(), foreignId).Return(foreignProfile, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeSelected, []string{foreignId}, ptrStr("testpw"), nil)
	assert.ErrorIs(t, err, dbErrors.ErrProfileNotFound)
	assert.Nil(t, env, "no profile data must be returned when ownership check fails")
}

// specRef:"E9" — profile does not exist at all
func TestExport_ScopeSelected_NonExistentProfileId_ReturnsNotFound(t *testing.T) {
	const accountId = "acct-owner"
	const missingId = "prf-missing"

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfileById", context.Background(), missingId).Return(nil, dbErrors.ErrProfileNotFound)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeSelected, []string{missingId}, ptrStr("testpw"), nil)
	assert.ErrorIs(t, err, dbErrors.ErrProfileNotFound)
	assert.Nil(t, env)
}

// specRef:"E10"
func TestExport_ScopeSelected_TooManyIds(t *testing.T) {
	const accountId = "acct-toomany"
	const maxProfiles = 3

	// len(ids) = 4 > maxProfiles = 3
	ids := []string{"prf-a", "prf-b", "prf-c", "prf-d"}

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: maxProfiles})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeSelected, ids, ptrStr("testpw"), nil)
	assert.ErrorIs(t, err, profile.ErrTooManyProfileIds)
	assert.Nil(t, env)
}

// ---- Field-mapping tests (spec Section F) ----------------------------------

// specRef:"F1-F7"
func TestExport_ProfileMapping_Includes(t *testing.T) {
	const accountId = "acct-map"

	src := fullProfile(accountId)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{*src}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	require.NoError(t, err)
	require.Len(t, env.Profiles, 1)

	ep := env.Profiles[0]

	// Name — specRef: F1
	assert.Equal(t, src.Name, ep.Name)

	require.NotNil(t, ep.Settings)
	require.NotNil(t, ep.Settings.Privacy)

	// Blocklists — specRef: F2; model stores enabled IDs only → plain string slice
	require.Len(t, ep.Settings.Privacy.Blocklists, 2)
	assert.Equal(t, "bl-1", ep.Settings.Privacy.Blocklists[0])
	assert.Equal(t, "bl-2", ep.Settings.Privacy.Blocklists[1])

	// Services — specRef: F3
	require.Len(t, ep.Settings.Privacy.Services, 1)
	assert.Equal(t, "svc-a", ep.Settings.Privacy.Services[0])

	// Privacy rule fields — specRef: F3, F4, F5
	assert.Equal(t, "allow", ep.Settings.Privacy.DefaultRule)
	assert.Equal(t, "block", ep.Settings.Privacy.BlocklistsSubdomainsRule)
	assert.Equal(t, "include", ep.Settings.Privacy.CustomRulesSubdomainsRule)

	// DNSSEC — specRef: F6
	require.NotNil(t, ep.Settings.Security)
	require.NotNil(t, ep.Settings.Security.DNSSEC)
	assert.True(t, ep.Settings.Security.DNSSEC.Enabled)
	assert.True(t, ep.Settings.Security.DNSSEC.SendDoBit)

	// Custom rules — specRef: F7; ObjectID must not appear (F9)
	require.Len(t, ep.Settings.CustomRules, 2)
	assert.Equal(t, "block", ep.Settings.CustomRules[0].Action)
	assert.Equal(t, "ads.example.com", ep.Settings.CustomRules[0].Value)
	assert.Equal(t, "allow", ep.Settings.CustomRules[1].Action)
	assert.Equal(t, "safe.example.com", ep.Settings.CustomRules[1].Value)

	// Logs
	require.NotNil(t, ep.Settings.Logs)
	assert.True(t, ep.Settings.Logs.Enabled)
	assert.True(t, ep.Settings.Logs.LogClientsIPs)
	assert.False(t, ep.Settings.Logs.LogDomains)
	assert.Equal(t, "1w", ep.Settings.Logs.Retention)

	// Statistics
	require.NotNil(t, ep.Settings.Statistics)
	assert.True(t, ep.Settings.Statistics.Enabled)

	// Advanced — specRef: F7 (recursor)
	require.NotNil(t, ep.Settings.Advanced)
	assert.Equal(t, "unbound", ep.Settings.Advanced.Recursor)
}

// specRef:"F8"
func TestExport_AccountFields_AreNotIncluded(t *testing.T) {
	const accountId = "acct-f8"

	src := minimalProfile("Privacy Check", accountId)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{*src}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	require.NoError(t, err)

	raw, err := json.Marshal(env)
	require.NoError(t, err)

	lower := strings.ToLower(string(raw))
	assert.NotContains(t, lower, "email", "account email must not appear in export")
	assert.NotContains(t, lower, "password", "account password must not appear in export")
	assert.NotContains(t, lower, `"mfa"`, "mfa settings must not appear in export")
	assert.NotContains(t, lower, `"tokens"`, "account tokens must not appear in export")
}

// specRef:"F9"
func TestExport_InternalIDs_AreStripped(t *testing.T) {
	const accountId = "acct-f9"

	src := fullProfile(accountId)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{*src}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	require.NoError(t, err)

	raw, err := json.Marshal(env)
	require.NoError(t, err)

	lower := strings.ToLower(string(raw))
	// MongoDB internal identifiers must not appear in any form.
	assert.NotContains(t, lower, `"_id"`, "MongoDB _id must be stripped from export")
	assert.NotContains(t, lower, `"accountid"`, "accountId must be stripped")
	assert.NotContains(t, lower, `"account_id"`, "account_id must be stripped")
	assert.NotContains(t, lower, `"profileid"`, "profileId must be stripped")
	assert.NotContains(t, lower, `"profile_id"`, "profile_id must be stripped")

	// Custom rule ObjectIDs must not appear in the output JSON.
	for _, r := range src.Settings.CustomRules {
		assert.NotContains(t, string(raw), r.ID.Hex(),
			"custom rule ObjectID %s must be stripped", r.ID.Hex())
	}

	// The profile's own MongoDB ObjectID must not appear.
	assert.NotContains(t, string(raw), src.ID.Hex(), "profile ObjectID must be stripped")
}

// ---- Envelope metadata test ------------------------------------------------

// TestExport_EnvelopeMetadata verifies schemaVersion, kind, exportedAt, and
// exportedFrom.service. The __warning field was removed from the envelope so
// that DisallowUnknownFields() on import never rejects a fresh export; the
// warning text is now displayed by the Export dialog UI at download time.
func TestExport_EnvelopeMetadata(t *testing.T) {
	const accountId = "acct-meta"

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	before := time.Now().UTC()
	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	after := time.Now().UTC()

	require.NoError(t, err)
	require.NotNil(t, env)

	assert.Equal(t, 1, env.SchemaVersion)
	assert.Equal(t, "moddns-export", env.Kind)
	assert.False(t, env.ExportedAt.IsZero(), "exportedAt must be set")
	assert.True(t, !env.ExportedAt.Before(before), "exportedAt must not precede test start")
	assert.True(t, !env.ExportedAt.After(after), "exportedAt must not follow test end")

	require.NotNil(t, env.ExportedFrom)
	assert.Equal(t, "modDNS", env.ExportedFrom.Service)
}

// ---- Edge cases ------------------------------------------------------------

// TestExport_EmptyProfileList documents the contract for an account that owns
// zero profiles: Export succeeds and the profiles array is empty but non-nil,
// so the JSON output is [] rather than null.
func TestExport_EmptyProfileList(t *testing.T) {
	const accountId = "acct-empty"

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).Return([]model.Profile{}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil)
	require.NoError(t, err)
	require.NotNil(t, env)
	assert.NotNil(t, env.Profiles, "profiles slice must not be nil")
	assert.Empty(t, env.Profiles)

	raw, err := json.Marshal(env)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"profiles":[]`,
		"empty profiles must marshal as JSON array, not null")
}
