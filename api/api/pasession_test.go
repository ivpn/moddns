package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/ivpn/dns/libs/urlshort"
)

type PASessionAPITestSuite struct {
	suite.Suite
	mockService *mocks.Servicer
	mockDB      *mocks.Db
	validator   *validator.APIValidator
	config      *config.Config
}

func (suite *PASessionAPITestSuite) SetupSuite() {
	var err error
	suite.validator, err = validator.NewAPIValidator()
	suite.Require().NoError(err)
	suite.config = &config.Config{
		API: &config.APIConfig{
			ApiAllowOrigin: "http://localhost:3000",
			ApiAllowIP:     "*",
			PSK:            "test-psk-token",
		},
		Server:  &config.ServerConfig{Name: "modDNS Test", FQDN: "test.local"},
		Service: &config.ServiceConfig{},
	}
}

func (suite *PASessionAPITestSuite) SetupTest() {
	suite.mockService = mocks.NewServicer(suite.T())
	suite.mockDB = mocks.NewDb(suite.T())
}

func (suite *PASessionAPITestSuite) createTestServer() *APIServer {
	testService := service.Service{
		Store:                suite.mockDB,
		SubscriptionServicer: suite.mockService,
	}
	mockCache := mocks.NewCachecache(suite.T())
	mockIDGen := mocks.NewGeneratoridgen(suite.T())
	mockMailer := mocks.NewMaileremail(suite.T())
	mockShortener := urlshort.NewURLShortener()

	server, err := NewServer(
		suite.config,
		testService,
		suite.mockDB,
		mockCache,
		mockIDGen,
		suite.validator,
		mockMailer,
		mockShortener,
		nil,
	)
	suite.Require().NoError(err, "Failed to create test server")
	server.RegisterRoutes()
	return server
}

func putRotateRequest(body string, cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/pasession/rotate",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

// TestRotate_FreshSessionRotates is the happy path: the URL sessionid is
// in the cache, RotatePASessionID returns a new ID, the handler sets the
// pa_session cookie and returns 200.
func (suite *PASessionAPITestSuite) TestRotate_FreshSessionRotates() {
	oldID := "c259d7e2-a8c4-4817-aac5-b0cd2fcddfad"
	newID := "8f0a1b2c-3d4e-4f5a-6b7c-8d9e0f1a2b3c"

	suite.mockService.
		On("RotatePASessionID", mock.Anything, oldID).
		Return(newID, nil)

	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(`{"sessionid":"`+oldID+`"}`),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	// New cookie must be set with the rotated ID.
	var pa *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == PASessionCookie {
			pa = c
			break
		}
	}
	suite.Require().NotNil(pa, "expected pa_session cookie to be set")
	assert.Equal(suite.T(), newID, pa.Value)
	assert.True(suite.T(), pa.HttpOnly)
}

// TestRotate_AlreadyRotated_NoCookie reproduces the original bug pre-fix:
// the URL sessionid was already consumed and the caller has no pa_session
// cookie. The handler must return 400 — there's no way to recover.
func (suite *PASessionAPITestSuite) TestRotate_AlreadyRotated_NoCookie() {
	oldID := "c259d7e2-a8c4-4817-aac5-b0cd2fcddfad"

	suite.mockService.
		On("RotatePASessionID", mock.Anything, oldID).
		Return("", subscription.ErrPASessionNotFound)

	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(`{"sessionid":"`+oldID+`"}`),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// TestRotate_AlreadyRotated_CookieValid is the fix: URL sessionid is gone
// but the caller still holds a valid pa_session cookie (typical when a
// signup link is reopened in the same browser). The handler must return
// 200 without rewriting the cookie.
func (suite *PASessionAPITestSuite) TestRotate_AlreadyRotated_CookieValid() {
	oldID := "c259d7e2-a8c4-4817-aac5-b0cd2fcddfad"
	cookieID := "8f0a1b2c-3d4e-4f5a-6b7c-8d9e0f1a2b3c"

	suite.mockService.
		On("RotatePASessionID", mock.Anything, oldID).
		Return("", subscription.ErrPASessionNotFound)
	suite.mockService.
		On("ValidateAndGetPreauth", mock.Anything, cookieID).
		Return(&model.Preauth{}, nil)

	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(
			`{"sessionid":"`+oldID+`"}`,
			&http.Cookie{Name: PASessionCookie, Value: cookieID},
		),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	// The handler must NOT overwrite the existing cookie on the fallback path.
	for _, c := range resp.Cookies() {
		if c.Name == PASessionCookie {
			suite.Failf("unexpected cookie write", "got Set-Cookie pa_session=%q on fallback path", c.Value)
		}
	}
}

// TestRotate_AlreadyRotated_CookieAlsoExpired covers the case where the
// caller presents a stale cookie that no longer maps to a cache entry.
// Both paths fail → 400.
func (suite *PASessionAPITestSuite) TestRotate_AlreadyRotated_CookieAlsoExpired() {
	oldID := "c259d7e2-a8c4-4817-aac5-b0cd2fcddfad"
	staleCookieID := "8f0a1b2c-3d4e-4f5a-6b7c-8d9e0f1a2b3c"

	suite.mockService.
		On("RotatePASessionID", mock.Anything, oldID).
		Return("", subscription.ErrPASessionNotFound)
	suite.mockService.
		On("ValidateAndGetPreauth", mock.Anything, staleCookieID).
		Return(nil, subscription.ErrPASessionNotFound)

	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(
			`{"sessionid":"`+oldID+`"}`,
			&http.Cookie{Name: PASessionCookie, Value: staleCookieID},
		),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// TestRotate_MalformedBody is unchanged from the original behaviour; the
// fallback should not run because the request itself is invalid.
func (suite *PASessionAPITestSuite) TestRotate_MalformedBody() {
	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(`{not json`),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// TestRotate_NonUUIDSessionID exercises the validator: the rotate service
// must NOT be called for a malformed sessionid, and the cookie fallback
// must NOT be attempted.
func (suite *PASessionAPITestSuite) TestRotate_NonUUIDSessionID() {
	server := suite.createTestServer()
	resp, err := server.App.Test(
		putRotateRequest(`{"sessionid":"not-a-uuid"}`),
		-1,
	)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

func TestPASessionAPITestSuite(t *testing.T) {
	suite.Run(t, new(PASessionAPITestSuite))
}
