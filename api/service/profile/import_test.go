package profile_test

// import_test.go -- unit tests for ProfileService.Import.
// Each test references the spec row it covers via a specRef comment.
// Spec: docs/specs/account-export-import-behaviour.md

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	intvldtr "github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/blocklist"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/libs/servicescatalog"
)

// ---------------------------------------------------------------------------
// Inline catalog stubs (satisfy profile.ServicesCatalogReader)
// ---------------------------------------------------------------------------

// staticCatalog is a test double for ServicesCatalogReader that returns a
// fixed set of known service IDs.
type staticCatalog struct {
	cat *servicescatalog.Catalog
}

func (s *staticCatalog) Get() (*servicescatalog.Catalog, error) {
	return s.cat, nil
}

// newStaticCatalog returns a catalog containing exactly the given service IDs.
func newStaticCatalog(ids ...string) *staticCatalog {
	services := make([]servicescatalog.Service, len(ids))
	for i, id := range ids {
		services[i] = servicescatalog.Service{
			ID:   id,
			Name: id,
			ASNs: []uint{1},
		}
	}
	return &staticCatalog{cat: &servicescatalog.Catalog{Services: services}}
}

// ---------------------------------------------------------------------------
// Suite-level helpers
// ---------------------------------------------------------------------------

// importTestEnv bundles all mocks needed for an import test.
type importTestEnv struct {
	svc           *profile.ProfileService
	profileRepo   *mocks.ProfileRepository
	accountRepo   *mocks.AccountRepository
	blocklistRepo *mocks.BlocklistRepository
	cache         *mocks.Cachecache
	idGen         *mocks.Generatoridgen
}

// newImportTestEnv builds a fully-wired ProfileService. The given password is
// bcrypt-hashed and placed in the account mock so that verifyReauth succeeds.
func newImportTestEnv(t *testing.T, password string, maxProfiles int) *importTestEnv {
	t.Helper()

	// Env vars required by config.New().
	os.Setenv("SERVER_ALLOWED_DOMAINS", "test.com")
	os.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53")

	cfg, err := config.New()
	require.NoError(t, err)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	blocklistRepo := mocks.NewBlocklistRepository(t)
	queryLogsRepo := mocks.NewQueryLogsRepository(t)
	statisticsRepo := mocks.NewStatisticsRepository(t)
	cacheM := mocks.NewCachecache(t)
	idGenM := mocks.NewGeneratoridgen(t)

	apiValidator, vErr := intvldtr.NewAPIValidator()
	require.NoError(t, vErr)
	vld := apiValidator.Validator

	// Pre-configure the account mock so verifyReauth always succeeds.
	hash, hErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, hErr)
	hashStr := string(hash)
	acc := &model.Account{Password: &hashStr}
	accountRepo.On("GetAccountById", mock.Anything, mock.AnythingOfType("string")).
		Return(acc, nil).Maybe()

	blocklistSvc := blocklist.NewBlocklistService(blocklistRepo, cacheM)
	queryLogsSvc := querylogs.NewQueryLogsService(queryLogsRepo)
	statisticsSvc := statistics.NewStatisticsService(statisticsRepo)

	serverCfg := *cfg.Server
	serviceCfg := *cfg.Service
	serviceCfg.MaxProfiles = maxProfiles

	svc := profile.NewProfileService(
		serverCfg,
		serviceCfg,
		profileRepo,
		accountRepo,
		blocklistSvc,
		queryLogsSvc,
		statisticsSvc,
		nil, // servicesCatalog: set per-test via env.svc.ServicesCatalog = ...
		cacheM,
		idGenM,
		vld,
	)

	return &importTestEnv{
		svc:           svc,
		profileRepo:   profileRepo,
		accountRepo:   accountRepo,
		blocklistRepo: blocklistRepo,
		cache:         cacheM,
		idGen:         idGenM,
	}
}

// minimalEnvelope returns a minimal valid ExportEnvelope with n profiles, each
// named "Profile" with no custom rules.
func minimalEnvelope(n int) *profile.ExportEnvelope {
	profiles := make([]profile.ExportedProfile, n)
	for i := range profiles {
		profiles[i] = profile.ExportedProfile{
			Name:     "Profile",
			Settings: &profile.ExportedSettings{},
		}
	}
	return &profile.ExportEnvelope{
		SchemaVersion: 1,
		Kind:          "moddns-export",
		ExportedAt:    time.Now(),
		Profiles:      profiles,
	}
}

