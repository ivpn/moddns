package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ivpn/dns/api/internal/auth"
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
