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

// Set-up takes a reauth token. specRef: api-endpoint-behaviour.md E1
func (suite *AccountsAPITestSuite) TestEnableNeedsReauth() {
	token := "reauth-token"
	suite.mockService.On("TotpEnable", mock.Anything, testAccountID, requests.TotpEnableReq{ReauthToken: &token}).Return(&model.TOTPNew{Secret: "S"}, nil).Once()
	suite.Equal(http.StatusOK, suite.postJSON(enablePath, map[string]string{"reauth_token": token}))
}

// A refused reauth is a bad request on a valid session, not an expired
// session. specRef: api-endpoint-behaviour.md E1
func (suite *AccountsAPITestSuite) TestEnableRefusals() {
	token := "reauth-token"
	cases := map[string]struct {
		err    error
		status int
	}{
		"no reauth":          {reauth.ErrMissingAuthMethod, http.StatusBadRequest},
		"wrong token":        {reauth.ErrInvalidReauthToken, http.StatusBadRequest},
		"expired token":      {reauth.ErrReauthTokenExpired, http.StatusBadRequest},
		"already configured": {account.ErrTOTPAlreadyConfigured, http.StatusBadRequest},
		"internal error":     {errors.New("storage unavailable"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("TotpEnable", mock.Anything, testAccountID, requests.TotpEnableReq{ReauthToken: &token}).Return(nil, tc.err).Once()
			suite.Equal(tc.status, suite.postJSON(enablePath, map[string]string{"reauth_token": token}))
		})
	}
}

const disablePath = "/api/v1/accounts/mfa/totp/disable"

func disableReq(otp string) requests.TotpDisableReq {
	token := "reauth-token"
	return requests.TotpDisableReq{OTP: otp, ReauthToken: &token}
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

// specRef: api-endpoint-behaviour.md E3
func (suite *AccountsAPITestSuite) TestDisableRejectsMalformedRequest() {
	cases := map[string]map[string]string{
		"long code": reauthBody(strings.Repeat("A", 65)),
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
		"no reauth":     {reauth.ErrMissingAuthMethod, http.StatusBadRequest},
		"wrong token":   {reauth.ErrInvalidReauthToken, http.StatusBadRequest},
		"expired token": {reauth.ErrReauthTokenExpired, http.StatusBadRequest},
		"invalid code":  {account.ErrInvalidTOTPCode, http.StatusBadRequest},
		"lockout":       {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
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

// reauthBody is a disable or regenerate body with a reauth token.
func reauthBody(otp string) map[string]string {
	return map[string]string{"otp": otp, "reauth_token": "reauth-token"}
}

func regenerateReq(otp string) requests.TotpRegenerateReq {
	token := "reauth-token"
	return requests.TotpRegenerateReq{OTP: otp, ReauthToken: &token}
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

// specRef: api-endpoint-behaviour.md E13
func (suite *AccountsAPITestSuite) TestRegenerateRejectsMalformedRequest() {
	cases := map[string]map[string]string{
		"empty code": reauthBody(""),
		"short code": reauthBody("12345"),
		"long code":  reauthBody(strings.Repeat("A", 65)),
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
