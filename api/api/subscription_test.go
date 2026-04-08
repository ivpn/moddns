package api

import (
	"net/http"
	"net/http/httptest"
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

func TestSubscriptionAPITestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionAPITestSuite))
}
