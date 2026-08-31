package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/valyala/fasthttp"
)

// specRef: registration-subscription-behaviour.md S4, S4a
func TestNewPSK(t *testing.T) {
	tests := []struct {
		name           string
		configuredPSK  string
		authorization  string
		expectedStatus int
	}{
		{
			name:           "Matching PSK passes",
			configuredPSK:  "secret",
			authorization:  "Bearer secret",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "Wrong PSK rejected",
			configuredPSK:  "secret",
			authorization:  "Bearer wrong",
			expectedStatus: fiber.StatusUnauthorized,
		},
		{
			name:           "Missing token rejected",
			configuredPSK:  "secret",
			authorization:  "",
			expectedStatus: fiber.StatusUnauthorized,
		},
		{
			name:           "Unset PSK rejects all requests",
			configuredPSK:  "",
			authorization:  "",
			expectedStatus: fiber.StatusUnauthorized,
		},
		{
			name:           "Unset PSK rejects even a bearer token",
			configuredPSK:  "",
			authorization:  "Bearer anything",
			expectedStatus: fiber.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(NewPSK(config.APIConfig{PSK: tt.configuredPSK}))
			app.Get("/", func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestGetToken(t *testing.T) {
	tests := []struct {
		name           string
		authorization  string
		cookie         string
		expectedResult string
	}{
		{
			name:           "Valid Bearer token in Authorization header",
			authorization:  "Bearer validtoken",
			cookie:         "",
			expectedResult: "validtoken",
		},
		{
			name:           "Valid token in cookie",
			authorization:  "",
			cookie:         "validtoken",
			expectedResult: "validtoken",
		},
		{
			name:           "No token in Authorization header or cookie",
			authorization:  "",
			cookie:         "",
			expectedResult: "",
		},
		{
			name:           "Invalid Authorization header format",
			authorization:  "Invalid validtoken",
			cookie:         "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			req := &fasthttp.RequestCtx{}
			c := app.AcquireCtx(req)
			c.Request().Header.Set("Authorization", tt.authorization)
			c.Request().Header.SetCookie(auth.AUTH_COOKIE, tt.cookie)

			result := GetToken(c)
			if result != tt.expectedResult {
				t.Errorf("expected %s, got %s", tt.expectedResult, result)
			}
		})
	}
}
