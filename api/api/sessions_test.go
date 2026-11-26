package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

const sessionsPassword = "Curr3nt-Passw0rd!"

func (suite *AccountsAPITestSuite) deleteOtherSessions(body map[string]string, mfaCode string) int {
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
	if mfaCode != "" {
		body["otp"] = mfaCode
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	return resp.StatusCode
}

func (suite *AccountsAPITestSuite) sessionsAccount(totp bool, tokens ...model.Token) *model.Account {
	hash, err := bcrypt.GenerateFromPassword([]byte(sessionsPassword), bcrypt.MinCost)
	suite.Require().NoError(err)
	hashed := string(hash)
	id, err := primitive.ObjectIDFromHex(testAccountID)
	suite.Require().NoError(err)
	acc := &model.Account{ID: id, Password: &hashed, Tokens: tokens}
	acc.MFA.TOTP.Enabled = totp
	return acc
}

// Signing out the other sessions takes the current password (plus the 2FA
// code when TOTP is on) or a passkey reauth token of purpose sessions_delete.
// specRef: api-endpoint-behaviour.md A3
func (suite *AccountsAPITestSuite) TestDeleteOtherSessionsNeedsReauth() {
	token := model.Token{Type: auth.TokenTypeReauthSessionsDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}

	suite.Run("password", func() {
		suite.SetupTest()
		acc := suite.sessionsAccount(false)
		suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(acc, nil).Once()
		suite.mockService.On("MfaCheck", mock.Anything, acc, mock.Anything).Return(nil).Once()
		suite.mockDB.On("DeleteSessionsByAccountIDExceptCurrent", mock.Anything, testAccountID, testSessionToken).Return(nil).Once()
		suite.Equal(http.StatusNoContent, suite.deleteOtherSessions(map[string]string{"current_password": sessionsPassword}, ""))
	})

	suite.Run("password and 2FA code", func() {
		suite.SetupTest()
		acc := suite.sessionsAccount(true)
		suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(acc, nil).Once()
		suite.mockService.On("MfaCheck", mock.Anything, acc, &model.MfaData{OTP: "123456"}).Return(nil).Once()
		suite.mockDB.On("DeleteSessionsByAccountIDExceptCurrent", mock.Anything, testAccountID, testSessionToken).Return(nil).Once()
		suite.Equal(http.StatusNoContent, suite.deleteOtherSessions(map[string]string{"current_password": sessionsPassword}, "123456"))
	})

	suite.Run("passkey reauth token", func() {
		suite.SetupTest()
		suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(suite.sessionsAccount(true, token), nil).Once()
		suite.mockDB.On("UpdateAccount", mock.Anything, mock.MatchedBy(func(a *model.Account) bool { return len(a.Tokens) == 0 })).Return(&model.Account{}, nil).Once()
		suite.mockDB.On("DeleteSessionsByAccountIDExceptCurrent", mock.Anything, testAccountID, testSessionToken).Return(nil).Once()
		suite.Equal(http.StatusNoContent, suite.deleteOtherSessions(map[string]string{"reauth_token": token.Value}, ""))
	})
}

// A refused verification signs nothing out and is a bad request on a valid
// session, not an expired session. specRef: api-endpoint-behaviour.md A3
func (suite *AccountsAPITestSuite) TestDeleteOtherSessionsRefusals() {
	other := model.Token{Type: auth.TokenTypeReauthPasskeyDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}
	cases := map[string]struct {
		body    map[string]string
		code    string
		account *model.Account
		mfaErr  error
		status  int
	}{
		"no reauth":                {body: map[string]string{}, status: http.StatusBadRequest},
		"password + token":         {body: map[string]string{"current_password": "p", "reauth_token": "t"}, status: http.StatusBadRequest},
		"wrong password":           {body: map[string]string{"current_password": "wrong"}, account: suite.sessionsAccount(false), status: http.StatusBadRequest},
		"token of another purpose": {body: map[string]string{"reauth_token": other.Value}, account: suite.sessionsAccount(false, other), status: http.StatusBadRequest},
		"2FA code missing":         {body: map[string]string{"current_password": sessionsPassword}, account: suite.sessionsAccount(true), mfaErr: account.ErrTOTPRequired, status: http.StatusBadRequest},
		"wrong 2FA code":           {body: map[string]string{"current_password": sessionsPassword}, code: "000000", account: suite.sessionsAccount(true), mfaErr: account.ErrInvalidTOTPCode, status: http.StatusBadRequest},
		"2FA lockout":              {body: map[string]string{"current_password": sessionsPassword}, code: "000000", account: suite.sessionsAccount(true), mfaErr: account.ErrTOTPTooManyAttempts, status: http.StatusTooManyRequests},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			if tc.account != nil {
				suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(tc.account, nil).Once()
			}
			if tc.account != nil {
				suite.mockService.On("MfaCheck", mock.Anything, tc.account, mock.Anything).Return(tc.mfaErr).Maybe()
			}
			suite.Equal(tc.status, suite.deleteOtherSessions(tc.body, tc.code))
			suite.mockDB.AssertNotCalled(suite.T(), "DeleteSessionsByAccountIDExceptCurrent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
