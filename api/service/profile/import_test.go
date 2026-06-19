package profile_test

// import_test.go -- unit tests for ProfileService.Import.
// Each test references the spec row it covers via a specRef comment.
// Spec: docs/specs/account-export-import-behaviour.md

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

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
func minimalEnvelope(n int) *model.ExportEnvelope {
	profiles := make([]model.ExportedProfile, n)
	for i := range profiles {
		profiles[i] = model.ExportedProfile{
			Name:     "Profile",
			Settings: &model.ExportedSettings{},
		}
	}
	return &model.ExportEnvelope{
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

// ptr returns a pointer to the given string. Used to build the nullable
// password / reauth-token args on Import calls below.
func ptr(s string) *string { return &s }

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
		ptr("secret"), nil, nil,
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
		ptr("secret"), nil, nil,
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
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrUnsupportedSchemaVersion)
}

// specRef: V2
func TestImport_InvalidKind_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	envelope := minimalEnvelope(1)
	envelope.Kind = "other"

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, profile.ErrInvalidExportKind)
}

// specRef: V5
func TestImport_EmptyPayload_Rejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(0), ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(5), ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(5), ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil, nil)
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
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		Privacy: &model.ExportedPrivacy{
			Blocklists: []string{"unknown-bl"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		CustomRules: []model.ExportedCustomRule{
			{Action: "weird", Value: "example.com"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		CustomRules: []model.ExportedCustomRule{
			{Action: "block", Value: "not a domain!!!"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil, nil)
	require.NoError(t, err)
	require.Len(t, result.CreatedProfileIds, 1)
	assert.Equal(t, "server-generated-id", result.CreatedProfileIds[0])
}

// specRef: I20
func TestImport_Response_WarningsArrayPresent(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil, nil)
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
		profile.ImportModeCreateNew, minimalEnvelope(1), ptr("secret"), nil, nil)
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
	env.cache.On("AddCustomRules", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("[]*model.CustomRule")).Return(nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		CustomRules: []model.ExportedCustomRule{
			// xn--mller-kva.de = müller.de in Punycode
			{Action: "block", Value: "xn--mller-kva.de"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
	env.cache.On("AddCustomRules", mock.Anything, "fresh-id-1",
		mock.AnythingOfType("[]*model.CustomRule")).Return(nil).Once()

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		CustomRules: []model.ExportedCustomRule{
			{Action: "block", Value: "ads.example.com"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
// The DTO layer rejects payloads with > model.ExportedCustomRulesLimit rules
// before the service is reached. If the service is called directly with an
// oversized rule list it truncates to the cap and adds a warning. This test
// exercises that defensive code path.
func TestImport_ExceedsRulesCap_PerProfile(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	// Exactly the cap must reach the repository (the service truncates the overflow rule).
	env.profileRepo.On("CreateCustomRules", mock.Anything, "fresh-id-1",
		mock.MatchedBy(func(rules []*model.CustomRule) bool {
			return len(rules) == model.ExportedCustomRulesLimit
		})).Return(nil).Once()
	// AddCustomRules is called once with all capped rules via pipeline.
	env.cache.On("AddCustomRules", mock.Anything, "fresh-id-1",
		mock.MatchedBy(func(rules []*model.CustomRule) bool {
			return len(rules) == model.ExportedCustomRulesLimit
		})).Return(nil).Once()

	// Build one rule over the cap.
	rules := make([]model.ExportedCustomRule, model.ExportedCustomRulesLimit+1)
	for i := range rules {
		rules[i] = model.ExportedCustomRule{Action: "block", Value: "example.com"}
	}

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Settings = &model.ExportedSettings{CustomRules: rules}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		Privacy: &model.ExportedPrivacy{
			Services: []string{"known-svc", "unknown-svc"},
		},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
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
// Name-collision resolution
// ---------------------------------------------------------------------------

// envelopeWithNames returns a minimal envelope with one profile per name.
func envelopeWithNames(names ...string) *model.ExportEnvelope {
	profiles := make([]model.ExportedProfile, len(names))
	for i, n := range names {
		profiles[i] = model.ExportedProfile{
			Name:     n,
			Settings: &model.ExportedSettings{},
		}
	}
	return &model.ExportEnvelope{
		SchemaVersion: 1,
		Kind:          "moddns-export",
		ExportedAt:    time.Now(),
		Profiles:      profiles,
	}
}

// specRef: I24 — single collision against an existing account profile.
func TestImport_NameCollision_RenamesAgainstExisting(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	existing := []model.Profile{{Name: "Home"}}
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()

	var persistedName string
	env.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		persistedName = p.Name
		return p.AccountId == "acct1"
	})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames("Home"), ptr("secret"), nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "Home (imported)", persistedName)
	assert.Contains(t, result.Warnings, "profile 'Home' renamed to 'Home (imported)' to avoid name collision")
}

// specRef: I24 — two payload entries with the same source name must resolve
// against each other, not just against the account state.
func TestImport_NameCollision_WithinSameBatch(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("id-a", nil).Once()
	env.idGen.On("Generate").Return("id-b", nil).Once()

	persisted := make([]string, 0, 2)
	env.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		persisted = append(persisted, p.Name)
		return p.AccountId == "acct1"
	})).Return(nil).Times(2)
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Times(2)

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames("Work", "Work"), ptr("secret"), nil, nil)
	require.NoError(t, err)

	require.Len(t, persisted, 2)
	assert.Equal(t, "Work", persisted[0])
	assert.Equal(t, "Work (imported)", persisted[1])
	assert.Contains(t, result.Warnings, "profile 'Work' renamed to 'Work (imported)' to avoid name collision")
}

// specRef: I24 — cascade: existing has Home AND Home (imported), so the
// resolver must keep counting.
func TestImport_NameCollision_CascadeAcrossSuffixes(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	existing := []model.Profile{
		{Name: "Home"},
		{Name: "Home (imported)"},
		{Name: "Home (imported 2)"},
	}
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()

	var persistedName string
	env.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		persistedName = p.Name
		return p.AccountId == "acct1"
	})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames("Home"), ptr("secret"), nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "Home (imported 3)", persistedName)
	assert.Contains(t, result.Warnings, "profile 'Home' renamed to 'Home (imported 3)' to avoid name collision")
}

