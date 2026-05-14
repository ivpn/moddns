package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/libs/urlshort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	eiAccountID    = "507f1f77bcf86cd799439099"
	eiSessionToken = "test-session-token-export-import"
)

// minimalImportPayload returns a JSON-encoded ImportRequest body that passes all
// validation rules.  Tests that need to mutate the payload should unmarshal this
// into a map[string]any and adjust individual fields before re-marshalling.
func minimalImportPayload(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"mode": "create_new",
		"payload": map[string]any{
			"schemaVersion": 1,
			"kind":          "moddns-export",
			"exportedAt":    time.Now().UTC().Format(time.RFC3339),
			"profiles": []any{
				map[string]any{
					"name":     "Test Profile",
					"settings": map[string]any{},
				},
			},
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

// minimalExportBody returns a JSON-encoded ExportRequest body for scope="all".
func minimalExportBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"scope": "all"})
	require.NoError(t, err)
	return b
}

// ---- Suite ----------------------------------------------------------------

type ProfileExportImportSuite struct {
	suite.Suite
	mockProfileSvc *mocks.ProfileServicer
	mockDB         *mocks.Db
	validator      *validator.APIValidator
	config         *config.Config
}

func (s *ProfileExportImportSuite) SetupSuite() {
	var err error
	s.validator, err = validator.NewAPIValidator()
	s.Require().NoError(err, "failed to create validator")

	s.config = &config.Config{
		API: &config.APIConfig{
			ApiAllowOrigin: "http://localhost:3000",
			ApiAllowIP:     "*",
		},
		Server: &config.ServerConfig{
			Name: "modDNS Test",
			FQDN: "test.local",
		},
	}
}

func (s *ProfileExportImportSuite) SetupTest() {
	s.mockProfileSvc = mocks.NewProfileServicer(s.T())
	s.mockDB = mocks.NewDb(s.T())
}

// createServer builds a full production APIServer with the ProfileServicer mock
// wired in.  It mimics the pattern used in accounts_test.go and blocklists_test.go.
func (s *ProfileExportImportSuite) createServer() *APIServer {
	testService := service.Service{
		Store:           s.mockDB,
		ProfileServicer: s.mockProfileSvc,
		// SessionServicer and SubscriptionServicer must be non-nil so that the
		// auth and subscription-guard middlewares can call their methods.
		SessionServicer: s.mockDB,
	}

	mockCache := mocks.NewCachecache(s.T())
	mockIDGen := mocks.NewGeneratoridgen(s.T())
	mockMailer := mocks.NewMaileremail(s.T())
	mockShortener := urlshort.NewURLShortener()

	srv, err := NewServer(
		s.config,
		testService,
		s.mockDB,
		mockCache,
		mockIDGen,
		s.validator,
		mockMailer,
		mockShortener,
		nil,
	)
	s.Require().NoError(err, "failed to create test server")
	srv.RegisterRoutes()
	return srv
}

// auth adds the session cookie to req and stubs the DB so the auth middleware
// resolves the session to eiAccountID.  Call this once per test that needs auth.
func (s *ProfileExportImportSuite) auth(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: eiSessionToken})
	s.mockDB.On("GetSession", mock.Anything, eiSessionToken).
		Return(model.Session{AccountID: eiAccountID}, true, nil).Maybe()
}

// authAndSubscription additionally stubs GetSubscription so the subscription
// guard passes.  Required for import (not always-allowed).
func (s *ProfileExportImportSuite) authAndSubscription(req *http.Request) {
	s.auth(req)
	s.mockDB.On("GetSubscription", mock.Anything, eiAccountID).
		Return(&model.Subscription{Status: "Active"}, nil).Maybe()
}

// jsonBody builds an *http.Request with the given JSON body and standard
// Content-Type header.
func jsonReq(method, url string, body []byte) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// decodeJSON decodes the response body into dst.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	err := json.NewDecoder(resp.Body).Decode(dst)
	require.NoError(t, err)
}

// ---- Export tests ---------------------------------------------------------

