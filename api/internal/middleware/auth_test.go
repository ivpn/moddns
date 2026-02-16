package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewAuth(t *testing.T) {
	tests := []struct {
		name           string
		cookie         string
		mockSession    model.Session
		mockFound      bool
		mockErr        error
		expectMock     bool
		expectedStatus int
		expectNextCall bool
	}{
		{
			name:           "no cookie returns 401",
			cookie:         "",
			expectMock:     false,
			expectedStatus: fiber.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "valid session returns 200",
			cookie:         "valid-token",
			mockSession:    model.Session{AccountID: "acc-123", Token: "valid-token"},
			mockFound:      true,
			mockErr:        nil,
			expectMock:     true,
			expectedStatus: fiber.StatusOK,
			expectNextCall: true,
		},
		{
			name:           "session not found returns 401",
			cookie:         "unknown-token",
			mockSession:    model.Session{},
			mockFound:      false,
			mockErr:        nil,
			expectMock:     true,
			expectedStatus: fiber.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "DB error returns 503",
			cookie:         "some-token",
			mockSession:    model.Session{},
			mockFound:      false,
			mockErr:        errors.New("mongo: connection refused"),
			expectMock:     true,
			expectedStatus: fiber.StatusServiceUnavailable,
			expectNextCall: false,
		},
		{
			name:           "DB timeout returns 503",
			cookie:         "timeout-token",
			mockSession:    model.Session{},
			mockFound:      false,
			mockErr:        errors.New("context deadline exceeded"),
			expectMock:     true,
			expectedStatus: fiber.StatusServiceUnavailable,
			expectNextCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewServicer(t)
			cfg := &config.APIConfig{}
			app := fiber.New()
			app.Use(NewAuth(svc, cfg, nil))

			handlerCalled := false
			app.Get("/protected", func(c *fiber.Ctx) error {
				handlerCalled = true
				return c.SendString("OK")
			})

			if tt.expectMock {
				svc.EXPECT().GetSession(mock.Anything, tt.cookie).Return(tt.mockSession, tt.mockFound, tt.mockErr)
			}

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: tt.cookie})
			}

			resp, err := app.Test(req, -1)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			assert.Equal(t, tt.expectNextCall, handlerCalled)
		})
	}
}

func TestNewAuth_SetsLocals(t *testing.T) {
	svc := mocks.NewServicer(t)
	cfg := &config.APIConfig{}
	app := fiber.New()
	app.Use(NewAuth(svc, cfg, nil))

	var gotAccountID, gotSessionToken string
	app.Get("/protected", func(c *fiber.Ctx) error {
		gotAccountID, _ = c.Locals(auth.ACCOUNT_ID).(string)
		gotSessionToken, _ = c.Locals(auth.SESSION_TOKEN).(string)
		return c.SendString("OK")
	})

	session := model.Session{AccountID: "acc-456", Token: "tok-789"}
	svc.EXPECT().GetSession(mock.Anything, "tok-789").Return(session, true, nil)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: "tok-789"})

	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "acc-456", gotAccountID)
	assert.Equal(t, "tok-789", gotSessionToken)
}
