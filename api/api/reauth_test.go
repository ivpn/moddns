package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/stretchr/testify/mock"
)

const reauthPath = "/api/v1/accounts/reauth"

func (suite *AccountsAPITestSuite) postReauth(body any) *http.Response {
	suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, reauthPath, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	return resp
}

// The password (and code) is exchanged for a reauth token of the purpose.
// specRef: api-endpoint-behaviour.md B15
func (suite *AccountsAPITestSuite) TestReauth_IssuesToken() {
	expires := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	want := requests.ReauthRequest{Purpose: "email_change", CurrentPassword: "Curr3nt-Passw0rd!", OTP: "123456"}
	suite.mockService.On("Reauthenticate", mock.Anything, testAccountID, want).
		Return(&model.Token{Type: auth.TokenTypeReauthEmailChange, Value: "issued", ExpiresAt: expires}, nil).Once()

	resp := suite.postReauth(map[string]string{"purpose": "email_change", "current_password": "Curr3nt-Passw0rd!", "otp": "123456"})
	suite.Equal(http.StatusCreated, resp.StatusCode)
	var got responses.ReauthTokenResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&got))
	suite.Equal("issued", got.ReauthToken)
	suite.True(expires.Equal(got.ExpiresAt))
}

// specRef: api-endpoint-behaviour.md B15
func (suite *AccountsAPITestSuite) TestReauth_RejectsMalformedRequest() {
	cases := map[string]map[string]string{
		"no purpose":      {"current_password": "p"},
		"unknown purpose": {"purpose": "login", "current_password": "p"},
		"no password":     {"purpose": "email_change"},
		"long code":       {"purpose": "email_change", "current_password": "p", "otp": string(bytes.Repeat([]byte("A"), 65))},
	}
	for name, body := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.Equal(http.StatusBadRequest, suite.postReauth(body).StatusCode)
		})
	}
}

// A refused password or code is a bad request on a valid session, not an
// expired session; the per-account password limit and the 2FA lockout are 429.
// specRef: api-endpoint-behaviour.md B15, E12
func (suite *AccountsAPITestSuite) TestReauth_Refusals() {
	cases := map[string]struct {
		err    error
		status int
	}{
		"wrong password":  {reauth.ErrInvalidPassword, http.StatusBadRequest},
		"code required":   {account.ErrTOTPRequired, http.StatusBadRequest},
		"invalid code":    {account.ErrInvalidTOTPCode, http.StatusBadRequest},
		"password limit":  {account.ErrReauthRateLimited, http.StatusTooManyRequests},
		"2FA lockout":     {account.ErrTOTPTooManyAttempts, http.StatusTooManyRequests},
		"unknown purpose": {reauth.ErrUnknownPurpose, http.StatusBadRequest},
		"internal error":  {errors.New("storage unavailable"), http.StatusInternalServerError},
	}
	body := map[string]string{"purpose": "profile_export", "current_password": "p"}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("Reauthenticate", mock.Anything, testAccountID, requests.ReauthRequest{Purpose: "profile_export", CurrentPassword: "p"}).
				Return(nil, tc.err).Once()
			resp := suite.postReauth(body)
			suite.Equal(tc.status, resp.StatusCode)
			var errResp ErrResponse
			suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errResp))
			if tc.status != http.StatusInternalServerError {
				suite.Equal(tc.err.Error(), errResp.Error)
			}
		})
	}
}

// Without a session the endpoint is not reached.
func (suite *AccountsAPITestSuite) TestReauth_NeedsASession() {
	raw, _ := json.Marshal(map[string]string{"purpose": "email_change", "current_password": "p"})
	req := httptest.NewRequest(http.MethodPost, reauthPath, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := suite.createTestServer().App.Test(req, -1)
	suite.Require().NoError(err)
	suite.Equal(http.StatusUnauthorized, resp.StatusCode)
}

// The purposes a client may ask for are listed in the request validators and in
// reauth.Purposes; both reauth paths must accept the same set.
func (suite *AccountsAPITestSuite) TestReauthPurposesMatchTheTokenTypes() {
	for _, req := range []any{requests.ReauthRequest{}, requests.WebAuthnReauthBeginRequest{}} {
		field, ok := reflect.TypeOf(req).FieldByName("Purpose")
		suite.Require().True(ok)
		var listed []string
		for _, rule := range strings.Split(field.Tag.Get("validate"), ",") {
			if values, found := strings.CutPrefix(rule, "oneof="); found {
				listed = strings.Fields(values)
			}
		}
		suite.ElementsMatch(slices.Collect(maps.Keys(reauth.Purposes)), listed, reflect.TypeOf(req).Name())
	}
}
