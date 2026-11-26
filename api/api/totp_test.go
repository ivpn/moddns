package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/stretchr/testify/mock"
)

func (suite *AccountsAPITestSuite) postTotp(path, otp string) int {
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
	body, _ := json.Marshal(map[string]string{"otp": otp})
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	return resp.StatusCode
}

// Disable takes an authenticator code or a backup code as displayed.
// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableAcceptsBackupCode() {
	for _, otp := range []string{"123456", "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5", "gfuks z7kzb vpndn tmwms hakr5", strings.Repeat("A", 64)} {
		suite.Run(otp, func() {
			suite.SetupTest()
			suite.mockService.On("TotpDisable", mock.Anything, testAccountID, otp).Return(&model.Account{}, nil).Once()
			suite.Equal(http.StatusOK, suite.postTotp("/api/v1/accounts/mfa/totp/disable", otp))
		})
	}
}

// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableRejectsOversizedCode() {
	suite.Equal(http.StatusBadRequest, suite.postTotp("/api/v1/accounts/mfa/totp/disable", strings.Repeat("A", 65)))
}

// Confirming a new enrolment takes only an authenticator code.
// specRef: api-endpoint-behaviour.md E2
func (suite *AccountsAPITestSuite) TestConfirmKeepsAuthenticatorCodeLimit() {
	suite.Equal(http.StatusBadRequest, suite.postTotp("/api/v1/accounts/mfa/totp/enable/confirm", "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5"))
}

// A wrong or used code is a bad request on a valid session, not an expired session.
// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableWrongCodeIsBadRequest() {
	cases := map[string]struct {
		err    error
		status int
	}{
		"invalid code": {account.ErrInvalidTOTPCode, http.StatusBadRequest},
		"lockout":      {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpDisable", mock.Anything, testAccountID, "123456").Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postTotp("/api/v1/accounts/mfa/totp/disable", "123456"))
		})
	}
}

// Set-up takes the code as authenticator apps display it, separators and
// surrounding spaces included. specRef: api-endpoint-behaviour.md E2
func (suite *AccountsAPITestSuite) TestConfirmAcceptsSeparatedCode() {
	for _, otp := range []string{"123 456", "123 - 456", " 123 456 ", "1 2 3 4 5 6"} {
		suite.Run(otp, func() {
			suite.SetupTest()
			suite.mockService.On("TotpConfirm", mock.Anything, testAccountID, otp).Return(&model.TOTPBackup{}, nil).Once()
			suite.Equal(http.StatusOK, suite.postTotp("/api/v1/accounts/mfa/totp/enable/confirm", otp))
		})
	}
}

// A refused code at set-up is a bad request on a valid session, not an
// expired session. specRef: api-endpoint-behaviour.md E2
func (suite *AccountsAPITestSuite) TestConfirmRefusedCodeIsBadRequest() {
	cases := map[string]struct {
		err    error
		status int
	}{
		"incorrect code":     {account.ErrIncorrectOTP, http.StatusBadRequest},
		"already configured": {account.ErrTOTPAlreadyConfigured, http.StatusBadRequest},
		"lockout":            {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
		"internal error":     {errors.New("storage unavailable"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpConfirm", mock.Anything, testAccountID, "123456").Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postTotp("/api/v1/accounts/mfa/totp/enable/confirm", "123456"))
		})
	}
}

const regeneratePath = "/api/v1/accounts/mfa/totp/backup-codes/regenerate"

func (suite *AccountsAPITestSuite) postJSON(path string, body any) int {
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	return resp.StatusCode
}

func regenerateBody(otp string) map[string]string {
	return map[string]string{"otp": otp, "current_password": "Curr3nt-Passw0rd!"}
}

func regenerateReq(otp string) requests.TotpRegenerateReq {
	password := "Curr3nt-Passw0rd!"
	return requests.TotpRegenerateReq{OTP: otp, CurrentPassword: &password}
}

// Regenerating takes an authenticator code or a backup code as displayed, with
// the reauth, and returns the new codes. specRef: api-endpoint-behaviour.md E13
func (suite *AccountsAPITestSuite) TestRegenerateAcceptsEitherCode() {
	for _, otp := range []string{"123456", "123 456", "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5", "LegacyCodeAAAAA1", strings.Repeat("A", 64)} {
		suite.Run(otp, func() {
			suite.SetupTest()
			suite.mockService.On("TotpRegenerateBackupCodes", mock.Anything, testAccountID, regenerateReq(otp)).Return(&model.TOTPBackup{BackupCodes: []string{"NEW"}}, nil).Once()
			suite.Equal(http.StatusOK, suite.postJSON(regeneratePath, regenerateBody(otp)))
		})
	}
}

// The passkey reauth token is passed on instead of the password.
// specRef: api-endpoint-behaviour.md E13
func (suite *AccountsAPITestSuite) TestRegenerateAcceptsReauthToken() {
	token := "reauth-token"
	suite.mockService.On("TotpRegenerateBackupCodes", mock.Anything, testAccountID, requests.TotpRegenerateReq{OTP: "123456", ReauthToken: &token}).Return(&model.TOTPBackup{BackupCodes: []string{"NEW"}}, nil).Once()
	suite.Equal(http.StatusOK, suite.postJSON(regeneratePath, map[string]string{"otp": "123456", "reauth_token": token}))
}

// specRef: api-endpoint-behaviour.md E13
func (suite *AccountsAPITestSuite) TestRegenerateRejectsMalformedRequest() {
	cases := map[string]map[string]string{
		"empty code":       regenerateBody(""),
		"short code":       regenerateBody("12345"),
		"long code":        regenerateBody(strings.Repeat("A", 65)),
		"password + token": {"otp": "123456", "current_password": "p", "reauth_token": "t"},
	}
	for name, body := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.Equal(http.StatusBadRequest, suite.postJSON(regeneratePath, body))
		})
	}
}

// A refused reauth or code is a bad request on a valid session, not an expired
// session. specRef: api-endpoint-behaviour.md E13, E12
func (suite *AccountsAPITestSuite) TestRegenerateRefusals() {
	cases := map[string]struct {
		err    error
		status int
	}{
		"no reauth":      {reauth.ErrMissingAuthMethod, http.StatusBadRequest},
		"wrong password": {reauth.ErrInvalidPassword, http.StatusBadRequest},
		"wrong token":    {reauth.ErrInvalidReauthToken, http.StatusBadRequest},
		"expired token":  {reauth.ErrReauthTokenExpired, http.StatusBadRequest},
		"invalid code":   {account.ErrInvalidTOTPCode, http.StatusBadRequest},
		"2FA off":        {account.ErrTOTPNotConfigured, http.StatusBadRequest},
		"lockout":        {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
		"internal error": {errors.New("storage unavailable"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpRegenerateBackupCodes", mock.Anything, testAccountID, regenerateReq("123456")).Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postJSON(regeneratePath, regenerateBody("123456")))
		})
	}
}
