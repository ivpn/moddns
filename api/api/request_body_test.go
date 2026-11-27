package api

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/mock"
)

// A body the parser cannot read (none, an unknown content type, malformed or
// mistyped JSON) is a bad request, not a server error.
// specRef: api-endpoint-behaviour.md L1
func (suite *AccountsAPITestSuite) TestUnreadableBodyIsBadRequest() {
	routes := []struct{ method, path string }{
		{http.MethodDelete, "/api/v1/sessions"},
		{http.MethodPost, "/api/v1/accounts/mfa/totp/enable"},
		{http.MethodDelete, "/api/v1/accounts/current"},
		{http.MethodPost, "/api/v1/accounts/reauth"},
	}
	bodies := map[string]struct{ contentType, body string }{
		"no body":        {"", ""},
		"text body":      {"text/plain", "x"},
		"malformed JSON": {"application/json", "{"},
		"mistyped JSON":  {"application/json", `{"reauth_token": 1, "current_password": 1}`},
	}
	for _, r := range routes {
		for name, b := range bodies {
			suite.Run(r.method+" "+r.path+" "+name, func() {
				suite.SetupTest()
				suite.mockDB.On("GetSession", mock.Anything, testSessionToken).Return(model.Session{AccountID: testAccountID}, true, nil).Maybe()
				req := httptest.NewRequest(r.method, r.path, strings.NewReader(b.body))
				if b.contentType != "" {
					req.Header.Set("Content-Type", b.contentType)
				}
				req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: testSessionToken})
				resp, err := suite.createTestServer().App.Test(req, -1)
				suite.Require().NoError(err)
				suite.Equal(http.StatusBadRequest, resp.StatusCode)
			})
		}
	}
}