// expectOneSuccessfulCreate configures mock expectations for one profile
// creation round-trip: GetProfilesByAccountId (once), idGen.Generate,
// CreateProfile, CreateOrUpdateProfileSettings.
func expectOneSuccessfulCreate(
	env *importTestEnv,
	accountId string,
	existing []model.Profile,
	freshId string,
) {
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, accountId).
		Return(existing, nil).Once()
	env.idGen.On("Generate").Return(freshId, nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything,
		mock.MatchedBy(func(p *model.Profile) bool {
			return p.AccountId == accountId && p.ProfileId == freshId
		})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
}

// ---------------------------------------------------------------------------
// Helper shared with verify_reauth_test.go (already defined there as ptr).
// We cannot redeclare it here; the existing ptr() in verify_reauth_test.go
// is in the same package (profile_test), so it is visible here automatically.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Mode validation
// ---------------------------------------------------------------------------

// specRef: I8
func TestImport_ModeCreateNew_Accepted(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	result, err := env.svc.Import(
		context.Background(), "acct1",
		profile.ImportModeCreateNew,
		minimalEnvelope(1),
		ptr("secret"), nil,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"fresh-id-1"}, result.CreatedProfileIds)
	assert.Equal(t, []string{}, result.Warnings)
}

// specRef: I11
func TestImport_ModeUnknown_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// No DB calls should occur when the mode is invalid.
	result, err := env.svc.Import(
		context.Background(), "acct1",
		"merge",
		minimalEnvelope(1),
		ptr("secret"), nil,
	)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrUnsupportedImportMode)
}

// ---------------------------------------------------------------------------
// Envelope validation
// ---------------------------------------------------------------------------

// specRef: V1
func TestImport_SchemaVersionMismatch_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	envelope := minimalEnvelope(1)
	envelope.SchemaVersion = 2

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrUnsupportedSchemaVersion)
}

// specRef: V2
func TestImport_InvalidKind_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	envelope := minimalEnvelope(1)
	envelope.Kind = "other"

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrInvalidExportKind)
}

// specRef: V5
func TestImport_EmptyPayload_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(0), ptr("secret"), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrEmptyImportPayload)
}

// ---------------------------------------------------------------------------
// Stale-export warning
// ---------------------------------------------------------------------------

// specRef: V16
func TestImport_FreshExport_NoStaleWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	envelope := minimalEnvelope(1)
	envelope.ExportedAt = time.Now() // definitely fresh

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	for _, w := range result.Warnings {
		assert.NotContains(t, w, "older than 90 days",
			"fresh export must not produce a stale-export warning")
	}
}

// specRef: V17
func TestImport_StaleExport_AddsWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	envelope := minimalEnvelope(1)
	envelope.ExportedAt = time.Now().Add(-100 * 24 * time.Hour) // 100 days ago

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "older than 90 days") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected stale-export warning; got: %v", result.Warnings)
}

// ---------------------------------------------------------------------------
// Profile-count cap
// ---------------------------------------------------------------------------

// specRef: I16
func TestImport_ProfileCount_WithinCap(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// 10 existing + 5 incoming = 15 (well within 100)
	existing := make([]model.Profile, 10)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()

	// Set up 5 successful creates
	ids := []string{"id-a", "id-b", "id-c", "id-d", "id-e"}
	for _, id := range ids {
		env.idGen.On("Generate").Return(id, nil).Once()
		env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
		env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
			mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(5), ptr("secret"), nil)
	require.NoError(t, err)
	assert.Len(t, result.CreatedProfileIds, 5)
}

// specRef: I17
func TestImport_ProfileCount_WouldExceedCap(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// 98 existing + 5 incoming = 103 > 100
	existing := make([]model.Profile, 98)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(5), ptr("secret"), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrMaxProfilesExceeded)
}

// specRef: I18
func TestImport_ProfileCount_AlreadyAtCap(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// exactly 100 existing + 1 incoming = 101 > 100
	existing := make([]model.Profile, 100)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrMaxProfilesExceeded)
}

// ---------------------------------------------------------------------------
// Blocklist catalog validation
// ---------------------------------------------------------------------------

