package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/libs/urlshort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	testAccountID    = "507f1f77bcf86cd799439011"
	testSessionToken = "test-session-token-123"
)

type AccountsAPITestSuite struct {
	suite.Suite
	mockService *mocks.Servicer
	mockDB      *mocks.Db
	validator   *validator.APIValidator
	config      *config.Config
}

func (suite *AccountsAPITestSuite) SetupSuite() {
	// Create validator
	var err error
	suite.validator, err = validator.NewAPIValidator()
	suite.Require().NoError(err, "Failed to create validator")

	// Create config with all required settings for production middlewares
	suite.config = &config.Config{
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

func (suite *AccountsAPITestSuite) SetupTest() {
	// Create fresh mocks for each test to avoid state leakage
	suite.mockService = mocks.NewServicer(suite.T())
	suite.mockDB = mocks.NewDb(suite.T())
}

// createTestServer creates a full production API server with all middlewares and routes
func (suite *AccountsAPITestSuite) createTestServer() *APIServer {
	// Create a service.Service struct with both:
	// - Store field set to mockDB (for Service.GetSession to work)
	// - AccountServicer interface set to mockService (for UpdateAccount to work)
	testService := service.Service{
		Store:           suite.mockDB,
		AccountServicer: suite.mockService,
		SessionServicer: suite.mockService,
	}

	// Create other mock dependencies required by NewServer
	mockCache := mocks.NewCachecache(suite.T())
	mockIDGen := mocks.NewGeneratoridgen(suite.T())
	mockMailer := mocks.NewMaileremail(suite.T())
	mockShortener := urlshort.NewURLShortener()

	// Use the real production NewServer function
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

	// Register all production routes with real middlewares
	server.RegisterRoutes()

	return server
}

// TestUpdateAccount_SuccessfulEmailUpdate tests successful email update
func (suite *AccountsAPITestSuite) TestUpdateAccount_SuccessfulEmailUpdate() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "currentPass123!",
				"new_email":        "new@example.com",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request with session cookie
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// TestUpdateAccount_SuccessfulPasswordUpdate tests successful password update
func (suite *AccountsAPITestSuite) TestUpdateAccount_SuccessfulPasswordUpdate() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{Operation: "test", Path: "/password", Value: "CurrentPassword123!@#"},
		{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!@#"},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock session validation (required by auth middleware)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// TestUpdateAccount_SuccessfulErrorReportsConsentUpdate tests successful consent update
func (suite *AccountsAPITestSuite) TestUpdateAccount_SuccessfulErrorReportsConsentUpdate() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/error_reports_consent",
			Value:     true,
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock session validation (required by auth middleware)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// TestUpdateAccount_SameEmailAddress tests error when trying to update with same email
func (suite *AccountsAPITestSuite) TestUpdateAccount_SameEmailAddress() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "currentPass123!",
				"new_email":        "old@example.com",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrSameEmailAddress)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "new email address is the same as the current one")
}

// TestUpdateAccount_InvalidCurrentPassword tests error when current password is wrong
func (suite *AccountsAPITestSuite) TestUpdateAccount_InvalidCurrentPassword() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "wrongpassword",
				"new_email":        "new@example.com",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrInvalidCurrentPassword)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "invalid current password")
}

// TestUpdateAccount_InvalidNewEmail tests error when new email format is invalid
func (suite *AccountsAPITestSuite) TestUpdateAccount_InvalidNewEmail() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "currentPass123!",
				"new_email":        "invalid-email",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrInvalidNewEmail)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "invalid new email")
}

// TestUpdateAccount_MissingEmailUpdateFields tests error when required fields are missing
func (suite *AccountsAPITestSuite) TestUpdateAccount_MissingEmailUpdateFields() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"new_email": "new@example.com",
				// missing current_password
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrMissingEmailUpdateFields)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "missing current_password")
}