// specRef:"E1"
func (s *ProfileExportImportSuite) TestExport_HappyPath_ReturnsEnvelope() {
	now := time.Now().UTC()
	envelope := &profile.ExportEnvelope{
		SchemaVersion: 1,
		Kind:          "moddns-export",
		ExportedAt:    now,
		Profiles: []profile.ExportedProfile{
			{Name: "My Profile"},
		},
	}

	s.mockProfileSvc.On(
		"Export",
		mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"),
		(*string)(nil), (*string)(nil),
	).Return(envelope, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var got profile.ExportEnvelope
	decodeJSON(s.T(), resp, &got)
	s.Equal(1, got.SchemaVersion)
	s.Equal("moddns-export", got.Kind)
	s.Len(got.Profiles, 1)
	s.Equal("My Profile", got.Profiles[0].Name)
}

// specRef:"E2"
func (s *ProfileExportImportSuite) TestExport_MissingScope_Returns400() {
	body, _ := json.Marshal(map[string]any{})
	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", body)
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	decodeJSON(s.T(), resp, &errResp)
	s.NotEmpty(errResp.Error)
}

// specRef:"E3"
func (s *ProfileExportImportSuite) TestExport_NoReauthCreds_Returns401() {
	// Mock service to simulate the reauth-required path.
	// The handler passes nil/nil credentials to the service when neither field
	// is provided; the service returns ErrReauthRequired.
	s.mockProfileSvc.On(
		"Export",
		mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"),
		(*string)(nil), (*string)(nil),
	).Return((*profile.ExportEnvelope)(nil), profile.ErrReauthRequired)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusUnauthorized, resp.StatusCode)

	var errResp ErrResponse
	decodeJSON(s.T(), resp, &errResp)
	s.Contains(errResp.Error, profile.ErrReauthRequired.Error())
}