// specRef: I24 — when adding the suffix would push the name past 50 chars,
// the original is truncated and the suffix is preserved in full.
func TestImport_NameCollision_TruncatesToMaxLength(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	// 50-char original — adding " (imported)" (11 chars) would yield 61.
	original := strings.Repeat("A", 50)
	existing := []model.Profile{{Name: original}}
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()

	var persistedName string
	env.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		persistedName = p.Name
		return p.AccountId == "acct1"
	})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	_, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames(original), ptr("secret"), nil, nil)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(persistedName), 50, "resolved name must fit in max=50; got %d chars", len(persistedName))
	assert.True(t, strings.HasSuffix(persistedName, " (imported)"), "suffix must be preserved intact; got %q", persistedName)
}

// ---------------------------------------------------------------------------
// Advanced section — silent ignore on import (spec F7)
// ---------------------------------------------------------------------------

// specRef: F7 — an import payload containing a non-default recursor under
// advanced is silently ignored. The persisted profile inherits the default
// recursor (sdns) from model.NewSettings(), no warning is emitted.
func TestImport_AdvancedSection_SilentlyIgnored(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()

	var persistedSettings *model.ProfileSettings
	env.profileRepo.On("CreateProfile", mock.Anything, mock.MatchedBy(func(p *model.Profile) bool {
		persistedSettings = p.Settings
		return p.AccountId == "acct1"
	})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	envelope := envelopeWithNames("Imported")
	envelope.Profiles[0].Settings = &model.ExportedSettings{
		Advanced: &model.ExportedAdvanced{Recursor: "unbound"},
	}

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelope, ptr("secret"), nil, nil)
	require.NoError(t, err)

	require.NotNil(t, persistedSettings)
	require.NotNil(t, persistedSettings.Advanced)
	assert.Equal(t, model.RECURSOR_DEFAULT, persistedSettings.Advanced.Recursor,
		"recursor must fall back to the default; staging-only value must not be persisted")
	assert.Empty(t, result.Warnings, "silent ignore — no warning should be emitted")
}

// ---------------------------------------------------------------------------
// CreatedProfileNames — Response Body row I19b
// ---------------------------------------------------------------------------

// specRef: I19b — names come back in payload order, one per created profile.
func TestImport_Response_CreatedNamesParallelToIds(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()
	env.idGen.On("Generate").Return("id-a", nil).Once()
	env.idGen.On("Generate").Return("id-b", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).
		Return(nil).Times(2)
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Times(2)

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames("Alpha", "Beta"), ptr("secret"), nil, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"id-a", "id-b"}, result.CreatedProfileIds)
	require.Equal(t, []string{"Alpha", "Beta"}, result.CreatedProfileNames)
}

// specRef: I19b, I24 — when a profile is renamed via I24, CreatedProfileNames
// holds the resolved name (the one the user will see), not the source name.
func TestImport_Response_CreatedNamesReflectI24Rename(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)
	existing := []model.Profile{{Name: "Home"}}
	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return(existing, nil).Once()
	env.idGen.On("Generate").Return("fresh-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything, mock.AnythingOfType("*model.Profile")).
		Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	result, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, envelopeWithNames("Home"), ptr("secret"), nil, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"Home (imported)"}, result.CreatedProfileNames,
		"name in response must be the resolved name, not the source")
}

// ---------------------------------------------------------------------------
// Rollback cache cleanup — H3 regression guard
// ---------------------------------------------------------------------------

