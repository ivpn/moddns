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

const enablePath = "/api/v1/accounts/mfa/totp/enable"

// Set-up takes the current password or a passkey reauth token, not both.
// specRef: api-endpoint-behaviour.md E1
func (suite *AccountsAPITestSuite) TestEnableNeedsReauth() {
	password, token := "Curr3nt-Passw0rd!", "reauth-token"
	accepted := map[string]struct {
		body map[string]string
		req  requests.TotpEnableReq
	}{
		"password": {map[string]string{"current_password": password}, requests.TotpEnableReq{CurrentPassword: &password}},
		"token":    {map[string]string{"reauth_token": token}, requests.TotpEnableReq{ReauthToken: &token}},
	}
	for name, tc := range accepted {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpEnable", mock.Anything, testAccountID, tc.req).Return(&model.TOTPNew{Secret: "S"}, nil).Once()
			suite.Equal(http.StatusOK, suite.postJSON(enablePath, tc.body))
		})
	}
	suite.Run("password + token", func() {
		suite.SetupTest()
		suite.Equal(http.StatusBadRequest, suite.postJSON(enablePath, map[string]string{"current_password": "p", "reauth_token": "t"}))
	})
}

// A refused reauth is a bad request on a valid session, not an expired
// session. specRef: api-endpoint-behaviour.md E1
func (suite *AccountsAPITestSuite) TestEnableRefusals() {
	password := "Curr3nt-Passw0rd!"
	cases := map[string]struct {
		err    error
		status int
	}{
		"no reauth":          {reauth.ErrMissingAuthMethod, http.StatusBadRequest},
		"wrong password":     {reauth.ErrInvalidPassword, http.StatusBadRequest},
		"wrong token":        {reauth.ErrInvalidReauthToken, http.StatusBadRequest},
		"expired token":      {reauth.ErrReauthTokenExpired, http.StatusBadRequest},
		"already configured": {account.ErrTOTPAlreadyConfigured, http.StatusBadRequest},
		"internal error":     {errors.New("storage unavailable"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpEnable", mock.Anything, testAccountID, requests.TotpEnableReq{CurrentPassword: &password}).Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postJSON(enablePath, map[string]string{"current_password": password}))
		})
	}
}

const disablePath = "/api/v1/accounts/mfa/totp/disable"

func disableReq(otp string) requests.TotpDisableReq {
	password := "Curr3nt-Passw0rd!"
	return requests.TotpDisableReq{OTP: otp, CurrentPassword: &password}
}

// Disable takes an authenticator code or a backup code as displayed, with the
// reauth. specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableAcceptsBackupCode() {
	for _, otp := range []string{"123456", "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5", "gfuks z7kzb vpndn tmwms hakr5", strings.Repeat("A", 64)} {
		suite.Run(otp, func() {
			suite.SetupTest()
			suite.mockService.On("TotpDisable", mock.Anything, testAccountID, disableReq(otp)).Return(&model.Account{}, nil).Once()
			suite.Equal(http.StatusOK, suite.postJSON(disablePath, reauthBody(otp)))
		})
	}
}

// The passkey reauth token is passed on instead of the password.
// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableAcceptsReauthToken() {
	token := "reauth-token"
	suite.mockService.On("TotpDisable", mock.Anything, testAccountID, requests.TotpDisableReq{OTP: "123456", ReauthToken: &token}).Return(&model.Account{}, nil).Once()
	suite.Equal(http.StatusOK, suite.postJSON(disablePath, map[string]string{"otp": "123456", "reauth_token": token}))
}

// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableRejectsMalformedRequest() {
	cases := map[string]map[string]string{
		"long code":        reauthBody(strings.Repeat("A", 65)),
		"password + token": {"otp": "123456", "current_password": "p", "reauth_token": "t"},
	}
	for name, body := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.Equal(http.StatusBadRequest, suite.postJSON(disablePath, body))
		})
	}
}

// Confirming a new enrolment takes only an authenticator code.
// specRef: api-endpoint-behaviour.md E2
func (suite *AccountsAPITestSuite) TestConfirmKeepsAuthenticatorCodeLimit() {
	suite.Equal(http.StatusBadRequest, suite.postTotp("/api/v1/accounts/mfa/totp/enable/confirm", "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5"))
}

// A refused reauth or code is a bad request on a valid session, not an
// expired session. specRef: api-endpoint-behaviour.md E3, E12
func (suite *AccountsAPITestSuite) TestDisableRefusals() {
	cases := map[string]struct {
		err    error
		status int
	}{
		"no reauth":      {reauth.ErrMissingAuthMethod, http.StatusBadRequest},
		"wrong password": {reauth.ErrInvalidPassword, http.StatusBadRequest},
		"wrong token":    {reauth.ErrInvalidReauthToken, http.StatusBadRequest},
		"expired token":  {reauth.ErrReauthTokenExpired, http.StatusBadRequest},
		"invalid code":   {account.ErrInvalidTOTPCode, http.StatusBadRequest},
		"lockout":        {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpDisable", mock.Anything, testAccountID, disableReq("123456")).Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postJSON(disablePath, reauthBody("123456")))
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
		"no set-up started":  {account.ErrTOTPSetupNotStarted, http.StatusBadRequest},
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

// reauthBody is a disable or regenerate body verified with the current password.
func reauthBody(otp string) map[string]string {
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
			suite.Equal(http.StatusOK, suite.postJSON(regeneratePath, reauthBody(otp)))
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
		"empty code":       reauthBody(""),
		"short code":       reauthBody("12345"),
		"long code":        reauthBody(strings.Repeat("A", 65)),
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
			suite.Equal(tc.status, suite.postJSON(regeneratePath, reauthBody("123456")))
		})
	}
}