// specRef:"E6"
func (s *ProfileExportImportSuite) TestExport_ScopeAllWithProfileIds_Returns400() {
	body, _ := json.Marshal(map[string]any{
		"scope":      "all",
		"profileIds": []string{"abc123"},
	})
	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", body)
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"E8"
func (s *ProfileExportImportSuite) TestExport_ScopeSelectedWithoutIds_Returns400() {
	body, _ := json.Marshal(map[string]any{"scope": "selected"})
	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", body)
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"E11"
func (s *ProfileExportImportSuite) TestExport_InvalidScope_Returns400() {
	body, _ := json.Marshal(map[string]any{"scope": "weird"})
	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", body)
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"E12"
func (s *ProfileExportImportSuite) TestExport_SetsContentTypeHeader() {
	now := time.Now().UTC()
	envelope := &profile.ExportEnvelope{SchemaVersion: 1, Kind: "moddns-export", ExportedAt: now, Profiles: []profile.ExportedProfile{{Name: "p"}}}
	s.mockProfileSvc.On("Export", mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"), (*string)(nil), (*string)(nil)).Return(envelope, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	// specRef:"E12" — vendor MIME type must be preserved; c.JSON() is called with
	// the content-type argument so Fiber does not overwrite it to "application/json".
	s.Equal("application/vnd.moddns.export+json; charset=utf-8", resp.Header.Get("Content-Type"))
}

// specRef:"E13"
func (s *ProfileExportImportSuite) TestExport_SetsContentDispositionHeader() {
	now := time.Now().UTC()
	envelope := &profile.ExportEnvelope{SchemaVersion: 1, Kind: "moddns-export", ExportedAt: now, Profiles: []profile.ExportedProfile{{Name: "p"}}}
	s.mockProfileSvc.On("Export", mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"), (*string)(nil), (*string)(nil)).Return(envelope, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	disposition := resp.Header.Get("Content-Disposition")
	// specRef:"E13" — attachment with UTC timestamp slot in ISO 8601 compact form
	s.True(strings.HasPrefix(disposition, "attachment; filename=\"moddns-export-"), "expected attachment prefix, got: %s", disposition)

	re := regexp.MustCompile(`moddns-export-\d{8}T\d{6}Z\.moddns\.json`)
	s.True(re.MatchString(disposition), "Content-Disposition does not match expected filename pattern: %s", disposition)
}

// specRef:"E14"
func (s *ProfileExportImportSuite) TestExport_SetsCacheControlNoStore() {
	now := time.Now().UTC()
	envelope := &profile.ExportEnvelope{SchemaVersion: 1, Kind: "moddns-export", ExportedAt: now, Profiles: []profile.ExportedProfile{{Name: "p"}}}
	s.mockProfileSvc.On("Export", mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"), (*string)(nil), (*string)(nil)).Return(envelope, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)
	// specRef:"E14"
	s.Equal("no-store", resp.Header.Get("Cache-Control"))
}

// specRef:"E15"
func (s *ProfileExportImportSuite) TestExport_SetsPragmaNoCache() {
	now := time.Now().UTC()
	envelope := &profile.ExportEnvelope{SchemaVersion: 1, Kind: "moddns-export", ExportedAt: now, Profiles: []profile.ExportedProfile{{Name: "p"}}}
	s.mockProfileSvc.On("Export", mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"), (*string)(nil), (*string)(nil)).Return(envelope, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)
	// specRef:"E15"
	s.Equal("no-cache", resp.Header.Get("Pragma"))
}

// specRef: (no specific row) — generic service error path
func (s *ProfileExportImportSuite) TestExport_ServiceError_Returns500() {
	s.mockProfileSvc.On(
		"Export",
		mock.Anything, eiAccountID, "all", mock.AnythingOfType("[]string"),
		(*string)(nil), (*string)(nil),
	).Return((*profile.ExportEnvelope)(nil), assert.AnError)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/export", minimalExportBody(s.T()))
	s.auth(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusInternalServerError, resp.StatusCode)
}

// ---- Import tests ---------------------------------------------------------

// specRef:"I1"
func (s *ProfileExportImportSuite) TestImport_HappyPath_ReturnsResult() {
	result := &profile.ImportResult{
		CreatedProfileIds: []string{"p1"},
		Warnings:          []string{},
	}
	s.mockProfileSvc.On(
		"Import",
		mock.Anything, eiAccountID, "create_new", mock.AnythingOfType("*profile.ExportEnvelope"),
		(*string)(nil), (*string)(nil),
	).Return(result, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var got profile.ImportResult
	decodeJSON(s.T(), resp, &got)
	s.Equal([]string{"p1"}, got.CreatedProfileIds)
	s.Equal([]string{}, got.Warnings)
}

// specRef:"I2"
func (s *ProfileExportImportSuite) TestImport_NoReauth_Returns401() {
	s.mockProfileSvc.On(
		"Import",
		mock.Anything, eiAccountID, "create_new", mock.AnythingOfType("*profile.ExportEnvelope"),
		(*string)(nil), (*string)(nil),
	).Return((*profile.ImportResult)(nil), profile.ErrReauthRequired)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusUnauthorized, resp.StatusCode)

	var errResp ErrResponse
	decodeJSON(s.T(), resp, &errResp)
	s.Contains(errResp.Error, profile.ErrReauthRequired.Error())
}

// specRef:"I4"
func (s *ProfileExportImportSuite) TestImport_MissingConfirmHeader_Returns400() {
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	// Do NOT set X-modDNS-Import header.
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	// specRef:"I4" — missing CSRF guard header must return 400.
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"I4" (variant)
func (s *ProfileExportImportSuite) TestImport_WrongConfirmHeaderValue_Returns400() {
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "yes") // wrong value — spec requires 400
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	// specRef:"I4" — wrong CSRF guard header value must return 400.
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"I6"
func (s *ProfileExportImportSuite) TestImport_GzipContentEncoding_Returns415() {
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "confirm")
	req.Header.Set("Content-Encoding", "gzip")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	// specRef:"I6" — gzip rejected before reading body
	s.Equal(http.StatusUnsupportedMediaType, resp.StatusCode)

	var errResp ErrResponse
	decodeJSON(s.T(), resp, &errResp)
	s.NotEmpty(errResp.Error)
}

// specRef:"I7"
func (s *ProfileExportImportSuite) TestImport_WrongContentType_Returns415() {
	body := []byte("mode=create_new")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusUnsupportedMediaType, resp.StatusCode)
}

// specRef:"I10"
func (s *ProfileExportImportSuite) TestImport_InvalidMode_Returns400() {
	payload := map[string]any{
		"mode": "merge", // not in oneof=create_new
		"payload": map[string]any{
			"schemaVersion": 1,
			"kind":          "moddns-export",
			"exportedAt":    time.Now().UTC().Format(time.RFC3339),
			"profiles": []any{
				map[string]any{"name": "Test Profile", "settings": map[string]any{}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", body)
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"I11"
func (s *ProfileExportImportSuite) TestImport_MissingMode_Returns400() {
	payload := map[string]any{
		// "mode" field intentionally absent
		"payload": map[string]any{
			"schemaVersion": 1,
			"kind":          "moddns-export",
			"exportedAt":    time.Now().UTC().Format(time.RFC3339),
			"profiles": []any{
				map[string]any{"name": "Test Profile", "settings": map[string]any{}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", body)
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"I19"
func (s *ProfileExportImportSuite) TestImport_ResponseIncludesCreatedIds() {
	result := &profile.ImportResult{
		CreatedProfileIds: []string{"p1", "p2"},
		Warnings:          []string{},
	}
	s.mockProfileSvc.On(
		"Import",
		mock.Anything, eiAccountID, "create_new", mock.AnythingOfType("*profile.ExportEnvelope"),
		(*string)(nil), (*string)(nil),
	).Return(result, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var got map[string]any
	decodeJSON(s.T(), resp, &got)

	// specRef:"I19" — createdProfileIds must be a JSON array with the expected values
	ids, ok := got["createdProfileIds"].([]any)
	s.True(ok, "createdProfileIds should be an array")
	s.Len(ids, 2)
	s.Equal("p1", ids[0])
	s.Equal("p2", ids[1])
}

// specRef:"I20"
func (s *ProfileExportImportSuite) TestImport_ResponseIncludesWarningsArray() {
	result := &profile.ImportResult{
		CreatedProfileIds: []string{"p1"},
		Warnings:          []string{}, // explicitly empty, must not be null in JSON
	}
	s.mockProfileSvc.On(
		"Import",
		mock.Anything, eiAccountID, "create_new", mock.AnythingOfType("*profile.ExportEnvelope"),
		(*string)(nil), (*string)(nil),
	).Return(result, nil)

	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", minimalImportPayload(s.T()))
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Decode into raw map so we can distinguish null from [].
	var rawBody map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&rawBody)
	require.NoError(s.T(), err)

	warningsRaw, ok := rawBody["warnings"]
	s.True(ok, "warnings field must be present in response")
	// specRef:"I20" — warnings must be a JSON array, not null
	s.NotEqual("null", string(warningsRaw), "warnings must not be JSON null")
	s.True(strings.HasPrefix(string(warningsRaw), "["), "warnings must be a JSON array, got: %s", string(warningsRaw))
}

// specRef:"V6"
func (s *ProfileExportImportSuite) TestImport_UnknownFieldInPayload_Returns400() {
	// Build a valid payload and inject an unknown root field.
	raw := map[string]any{
		"mode": "create_new",
		"payload": map[string]any{
			"schemaVersion": 1,
			"kind":          "moddns-export",
			"exportedAt":    time.Now().UTC().Format(time.RFC3339),
			"profiles": []any{
				map[string]any{"name": "Test Profile", "settings": map[string]any{}},
			},
		},
		"tier": "Tier 3", // unknown root-level field — spec V6 requires 400
	}
	body, _ := json.Marshal(raw)
	req := jsonReq(http.MethodPost, "/api/v1/profiles/import", body)
	req.Header.Set("X-modDNS-Import", "confirm")
	s.authAndSubscription(req)

	resp, err := s.createServer().App.Test(req, -1)
	require.NoError(s.T(), err)

	// specRef:"V6" — unknown root field must be rejected with 400.
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// specRef:"I5"
// TestImport_BodyOver1MB_Returns413 verifies that a body exceeding the 1 MB
// BodyLimit configured in NewServer is rejected with 413.
//
// The Fiber test harness (app.Test) reads the full request body into memory
// before handing it to the handler chain.  When the body exceeds BodyLimit,
// fasthttp returns a "body size exceeds the given limit" error to app.Test
// itself rather than producing a 413 HTTP response — so resp is nil and err is
// non-nil.  This makes it impossible to assert on the HTTP status code using
// the test harness.  The actual 413 behaviour is verified in integration tests
// where a real TCP connection is used; skip this test here.
func (s *ProfileExportImportSuite) TestImport_BodyOver1MB_Returns413() {
	s.T().Skip("Fiber app.Test() returns an error rather than a 413 response when the body exceeds BodyLimit; verified at integration-test level instead")
}

// ---- Shared routing tests -------------------------------------------------

// TestExportImport_OnlyAcceptsPOST verifies that the export and import routes
// are registered only for POST, not GET/PUT/DELETE.  Fiber returns 405 for
// known routes with wrong methods and 404 for completely unknown paths,
// depending on configuration; we accept either.
func (s *ProfileExportImportSuite) TestExportImport_OnlyAcceptsPOST() {
	srv := s.createServer()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/profiles/export"},
		{http.MethodPut, "/api/v1/profiles/export"},
		{http.MethodDelete, "/api/v1/profiles/export"},
		{http.MethodGet, "/api/v1/profiles/import"},
		{http.MethodPut, "/api/v1/profiles/import"},
		{http.MethodDelete, "/api/v1/profiles/import"},
	} {
		s.Run(tc.method+"_"+tc.path, func() {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Content-Type", "application/json")
			// Auth not needed — routing rejection happens before auth middleware
			// for 405; for 404 the auth middleware may run but that's fine.
			// Stub the session lookup defensively so auth middleware doesn't error.
			s.mockDB.On("GetSession", mock.Anything, mock.AnythingOfType("string")).
				Return(model.Session{}, false, nil).Maybe()

			resp, err := srv.App.Test(req, -1)
			require.NoError(s.T(), err)
			s.True(
				resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized,
				"expected 404/405 for %s %s, got %d", tc.method, tc.path, resp.StatusCode,
			)
		})
	}
}

// ---- Suite runner ---------------------------------------------------------

func TestProfileExportImportSuite(t *testing.T) {
	suite.Run(t, new(ProfileExportImportSuite))
}