// TestUpdateAccount_InvalidRequestBody tests error when request body is malformed
func (suite *AccountsAPITestSuite) TestUpdateAccount_InvalidRequestBody() {
	// Mock session validation (auth middleware will be called before body parsing fails)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: testAccountID},
		true,
		nil,
	)

	// Create invalid JSON
	body := []byte(`{"updates": "invalid"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	// Note: Body parser errors return 500 with fiber's default error handler
	// but 400 with custom HandleError. Since we test the happy path works,
	// we just verify an error is returned
	assert.True(suite.T(), resp.StatusCode >= 400, "Expected error status code")

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.NotEmpty(suite.T(), errResp.Error)
}

// TestUpdateAccount_EmptyUpdates tests validation error when updates array is empty
func (suite *AccountsAPITestSuite) TestUpdateAccount_EmptyUpdates() {
	accountID := testAccountID
	updates := []model.AccountUpdate{}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock session validation (required by auth middleware)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call - empty updates are allowed
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// TestUpdateAccount_TOTPRequired tests error when TOTP is required but not provided
func (suite *AccountsAPITestSuite) TestUpdateAccount_TOTPRequired() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "currentPass123!",
				"new_email":        "new@example.com",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return TOTP required error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrTOTPRequired)

	// Create request without TOTP header
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "TOTP is required")
}

// TestUpdateAccount_InvalidTOTPCode tests error when TOTP code is invalid
func (suite *AccountsAPITestSuite) TestUpdateAccount_InvalidTOTPCode() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/email",
			Value: map[string]any{
				"current_password": "currentPass123!",
				"new_email":        "new@example.com",
			},
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return invalid TOTP error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(account.ErrInvalidTOTPCode)

	// Create request with TOTP header
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-mfa-code", "000000")
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "invalid 2FA code")
}

// TestUpdateAccount_ServiceError tests generic service error handling
func (suite *AccountsAPITestSuite) TestUpdateAccount_ServiceError() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/password",
			Value:     "NewPassword123!",
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock GetSession at DB level (auth middleware needs this)
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the service call to return generic error
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).
		Return(errors.New("database connection failed"))

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusInternalServerError, resp.StatusCode)

	var errResp ErrResponse
	assert.NoError(suite.T(), json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(suite.T(), errResp.Error, "failed to update account")
}

// TestUpdateAccount_MultipleUpdates tests multiple updates in single request
func (suite *AccountsAPITestSuite) TestUpdateAccount_MultipleUpdates() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{
			Operation: "replace",
			Path:      "/error_reports_consent",
			Value:     true,
		},
		{
			Operation: "add",
			Path:      "/profiles",
			Value:     "newprofile123",
		},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock session validation (required by auth middleware)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Create production server and execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// TestUpdateAccount_WithValidTOTP tests successful update with valid TOTP
func (suite *AccountsAPITestSuite) TestUpdateAccount_WithValidTOTP() {
	accountID := testAccountID
	updates := []model.AccountUpdate{
		{Operation: "test", Path: "/password", Value: "CurrentPassword123!@#"},
		{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!@#"},
	}

	payload := requests.AccountUpdates{
		Updates: updates,
	}

	// Mock session validation (required by auth middleware)
	// Create mockDB and mock GetSession at DB level (auth middleware needs this)

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)

	// Mock the UpdateAccount service call
	suite.mockService.On("UpdateAccount", mock.Anything, accountID, updates, mock.AnythingOfType("*model.MfaData")).Return(nil)

	// Create request with TOTP header
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-mfa-code", "123456")
	req.AddCookie(&http.Cookie{
		Name:  auth.AUTH_COOKIE,
		Value: testSessionToken,
	})

	// Execute request
	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNoContent, resp.StatusCode)
}

// -------- Registration handler tests --------

// helper to perform registration request
func (suite *AccountsAPITestSuite) doRegister(email, password, subID string) (*http.Response, error) {
	bodyMap := map[string]string{"email": email, "password": password, "subid": subID}
	b, _ := json.Marshal(bodyMap)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	server := suite.createTestServer()
	return server.App.Test(req, -1)
}

// Scenario 3: cache key exists, account does NOT exist, subscription absent -> success 201
func (suite *AccountsAPITestSuite) TestRegisterAccount_SuccessNewAccount() {
	email := "new@example.com"
	password := "StrongPass123!"                    // satisfies password validator
	subID := "550e8400-e29b-41d4-a716-446655440000" // uuid4

	// Mock service returning newly created finished account
	acc := &model.Account{ID: primitive.NewObjectID(), Email: email, Password: &password}
	suite.mockService.On("GetUnfinishedSignupOrPostAccount", mock.Anything, email, password, subID, mock.Anything).Return(acc, nil)

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode)
	var m map[string]string
	suite.NoError(json.NewDecoder(resp.Body).Decode(&m))
	suite.Equal("Account created successfully.", m["message"])
}

// Scenario 4A: cache key exists, unfinished account + subscription -> success 201
func (suite *AccountsAPITestSuite) TestRegisterAccount_SuccessUnfinishedReuse() {
	email := "unfinished@example.com"
	password := "StrongPass123!" // user now provides password to finish
	subID := "550e8400-e29b-41d4-a716-446655440001"

	// Unfinished account simulated by nil Password in returned account (service after reuse sets password internally)
	acc := &model.Account{ID: primitive.NewObjectID(), Email: email, Password: nil}
	suite.mockService.On("GetUnfinishedSignupOrPostAccount", mock.Anything, email, password, subID, mock.Anything).Return(acc, nil)

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusCreated, resp.StatusCode)
	var m map[string]string
	suite.NoError(json.NewDecoder(resp.Body).Decode(&m))
	suite.Equal("Account created successfully.", m["message"])
}

// Scenario 1: missing subscription cache key -> error 400 unified message
func (suite *AccountsAPITestSuite) TestRegisterAccount_ErrorCacheMissing() {
	email := "cachemiss@example.com"
	password := "StrongPass123!"
	subID := "550e8400-e29b-41d4-a716-446655440002"

	suite.mockService.On("GetUnfinishedSignupOrPostAccount", mock.Anything, email, password, subID, mock.Anything).Return(nil, account.ErrUnableToCreateAccount)

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	var errResp ErrResponse
	suite.NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	suite.Equal(account.ErrUnableToCreateAccount.Error(), errResp.Error)
}

// Scenario 2: subscription UUID already exists (duplicate) -> error 400 unified message
func (suite *AccountsAPITestSuite) TestRegisterAccount_ErrorSubscriptionDuplicate() {
	email := "dup@example.com"
	password := "StrongPass123!"
	subID := "550e8400-e29b-41d4-a716-446655440003"

	suite.mockService.On("GetUnfinishedSignupOrPostAccount", mock.Anything, email, password, subID, mock.Anything).Return(nil, account.ErrUnableToCreateAccount)

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	var errResp ErrResponse
	suite.NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	suite.Equal(account.ErrUnableToCreateAccount.Error(), errResp.Error)
}

// Scenario 4B: finished account reuse attempt -> error 400 unified message
func (suite *AccountsAPITestSuite) TestRegisterAccount_ErrorFinishedAccountReuse() {
	email := "finished@example.com"
	password := "StrongPass123!"
	subID := "550e8400-e29b-41d4-a716-446655440004"

	suite.mockService.On("GetUnfinishedSignupOrPostAccount", mock.Anything, email, password, subID, mock.Anything).Return(nil, account.ErrUnableToCreateAccount)

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	var errResp ErrResponse
	suite.NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	suite.Equal(account.ErrUnableToCreateAccount.Error(), errResp.Error)
}

// Validation failure: invalid UUID for subid -> 400 validation failed
func (suite *AccountsAPITestSuite) TestRegisterAccount_ValidationFailureInvalidUUID() {
	email := "valfail@example.com"
	password := "StrongPass123!"
	subID := "not-a-uuid"

	resp, err := suite.doRegister(email, password, subID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	var errResp ErrResponse
	suite.NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	suite.Equal("validation failed", errResp.Error)
	suite.Contains(errResp.Details, "uuid4")
}

// TestDeleteAccount_Success exercises the happy path: handler reaches Service.DeleteAccount
// (which the wrapper covers) and returns 204.
func (suite *AccountsAPITestSuite) TestDeleteAccount_Success() {
	accountID := testAccountID
	reauth := "reauth-token"
	payload := requests.AccountDeletionRequest{
		DeletionCode: "DELETE123",
		ReauthToken:  &reauth,
	}

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)
	suite.mockService.On(
		"DeleteAccount",
		mock.Anything,
		accountID,
		payload,
		mock.AnythingOfType("*model.MfaData"),
	).Return(nil)
	// Service.DeleteAccount wrapper invokes Store.DeleteSessionsByAccountID after
	// the inner deletion succeeds.
	suite.mockDB.On("DeleteSessionsByAccountID", mock.Anything, accountID).Return(nil)

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/current", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})

	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)
	suite.NoError(err)
	suite.Equal(http.StatusNoContent, resp.StatusCode)
}

// TestDeleteAccount_ServiceError surfaces a service-layer error (e.g. cleanup failure)
// as a non-2xx response carrying the unified error message.
func (suite *AccountsAPITestSuite) TestDeleteAccount_ServiceError() {
	accountID := testAccountID
	reauth := "reauth-token"
	payload := requests.AccountDeletionRequest{
		DeletionCode: "DELETE123",
		ReauthToken:  &reauth,
	}

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)
	suite.mockService.On(
		"DeleteAccount",
		mock.Anything,
		accountID,
		payload,
		mock.AnythingOfType("*model.MfaData"),
	).Return(errors.New("cleanup boom"))

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/current", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})

	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)
	suite.NoError(err)
	suite.NotEqual(http.StatusNoContent, resp.StatusCode)

	var errResp ErrResponse
	suite.NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	suite.Equal(ErrFailedToDeleteAccount.Error(), errResp.Error)
}

// TestDeleteAccount_ValidationFailure rejects a body missing the required deletion_code.
func (suite *AccountsAPITestSuite) TestDeleteAccount_ValidationFailure() {
	accountID := testAccountID

	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(
		model.Session{AccountID: accountID},
		true,
		nil,
	)
	// Service.DeleteAccount must never be called when validation fails (strict mock will fail otherwise).

	body, _ := json.Marshal(map[string]any{}) // no deletion_code
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/current", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})

	server := suite.createTestServer()
	resp, err := server.App.Test(req, -1)
	suite.NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func TestAccountsAPITestSuite(t *testing.T) {
	suite.Run(t, new(AccountsAPITestSuite))
}