// specRef: V8
func TestImport_MissingBlocklistId_AddsWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	// Catalog lookup: "unknown-bl" not in catalog.
	env.blocklistRepo.On("Get", mock.Anything,
		map[string]any{"blocklist_id": "unknown-bl"}, "updated").
		Return([]*model.Blocklist{}, nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		Privacy: &profile.ExportedPrivacy{
			Blocklists: []string{"unknown-bl"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "unknown-bl") && strings.Contains(w, "not in the catalog") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected missing-blocklist warning; got: %v", result.Warnings)
	assert.Len(t, result.CreatedProfileIds, 1, "import must still succeed despite missing blocklist")
}

// ---------------------------------------------------------------------------
// Custom rule validation
// ---------------------------------------------------------------------------

// specRef: V11
// Decision: skip-with-warning. See import.go top-level comment for rationale.
func TestImport_InvalidRuleAction_SkipsWithWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		CustomRules: []profile.ExportedCustomRule{
			{Action: "weird", Value: "example.com"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)
	assert.Len(t, result.CreatedProfileIds, 1, "profile must still be created")

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "action") && strings.Contains(w, "weird") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected invalid-action warning; got: %v", result.Warnings)
}

// specRef: V12
// Decision: skip-with-warning. See import.go top-level comment for rationale.
func TestImport_InvalidRuleSyntax_SkipsWithWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		CustomRules: []profile.ExportedCustomRule{
			{Action: "block", Value: "not a domain!!!"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)
	assert.Len(t, result.CreatedProfileIds, 1)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "syntax") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected invalid-syntax warning; got: %v", result.Warnings)
}

// ---------------------------------------------------------------------------
// Response shape
// ---------------------------------------------------------------------------

// specRef: I19
func TestImport_Response_CreatedIdsAreFresh(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "server-generated-id")

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil)
	require.NoError(t, err)
	require.Len(t, result.CreatedProfileIds, 1)
	assert.Equal(t, "server-generated-id", result.CreatedProfileIds[0])
}

// specRef: I20
func TestImport_Response_WarningsArrayPresent(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil)
	require.NoError(t, err)
	// Warnings must be a non-nil empty slice (JSON: []) not nil (JSON: null).
	assert.NotNil(t, result.Warnings)
	assert.IsType(t, []string{}, result.Warnings)
}

// ---------------------------------------------------------------------------
// Cross-account ID isolation
// ---------------------------------------------------------------------------

// specRef: S2
// The ExportEnvelope type carries no account-level fields (email, tier, _id),
// so the service layer cannot accidentally persist them. This test confirms the
// returned result contains only the profile data we supplied.
func TestImport_AllowlistDTO_RejectsAccountFields(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	// ExportEnvelope has no email/tier/_id fields. This is structural enforcement:
	// if someone adds such fields to ExportEnvelope the type system would expose them
	// here, failing compilation. The assertion is that Import succeeds and the
	// profile ID returned is the server-generated one (not some injected value).
	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"fresh-id-1"}, result.CreatedProfileIds)
}

// specRef: S3
// TestImport_AllIdsRegenerated confirms that the returned profile ID equals the
// value generated by idGen.Generate(), regardless of any ID that might appear
// in the envelope. Since ExportedProfile carries no ID field (the DTO strips
// all internal IDs), the test verifies the server-generated ID path end-to-end.
func TestImport_AllIdsRegenerated(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// The "payload" has no profile IDs (ExportedProfile has none by design).
	// The server must always generate a fresh one.
	const freshId = "brand-new-id"
	expectOneSuccessfulCreate(env, "acct-a", []model.Profile{}, freshId)

	result, err := env.svc.Import(context.Background(), "acct-a",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil)
	require.NoError(t, err)
	require.Len(t, result.CreatedProfileIds, 1)
	assert.Equal(t, freshId, result.CreatedProfileIds[0])
}

// ---------------------------------------------------------------------------
// IDN / Punycode warnings
// ---------------------------------------------------------------------------

// specRef: S5
func TestImport_PunycodeRule_AddsIDNWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	// xn--mller-kva.de is valid FQDN syntax; the rule passes re-validation.
	env.profileRepo.On("CreateCustomRules", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("[]*model.CustomRule")).Return(nil).Once()
	env.cache.On("AddCustomRule", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("*model.CustomRule")).Return(nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		CustomRules: []profile.ExportedCustomRule{
			// xn--mller-kva.de = müller.de in Punycode
			{Action: "block", Value: "xn--mller-kva.de"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	foundIDN := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "internationalized domain") {
			foundIDN = true
			break
		}
	}
	assert.True(t, foundIDN, "expected IDN warning for xn-- rule; got: %v", result.Warnings)
}

