package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// The session limit is checked before the 2FA code, so a refused login does not
// use a backup code up; other sessions are removed only after the code passed.
// specRef: api-endpoint-behaviour.md A8a, E6
func (suite *AccountsAPITestSuite) TestLoginChecksSessionLimitBeforeTheCode() {
	acc := &model.Account{ID: primitive.NewObjectID(), Email: "user@example.com", MFA: model.MFASettings{TOTP: model.TotpSettings{Enabled: true}}}
	suite.Require().NoError(acc.SetPassword("Correct-Passw0rd!"))
	login := func(headers map[string]string) (int, string) {
		b, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "Correct-Passw0rd!"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := suite.createTestServer().App.Test(req, -1)
		suite.Require().NoError(err)
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(out)
	}

	suite.Run("no code: 2FA prompt first, nothing counted", func() {
		suite.SetupTest()
		suite.mockDB.On("GetAccountByEmail", mock.Anything, "user@example.com").Return(acc, nil).Once()
		status, body := login(nil)
		suite.Equal(http.StatusUnauthorized, status)
		suite.JSONEq(`{"error":"TOTP is required"}`, body)
	})

	suite.Run("at the limit: refused before the code is checked", func() {
		suite.SetupTest()
		suite.mockDB.On("GetAccountByEmail", mock.Anything, "user@example.com").Return(acc, nil).Once()
		suite.mockDB.On("CountSessionsByAccountID", mock.Anything, acc.ID.Hex()).Return(int64(suite.config.API.SessionLimit), nil).Once()
		status, _ := login(map[string]string{"x-mfa-code": "GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5"})
		suite.Equal(http.StatusTooManyRequests, status)
		suite.mockService.AssertNotCalled(suite.T(), "MfaCheck", mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("removing sessions: only after the code passed", func() {
		suite.SetupTest()
		suite.mockDB.On("GetAccountByEmail", mock.Anything, "user@example.com").Return(acc, nil).Once()
		suite.mockService.On("MfaCheck", mock.Anything, acc, mock.Anything).Return(account.ErrInvalidTOTPCode).Once()
		status, _ := login(map[string]string{"x-mfa-code": "000000", "x-sessions-remove": "true"})
		suite.Equal(http.StatusUnauthorized, status)
		suite.mockDB.AssertNotCalled(suite.T(), "DeleteSessionsByAccountID", mock.Anything, mock.Anything)
	})
}
