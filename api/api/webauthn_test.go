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
	"golang.org/x/crypto/bcrypt"

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

// specRef: F5
func (suite *WebAuthnAPITestSuite) TestBeginReauthSuccess() {
	for _, purpose := range []string{"email_change", "account_deletion", "profile_export", "profile_import", "password_set", "passkey_add"} {
		suite.Run(purpose, func() {
			suite.SetupTest()
			suite.beginReauthSucceeds(purpose)
		})
	}
}

func (suite *WebAuthnAPITestSuite) beginReauthSucceeds(purpose string) {
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
	suite.mockCache.On("SaveCeremony", mock.Anything, mock.AnythingOfType("string"), mock.MatchedBy(func(c *model.Ceremony) bool {
		return c.Type == model.CeremonyReauth && c.AccountID == accountID.Hex() && c.Purpose == purpose
	}), model.CeremonyTimeout).Return(nil).Once()
	suite.mockCache.On("Incr", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Once()
	suite.mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("1", nil).Once()

	handler := server.beginReauth()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, accountID.Hex())
		return handler(c)
	})

	body, _ := json.Marshal(requests.WebAuthnReauthBeginRequest{Purpose: purpose})
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

	suite.mockCache.On("ConsumeCeremony", mock.Anything, "temp-token").Return(nil, errors.New("cache error")).Once()

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

	suite.mockCache.AssertExpectations(suite.T())
}

// specRef: F10
func (suite *WebAuthnAPITestSuite) TestFinishRejectsStateFromAnotherCeremonyOrAccount() {
	accountID := primitive.NewObjectID().Hex()
	otherAccountID := primitive.NewObjectID().Hex()

	tests := []struct {
		name     string
		handler  func(*APIServer) fiber.Handler
		ceremony model.Ceremony
	}{
		{"register finish with login state", (*APIServer).finishRegistration, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyLogin}},
		{"register finish with add-passkey state", (*APIServer).finishRegistration, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyAddPasskey}},
		{"register finish with untyped state", (*APIServer).finishRegistration, model.Ceremony{AccountID: otherAccountID}},
		{"login finish with registration state", (*APIServer).finishLogin, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyRegistration}},
		{"login finish with reauth state", (*APIServer).finishLogin, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyReauth, Purpose: "email_change"}},
		{"add-passkey finish with login state", (*APIServer).finishAddPasskey, model.Ceremony{AccountID: accountID, Type: model.CeremonyLogin}},
		{"add-passkey finish with another account's state", (*APIServer).finishAddPasskey, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyAddPasskey}},
		{"reauth finish with login state", (*APIServer).finishReauth, model.Ceremony{AccountID: accountID, Type: model.CeremonyLogin}},
		{"reauth finish with another account's state", (*APIServer).finishReauth, model.Ceremony{AccountID: otherAccountID, Type: model.CeremonyReauth, Purpose: "email_change"}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			server := suite.createServer()
			ceremony := tt.ceremony
			suite.mockCache.On("ConsumeCeremony", mock.Anything, "temp-token").Return(&ceremony, nil).Once()

			handler := tt.handler(server)
			app := fiber.New()
			app.Post("/", func(c *fiber.Ctx) error {
				c.Locals(auth.ACCOUNT_ID, accountID)
				return handler(c)
			})

			req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: WebAuthnTempCookie, Value: "temp-token"})

			resp, err := app.Test(req)
			suite.Require().NoError(err)
			suite.Equal(400, resp.StatusCode)
			suite.mockCache.AssertExpectations(suite.T())
		})
	}
}

// specRef: F3
func (suite *WebAuthnAPITestSuite) TestBeginAddPasskeyRequiresVerification() {
	accountID := primitive.NewObjectID()
	hash, err := bcrypt.GenerateFromPassword([]byte("CurrentPass123!"), bcrypt.MinCost)
	suite.Require().NoError(err)
	password := string(hash)
	freshToken := func(tokenType string) model.Token {
		return model.Token{Type: tokenType, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}
	}

	tests := []struct {
		name     string
		body     string
		account  model.Account
		verified bool
		mfaCheck bool
	}{
		{name: "no verification", body: `{}`, account: model.Account{Password: &password}},
		{name: "wrong password", body: `{"current_password":"wrong"}`, account: model.Account{Password: &password}, mfaCheck: true},
		{name: "token for another purpose", body: `{"reauth_token":"reauth-token"}`, account: model.Account{Tokens: []model.Token{freshToken(auth.TokenTypeReauthEmailChange)}}},
		{name: "current password", body: `{"current_password":"CurrentPass123!"}`, account: model.Account{Password: &password}, verified: true, mfaCheck: true},
		{name: "passkey_add reauth token", body: `{"reauth_token":"reauth-token"}`, account: model.Account{Tokens: []model.Token{freshToken(auth.TokenTypeReauthPasskeyAdd)}}, verified: true},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			server := suite.createServer()
			acc := tt.account
			acc.ID = accountID
			acc.Email = "user@example.com"

			suite.mockServ.On("GetAccount", mock.Anything, accountID.Hex()).Return(&acc, nil).Maybe()
			suite.mockDB.On("GetAccountById", mock.Anything, accountID.Hex()).Return(&acc, nil).Maybe()
			if tt.mfaCheck {
				suite.mockServ.On("MfaCheck", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			}
			if tt.verified {
				suite.mockDB.On("UpdateAccount", mock.Anything, mock.Anything).Return(&acc, nil).Maybe()
				suite.mockDB.On("GetCredentials", mock.Anything, accountID).Return([]model.Credential{}, nil).Once()
				suite.mockCache.On("SaveCeremony", mock.Anything, mock.AnythingOfType("string"), mock.MatchedBy(func(c *model.Ceremony) bool {
					return c.Type == model.CeremonyAddPasskey && c.AccountID == accountID.Hex()
				}), model.CeremonyTimeout).Return(nil).Once()
			}

			handler := server.beginAddPasskey()
			app := fiber.New()
			app.Post("/", func(c *fiber.Ctx) error {
				c.Locals(auth.ACCOUNT_ID, accountID.Hex())
				return handler(c)
			})
			req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			suite.Require().NoError(err)
			if tt.verified {
				suite.Equal(200, resp.StatusCode)
				suite.Empty(acc.Tokens, "the reauth token is single-use")
			} else {
				suite.Equal(400, resp.StatusCode)
				suite.mockCache.AssertNotCalled(suite.T(), "SaveCeremony", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			suite.mockServ.AssertExpectations(suite.T())
			suite.mockDB.AssertExpectations(suite.T())
			suite.mockCache.AssertExpectations(suite.T())
		})
	}
}
