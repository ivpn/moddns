package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (suite *AccountsAPITestSuite) deleteOtherSessions(body map[string]string) int {
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	return resp.StatusCode
}

func (suite *AccountsAPITestSuite) sessionsAccount(totp bool, tokens ...model.Token) *model.Account {
	id, err := primitive.ObjectIDFromHex(testAccountID)
	suite.Require().NoError(err)
	acc := &model.Account{ID: id, Tokens: tokens}
	acc.MFA.TOTP.Enabled = totp
	return acc
}

// Signing out the other sessions takes a reauth token of purpose
// sessions_delete; with 2FA on the token alone is enough.
// specRef: api-endpoint-behaviour.md A3
func (suite *AccountsAPITestSuite) TestDeleteOtherSessionsNeedsReauth() {
	token := model.Token{Type: auth.TokenTypeReauthSessionsDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}
	for _, totp := range []bool{false, true} {
		suite.Run(map[bool]string{false: "2FA off", true: "2FA on"}[totp], func() {
			suite.SetupTest()
			suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(suite.sessionsAccount(totp, token), nil).Once()
			suite.mockDB.On("UpdateAccount", mock.Anything, mock.MatchedBy(func(a *model.Account) bool { return len(a.Tokens) == 0 })).Return(&model.Account{}, nil).Once()
			suite.mockDB.On("DeleteSessionsByAccountIDExceptCurrent", mock.Anything, testAccountID, testSessionToken).Return(nil).Once()
			suite.Equal(http.StatusNoContent, suite.deleteOtherSessions(map[string]string{"reauth_token": token.Value}))
		})
	}
}

// A refused verification signs nothing out and is a bad request on a valid
// session, not an expired session. specRef: api-endpoint-behaviour.md A3
func (suite *AccountsAPITestSuite) TestDeleteOtherSessionsRefusals() {
	other := model.Token{Type: auth.TokenTypeReauthPasskeyDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}
	expired := model.Token{Type: auth.TokenTypeReauthSessionsDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(-time.Minute)}
	cases := map[string]struct {
		body    map[string]string
		account *model.Account
	}{
		"no reauth":                {body: map[string]string{}},
		"current password":         {body: map[string]string{"current_password": "Curr3nt-Passw0rd!", "otp": "123456"}},
		"token of another purpose": {body: map[string]string{"reauth_token": other.Value}, account: suite.sessionsAccount(false, other)},
		"expired token":            {body: map[string]string{"reauth_token": expired.Value}, account: suite.sessionsAccount(false, expired)},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			if tc.account != nil {
				suite.mockDB.On("GetAccountById", mock.Anything, testAccountID).Return(tc.account, nil).Once()
			}
			suite.Equal(http.StatusBadRequest, suite.deleteOtherSessions(tc.body))
			suite.mockDB.AssertNotCalled(suite.T(), "DeleteSessionsByAccountIDExceptCurrent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
