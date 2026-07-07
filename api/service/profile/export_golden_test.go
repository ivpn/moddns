package profile_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
)

// TestExport_GoldenEnvelope is a schema-drift / PII regression guard. It exports
// a fully-populated profile and compares the marshaled envelope byte-for-byte
// against a committed golden fixture (testdata/export/full-profile.golden.json).
//
// Why a golden snapshot in addition to the field-by-field and NotContains("email"
// /"password"/…) checks elsewhere: those only catch *known* bad substrings. The
// golden is a positive allowlist — any *new* field added to the export DTO (a
// potential account/PII leak) or any field rename/removal fails this test with an
// exact diff, forcing a deliberate review and fixture update.
//
// The two non-deterministic envelope fields (exportedAt = time.Now, exportedFrom
// = build-stamped version) are zeroed before comparison; everything under
// profiles[] — the security-critical surface — is asserted exactly. Custom-rule
// and profile IDs are already stripped by the exporter, so the output is stable.
//
// On an intentional export-shape change, delete the fixture (or update it by hand
// from the diff) and re-run: the test recreates it and fails once so the new
// contents get reviewed before being committed.
func TestExport_GoldenEnvelope(t *testing.T) {
	const accountId = "acct-golden"

	src := fullProfile(accountId)

	profileRepo := mocks.NewProfileRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo.On("GetProfilesByAccountId", context.Background(), accountId).
		Return([]model.Profile{*src}, nil)
	accountRepo.On("GetAccountById", context.Background(), accountId).
		Return(authorisedAccount(t), nil)

	svc := newExportSvc(t, profileRepo, accountRepo, config.ServiceConfig{MaxProfiles: 100})

	env, err := svc.Export(context.Background(), accountId, profile.ExportScopeAll, nil, ptrStr("testpw"), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, env)

	// Zero the non-deterministic metadata so the snapshot is stable.
	env.ExportedAt = time.Time{}
	env.ExportedFrom = nil

	actual, err := json.MarshalIndent(env, "", "  ")
	require.NoError(t, err)
	actual = append(actual, '\n')

	goldenPath := filepath.Join("testdata", "export", "full-profile.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o750))
		require.NoError(t, os.WriteFile(goldenPath, actual, 0o600))
		t.Fatalf("golden fixture %s did not exist — created it; review the contents (must contain no account/PII fields) and re-run", goldenPath)
	}

	assert.Equal(t, string(want), string(actual),
		"export envelope changed shape; if intentional, review for any leaked account/PII fields and update %s", goldenPath)
}
