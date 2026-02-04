package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/middleware"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
)

type WebAuthnAPITestSuite struct {
	suite.Suite
	validator *validator.APIValidator
	config    *config.Config
	mockDB    *mocks.Db
	mockCache *mocks.Cachecache
	mockServ  *mocks.Servicer
}

func TestWebAuthnAPITestSuite(t *testing.T) {
	suite.Run(t, new(WebAuthnAPITestSuite))
}

func (suite *WebAuthnAPITestSuite) SetupSuite() {
	var err error
	suite.validator, err = validator.NewAPIValidator()
	suite.Require().NoError(err)

	suite.config = &config.Config{
		API: &config.APIConfig{
			ApiAllowOrigin:        "http://localhost:3000",
			ApiAllowIP:            "*",
			SessionExpirationTime: 30 * time.Minute,
		},
		Service: &config.ServiceConfig{
			IdLimiterMax:        5,
			IdLimiterExpiration: time.Minute,
		},
		Server: &config.ServerConfig{
			Name: "modDNS Test",
			FQDN: "test.local",
		},
	}
}

func (suite *WebAuthnAPITestSuite) SetupTest() {
	suite.mockDB = mocks.NewDb(suite.T())
	suite.mockCache = mocks.NewCachecache(suite.T())
	suite.mockServ = mocks.NewServicer(suite.T())
}

func (suite *WebAuthnAPITestSuite) createServer() *APIServer {
	webAuthn, err := middleware.NewWebAuthn(*suite.config)
	suite.Require().NoError(err)

	srv := service.Service{
		Cfg:      *suite.config,
		Store:    suite.mockDB,
		Cache:    suite.mockCache,
		Webauthn: webAuthn,
	}
	srv.AccountServicer = suite.mockServ
	srv.SessionServicer = suite.mockServ

	return &APIServer{
		App:       fiber.New(),
		Service:   srv,
		Config:    suite.config,
		Validator: suite.validator,
	}
}

func readResponseBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return data
}

func (suite *WebAuthnAPITestSuite) TestBeginReauthSuccess() {
	server := suite.createServer()
	accountID := primitive.NewObjectID()

	acc := &model.Account{
		ID:    accountID,
		Email: "user@example.com",
	}

	credential := model.Credential{
		CredentialID: []byte{0x01, 0x02},
		PublicKey:    []byte{0xAA},
		Transport:    []string{"usb"},
		Flags: model.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: model.Authenticator{
			AAGUID:    []byte{0x01},
			SignCount: 1,
		},
	}

	suite.mockServ.On("GetAccount", mock.Anything, accountID.Hex()).Return(acc, nil).Once()
	suite.mockDB.On("GetCredentials", mock.Anything, accountID).Return([]model.Credential{credential}, nil).Once()
	suite.mockDB.On("SaveSession", mock.Anything, mock.Anything, mock.AnythingOfType("string"), accountID.Hex(), "email_change").Return(nil).Once()
	suite.mockCache.On("Incr", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Once()
	suite.mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("1", nil).Once()

	handler := server.beginReauth()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, accountID.Hex())
		return handler(c)
	})

	body, _ := json.Marshal(requests.WebAuthnReauthBeginRequest{Purpose: "email_change"})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	suite.Require().NoError(err)
	suite.Equal(200, resp.StatusCode)
	suite.Contains(resp.Header.Get("Set-Cookie"), WebAuthnTempCookie)

	respBody := readResponseBody(suite.T(), resp)
	var payload map[string]any
	suite.Require().NoError(json.Unmarshal(respBody, &payload))
	suite.Contains(payload, "publicKey")

	suite.mockServ.AssertExpectations(suite.T())
	suite.mockDB.AssertExpectations(suite.T())
	suite.mockCache.AssertExpectations(suite.T())
}

func (suite *WebAuthnAPITestSuite) TestBeginReauthValidationError() {
	server := suite.createServer()
	handler := server.beginReauth()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, primitive.NewObjectID().Hex())
		return handler(c)
	})

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{"purpose":"invalid"}`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	suite.Require().NoError(err)
	suite.Equal(400, resp.StatusCode)

	respBody := readResponseBody(suite.T(), resp)
	payload := make(map[string]any)
	suite.Require().NoError(json.Unmarshal(respBody, &payload))
	suite.Equal("validation failed", payload["error"])
	details, ok := payload["details"].([]any)
	suite.Require().True(ok)
	suite.Contains(details, "oneof")
}

func (suite *WebAuthnAPITestSuite) TestFinishReauthMissingCookie() {
	server := suite.createServer()
	handler := server.finishReauth()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, primitive.NewObjectID().Hex())
		return handler(c)
	})

	req := httptest.NewRequest("POST", "/", nil)

	resp, err := app.Test(req)
	suite.Require().NoError(err)
	suite.Equal(401, resp.StatusCode)
}

func (suite *WebAuthnAPITestSuite) TestFinishReauthSessionError() {
	server := suite.createServer()
	accountID := primitive.NewObjectID().Hex()

	suite.mockDB.On("GetSession", mock.Anything, "temp-token").Return(model.Session{}, false, errors.New("db error")).Once()

	handler := server.finishReauth()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, accountID)
		return handler(c)
	})

	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: WebAuthnTempCookie, Value: "temp-token"})

	resp, err := app.Test(req)
	suite.Require().NoError(err)
	suite.Equal(500, resp.StatusCode)

	payload := make(map[string]any)
	suite.Require().NoError(json.Unmarshal(readResponseBody(suite.T(), resp), &payload))
	suite.Equal("Failed to finish reauth", payload["error"])

	suite.mockDB.AssertExpectations(suite.T())
}
