package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/libs/urlshort"
)

type SubscriptionAPITestSuite struct {
	suite.Suite
	mockService *mocks.Servicer
	mockDB      *mocks.Db
	validator   *validator.APIValidator
	config      *config.Config
}

func (suite *SubscriptionAPITestSuite) SetupSuite() {
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

func (suite *SubscriptionAPITestSuite) SetupTest() {
	suite.mockService = mocks.NewServicer(suite.T())
	suite.mockDB = mocks.NewDb(suite.T())
}

func (suite *SubscriptionAPITestSuite) createTestServer() *APIServer {
	// Create a service.Service struct with Store and SubscriptionServicer
	testService := service.Service{
		Store:                suite.mockDB,
		SubscriptionServicer: suite.mockService,
	}
	// Other dependencies required by NewServer
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

func (suite *SubscriptionAPITestSuite) TestGetSubscription_Success() {
	accountID := "507f1f77bcf86cd799439011"
	sessionToken := "test-session-token"
	// Auth middleware requires a valid session cookie; mock session retrieval
	suite.mockDB.On("GetSession", mock.Anything, sessionToken).Return(model.Session{AccountID: accountID}, true, nil)
	sub := &model.Subscription{}
	// Mock subscription service call
	suite.mockService.On("GetSubscription", mock.Anything, accountID).Return(sub, nil)

	server := suite.createTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sub", nil)
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: sessionToken})

	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *SubscriptionAPITestSuite) TestGetSubscription_Unauthorized() {
	server := suite.createTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sub", nil)
	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode)
}

func (suite *SubscriptionAPITestSuite) TestGetSubscription_NotFound() {
	accountID := "507f1f77bcf86cd799439011"
	sessionToken := "test-session-token"

	// Mock auth session & service call returning not found
	suite.mockDB.On("GetSession", mock.Anything, sessionToken).Return(model.Session{AccountID: accountID}, true, nil)
	suite.mockService.On("GetSubscription", mock.Anything, accountID).Return(nil, dbErrors.ErrSubscriptionNotFound)

	server := suite.createTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sub", nil)
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: sessionToken})

	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
}

// TestUpdateSubscription_NoBody verifies that PUT /sub/update with no request
// body succeeds, and the service is called with an empty subID (so the
// downstream signup webhook is skipped).
// specRef: SY7
func (suite *SubscriptionAPITestSuite) TestUpdateSubscription_NoBody() {
	accountID := "507f1f77bcf86cd799439011"
	sessionToken := "test-session-token"

	suite.mockDB.On("GetSession", mock.Anything, sessionToken).Return(model.Session{AccountID: accountID}, true, nil)
	sub := &model.Subscription{}
	suite.mockService.On("GetSubscription", mock.Anything, accountID).Return(sub, nil)
	suite.mockService.On(
		"UpdateSubscriptionFromPASession",
		mock.Anything,
		sub,
		"pa-session-cookie-value",
		"",
	).Return(nil).Once()

	server := suite.createTestServer()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sub/update", nil)
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: PASessionCookie, Value: "pa-session-cookie-value"})

	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

// TestUpdateSubscription_WithSubID verifies that a valid `subid` in the body
// is propagated to the service layer (which then fires the signup webhook).
// specRef: SY7
func (suite *SubscriptionAPITestSuite) TestUpdateSubscription_WithSubID() {
	accountID := "507f1f77bcf86cd799439011"
	sessionToken := "test-session-token"
	subID := "550e8400-e29b-41d4-a716-446655440000"

	suite.mockDB.On("GetSession", mock.Anything, sessionToken).Return(model.Session{AccountID: accountID}, true, nil)
	sub := &model.Subscription{}
	suite.mockService.On("GetSubscription", mock.Anything, accountID).Return(sub, nil)
	suite.mockService.On(
		"UpdateSubscriptionFromPASession",
		mock.Anything,
		sub,
		"pa-session-cookie-value",
		subID,
	).Return(nil).Once()

	server := suite.createTestServer()
	body, _ := json.Marshal(map[string]string{"subid": subID})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sub/update", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: PASessionCookie, Value: "pa-session-cookie-value"})

	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

// TestUpdateSubscription_InvalidSubID verifies that a non-UUID4 `subid` is
// rejected at validation with 400 and a `uuid4` tag in the error body.
// specRef: SY7
func (suite *SubscriptionAPITestSuite) TestUpdateSubscription_InvalidSubID() {
	accountID := "507f1f77bcf86cd799439011"
	sessionToken := "test-session-token"

	suite.mockDB.On("GetSession", mock.Anything, sessionToken).Return(model.Session{AccountID: accountID}, true, nil)
	// SubscriptionGuard middleware fetches the subscription before the handler
	// runs; the body validation in updateSubscription() rejects the request
	// after that, before UpdateSubscriptionFromPASession is reached.
	suite.mockService.On("GetSubscription", mock.Anything, accountID).Return(&model.Subscription{}, nil)

	server := suite.createTestServer()
	body := `{"subid": "not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sub/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: PASessionCookie, Value: "pa-session-cookie-value"})

	resp, err := server.App.Test(req, -1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(suite.T(), string(respBody), "uuid4")
}

func TestSubscriptionAPITestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionAPITestSuite))
}