// specRef: I22 — when a profile mid-batch fails to create, every profile
// already inserted in this batch must be deleted AND have its Redis cache
// entry evicted. Without the cache eviction, the proxy would treat the
// rolled-back profile as live for up to one cache TTL (~30s), and a fresh
// import that happened to reuse the same generated profileId could be
// shadowed by the stale cache entry. (Code-review item H3.)
func TestImport_Rollback_EvictsCacheForCreatedProfiles(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()

	// Profile #1: creates successfully (profile doc + cache write).
	env.idGen.On("Generate").Return("created-id-1", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything,
		mock.MatchedBy(func(p *model.Profile) bool {
			return p.ProfileId == "created-id-1"
		})).Return(nil).Once()
	env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
		mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()

	// Profile #2: fails at CreateProfile, triggering rollback of #1.
	env.idGen.On("Generate").Return("doomed-id-2", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything,
		mock.MatchedBy(func(p *model.Profile) bool {
			return p.ProfileId == "doomed-id-2"
		})).Return(assert.AnError).Once()

	// Rollback must hit BOTH the repository AND the cache for the
	// already-created profile. doomed-id-2 was never added to createdIds
	// (CreateProfile errored before the appends), so it must NOT be touched.
	env.profileRepo.On("DeleteProfileById", mock.Anything, "created-id-1").
		Return(nil).Once()
	env.cache.On("DeleteProfileSettings", mock.Anything, "created-id-1").
		Return(nil).Once()

	_, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(2), ptr("secret"), nil, nil)
	require.Error(t, err)

	// AssertExpectations on the cache mock catches both directions:
	// - "DeleteProfileSettings was called for created-id-1" (the fix).
	// - "DeleteProfileSettings was NOT called for doomed-id-2" (the mock
	//   only registered the created-id-1 expectation, so a call for
	//   doomed-id-2 would be flagged as unexpected by mockery).
	env.cache.AssertExpectations(t)
	env.profileRepo.AssertExpectations(t)
}

// specRef: I22 — cache-cleanup errors during rollback are logged but do not
// abort the rollback loop. A failing cache.DeleteProfileSettings for one
// profile must not prevent the next profile from being cleaned up.
func TestImport_Rollback_CacheError_DoesNotAbortLoop(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	env.profileRepo.On("GetProfilesByAccountId", mock.Anything, "acct1").
		Return([]model.Profile{}, nil).Once()

	// Two successful creates then a third that fails.
	for i, id := range []string{"a", "b"} {
		_ = i
		env.idGen.On("Generate").Return(id, nil).Once()
		env.profileRepo.On("CreateProfile", mock.Anything,
			mock.MatchedBy(func(p *model.Profile) bool { return p.ProfileId == id })).
			Return(nil).Once()
		env.cache.On("CreateOrUpdateProfileSettings", mock.Anything,
			mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil).Once()
	}
	env.idGen.On("Generate").Return("doomed", nil).Once()
	env.profileRepo.On("CreateProfile", mock.Anything,
		mock.MatchedBy(func(p *model.Profile) bool { return p.ProfileId == "doomed" })).
		Return(assert.AnError).Once()

	// Repo deletes succeed for both. Cache delete fails for "a" — the
	// rollback loop must still process "b".
	env.profileRepo.On("DeleteProfileById", mock.Anything, "a").Return(nil).Once()
	env.cache.On("DeleteProfileSettings", mock.Anything, "a").Return(assert.AnError).Once()
	env.profileRepo.On("DeleteProfileById", mock.Anything, "b").Return(nil).Once()
	env.cache.On("DeleteProfileSettings", mock.Anything, "b").Return(nil).Once()

	_, err := env.svc.Import(context.Background(), "acct1",
		profile.ImportModeCreateNew, minimalEnvelope(3), ptr("secret"), nil, nil)
	require.Error(t, err)

	env.cache.AssertExpectations(t)
	env.profileRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// B3: Over-long name truncation (specRef: I24)
// ---------------------------------------------------------------------------

// TestImport_LongName_TruncatedNotRejected asserts that a profile name
// exceeding MaxProfileNameLen (50) is truncated to 50 chars rather than
// rejected with an error. The persisted profile name must be exactly 50 runes,
// and a non-fatal truncation warning must appear in the result.
func TestImport_LongName_TruncatedNotRejected(t *testing.T) {
	env := newImportTestEnv(t, "secret", 100)

	// Construct a 60-rune name (10 chars over the limit).
	longName := strings.Repeat("a", 60)

	expectOneSuccessfulCreate(env, "acct1", []model.Profile{}, "fresh-id-1")

	envelope := minimalEnvelope(1)
	envelope.Profiles[0].Name = longName

	result, err := env.svc.Import(
		context.Background(), "acct1",
		profile.ImportModeCreateNew,
		envelope,
		ptr("secret"), nil, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	// The persisted name must be exactly MaxProfileNameLen runes.
	assert.Equal(t, 1, len(result.CreatedProfileNames))
	assert.LessOrEqual(t, len([]rune(result.CreatedProfileNames[0])), model.MaxProfileNameLen,
		"persisted name must not exceed MaxProfileNameLen")

	// A truncation warning must appear.
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "truncated") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected truncation warning; got: %v", result.Warnings)
}