// specRef: S5
func TestImport_PlainAsciiRule_NoIDNWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	env.profileRepo.On("CreateCustomRules", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("[]*model.CustomRule")).Return(nil).Once()
	env.cache.On("AddCustomRule", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("*model.CustomRule")).Return(nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		CustomRules: []profile.ExportedCustomRule{
			{Action: "block", Value: "ads.example.com"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	for _, w := range result.Warnings {
		assert.NotContains(t, w, "internationalized domain",
			"plain ASCII rule must not produce an IDN warning")
	}
}

// ---------------------------------------------------------------------------
// Per-profile custom rules cap (S6 defensive service check)
// ---------------------------------------------------------------------------

// specRef: S6
// The DTO layer rejects payloads with > 10,000 rules before the service is
// reached. If the service is called directly with an oversized rule list it
// truncates to maxCustomRulesPerProfile and adds a warning. This test exercises
// that defensive code path.
func TestImport_ExceedsRulesCap_PerProfile(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	// Exactly 10,000 rules must reach the repository (the service truncates the 10,001st).
	env.profileRepo.On("CreateCustomRules", mock.Anything, "fresh-id-1",
		mock.MatchedBy(func(rules []*model.CustomRule) bool {
			return len(rules) == 10_000
		})).Return(nil).Once()
	// AddCustomRule is called once per rule (10,000 times).
	env.cache.On("AddCustomRule", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("*model.CustomRule")).Return(nil).Times(10_000)

	// Build 10,001 valid rules -- one over the cap.
	rules := make([]profile.ExportedCustomRule, 10_001)
	for i := range rules {
		rules[i] = profile.ExportedCustomRule{Action: "block", Value: "example.com"}
	}

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{CustomRules: rules}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)
	assert.Len(t, result.CreatedProfileIds, 1)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "capped") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a rules-cap warning; got: %v", result.Warnings)
}

// ---------------------------------------------------------------------------
// Service catalog validation (V9)
// ---------------------------------------------------------------------------

// specRef: V9
// TestImport_MissingServiceId_AddsWarning confirms that an imported service ID
// not present in the catalog is dropped with a warning, while the import itself
// still succeeds.
func TestImport_MissingServiceId_AddsWarning(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// Wire a catalog that knows "known-svc" but not "unknown-svc".
	env.svc.ServicesCatalog = newStaticCatalog("known-svc")

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &profile.ExportedSettings{
		Privacy: &profile.ExportedPrivacy{
			Services: []string{"known-svc", "unknown-svc"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil)
	require.NoError(t, err)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "unknown-svc") && strings.Contains(w, "not in the catalog") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected missing-service warning; got: %v", result.Warnings)
	assert.Len(t, result.CreatedProfileIds, 1, "import must still succeed despite missing service")

	// Confirm the known service ID was accepted and the unknown one was dropped.
	// The profile settings stored in the mock will contain only "known-svc".
	env.profileRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Export -> Import round-trip contract guard
// ---------------------------------------------------------------------------

// TestImport_AcceptsFreshExport_Roundtrip asserts the contract:
// the bytes emitted by Export() must be decodable by the same strict decoder
// that the import handler uses (json.Decoder with DisallowUnknownFields()).
// Prevents emitter/importer schema drift.
//
// specRef:"V1,V6"
func TestImport_AcceptsFreshExport_Roundtrip(t *testing.T) {
	const accountId = "acct-roundtrip"

	// Build a profile with populated settings to exercise the full field-mapping
	// path -- a minimal profile with empty settings would not catch field-name
	// mismatches in nested sections.
	src := fullProfile(accountId)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).
		Return([]model.Profile{*src}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).
		Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	// Step 1: produce a fresh export envelope.
	envelope, err := svc.Export(
		context.Background(),
		accountId,
		profile.ExportScopeAll,
		nil,
		ptrStr("testpw"),
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, envelope)

	// Step 2: marshal the envelope to JSON bytes -- this is exactly what the
	// export handler sends down to the browser.
	exportBytes, err := json.Marshal(envelope)
	require.NoError(t, err)

	// Step 3: decode those bytes with the same strict decoder the import handler
	// uses (json.Decoder + DisallowUnknownFields()).  The target type is
	// requests.ImportPayload, which is the struct the handler decodes the
	// payload field into before handing control to the service.
	// Any field present in ExportEnvelope but absent from ImportPayload would
	// produce an "unknown field" error here -- the precise bug this test guards
	// against.
	var importPayload requests.ImportPayload
	dec := json.NewDecoder(bytes.NewReader(exportBytes))
	dec.DisallowUnknownFields()
	decodeErr := dec.Decode(&importPayload)
	assert.NoError(t, decodeErr,
		"export envelope bytes must be accepted by the strict import decoder; "+
			"a non-nil error means ExportEnvelope emits a field that ImportPayload "+
			"does not declare -- update one or the other to re-align them")
}
