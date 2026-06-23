package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/stretchr/testify/require"
)

// wrapFixtureAsImportBody turns a downloaded export file (a bare envelope, which
// is what the user uploads) into the import request body the frontend sends:
// {mode, payload: <envelope>, current_password}.
func wrapFixtureAsImportBody(t require.TestingT, file string) []byte {
	raw, err := os.ReadFile(filepath.Join(importFixturesDir, file))
	require.NoError(t, err)
	return mustJSON(t, map[string]any{
		"mode":             "create_new",
		"current_password": "secret",
		"payload":          json.RawMessage(raw),
	})
}

// importFixturesDir holds the hand-authored import files (api/api/testdata/import)
// used both for manual UI testing and by this test, which feeds each one through
// the real import handler and asserts the exact user-facing error it produces so
// the fixtures' documented messages stay honest (QA issue #604). The path is
// relative to the package directory (the test working dir).
const importFixturesDir = "testdata/import"

// serverFixtureMessages maps each server-reachable fixture (one that passes the
// dialog's client-side schemaVersion/kind/profiles pre-check and is therefore
// validated by the API) to the message body it produces. The handler prepends
// validationErrorPrefix ("Validation error: ") to this, which the assertions add.
var serverFixtureMessages = map[string]string{
	"01-too-many-profiles.moddns.json":                    "profiles must be at most 100",
	"02-too-many-custom-rules.moddns.json":                "customRules must be at most 1000",
	"03-too-many-blocklists.moddns.json":                  "blocklists must be at most 100",
	"04-invalid-default-rule.moddns.json":                 "defaultRule must be one of: block, allow",
	"05-invalid-blocklists-subdomains-rule.moddns.json":   "blocklistsSubdomainsRule must be one of: block, allow",
	"06-invalid-custom-rules-subdomains-rule.moddns.json": "customRulesSubdomainsRule must be one of: include, exact",
	"07-invalid-custom-rule-action.moddns.json":           "action must be one of: block, allow, comment",
	"08-custom-rule-value-too-long.moddns.json":           "value must be at most 255",
	"09-invalid-retention.moddns.json":                    "retention must be one of: 1h, 6h, 1d, 1w, 1m",
	"10-invalid-recursor.moddns.json":                     "recursor must be one of: sdns, unbound",
	"11-profile-name-too-long.moddns.json":                "name must be at most 200",
	"12-profile-name-invalid-chars.moddns.json":           "name contains invalid characters",
	"13-missing-exported-at.moddns.json":                  "exportedAt is required",
	"14-unknown-field-id.moddns.json":                     "Unknown field '_id' is not allowed.",
	"15-wrong-type-bool.moddns.json":                      "Field 'enabled' has the wrong type (expected true or false).",
}

func (s *ProfileExportImportSuite) TestManualImportFixtures_ProduceFriendlyMessages() {
	for file, want := range serverFixtureMessages {
		s.Run(file, func() {
			body := wrapFixtureAsImportBody(s.T(), file)

			req := jsonReq(http.MethodPost, "/api/v1/profiles/import", body)
			req.Header.Set("X-modDNS-Import", "confirm")
			s.authAndSubscription(req)

			resp, err := s.createServer().App.Test(req, -1)
			require.NoError(s.T(), err)
			s.Equal(http.StatusBadRequest, resp.StatusCode)

			var errResp ErrResponse
			decodeJSON(s.T(), resp, &errResp)
			s.Equal(validationErrorPrefix+want, errResp.Error)
			s.NotContains(errResp.Error, "Needs to implement")
			s.NotContains(errResp.Error, "json:")
		})
	}
}

// TestManualImportFixtures_MultipleErrors checks the file that violates two
// fields at once — it should mention both, with no raw validator text. The exact
// joined form is logged so the README can quote it verbatim.
func (s *ProfileExportImportSuite) TestManualImportFixtures_MultipleErrors() {
	body := wrapFixtureAsImportBody(s.T(), "16-multiple-errors.moddns.json")

	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", body)
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	decodeJSON(s.T(), resp, &errResp)
	s.T().Logf("16-multiple-errors message: %q", errResp.Error)
	s.True(strings.HasPrefix(errResp.Error, validationErrorPrefix), "message should carry the validation prefix")
	s.Contains(errResp.Error, "defaultRule must be one of: block, allow")
	s.Contains(errResp.Error, "action must be one of: block, allow, comment")
	s.NotContains(errResp.Error, "Needs to implement")
}

// TestManualImportFixtures_AllCovered guards against an undocumented fixture: every
// server-reachable file in the dir must have an expected message (client-* files
// are validated in the browser and intentionally excluded).
func (s *ProfileExportImportSuite) TestManualImportFixtures_AllCovered() {
	entries, err := os.ReadDir(importFixturesDir)
	require.NoError(s.T(), err)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".moddns.json") || strings.HasPrefix(name, "client-") {
			continue
		}
		if name == "16-multiple-errors.moddns.json" {
			continue
		}
		_, ok := serverFixtureMessages[name]
		s.True(ok, "fixture %s has no documented expected message", name)
	}
}
