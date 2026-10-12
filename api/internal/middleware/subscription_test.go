package middleware

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAlwaysAllowed(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		// Always allowed
		{"POST", "/api/v1/login", true},
		{"POST", "/api/v1/accounts", true},
		{"POST", "/api/v1/webauthn/register/begin", true},
		{"POST", "/api/v1/webauthn/register/finish", true},
		{"POST", "/api/v1/webauthn/login/begin", true},
		{"POST", "/api/v1/webauthn/login/finish", true},
		{"PUT", "/api/v1/pasession/rotate", true},
		{"GET", "/api/v1/sub", true},
		{"PUT", "/api/v1/sub/update", true},
		{"POST", "/api/v1/accounts/logout", true},
		{"GET", "/api/v1/accounts/current", true},
		{"POST", "/api/v1/accounts/current/deletion-code", true},
		{"DELETE", "/api/v1/accounts/current", true},
		// Export profiles (Article 20 portability — specRef: M1, E18, E19)
		{"POST", "/api/v1/profiles/export", true},
		{"POST", "/api/v1/verify/reset-password", true},
		{"POST", "/api/v1/accounts/reset-password", true},
		{"GET", "/api/v1/webauthn/passkeys", true},
		// Consent withdrawal and erasure (specRef: api-endpoint-behaviour.md J3, J52, G27)
		{"DELETE", "/api/v1/profiles/abc123/logs", true},
		{"DELETE", "/api/v1/profiles/abc123/statistics", true},
		{"GET", "/api/v1/profiles", true},

		// POST /accounts is exact — sub-paths are NOT always allowed
		{"POST", "/api/v1/accounts/mfa/totp/enable", false},

		// GET /accounts/current is exact — sub-paths are NOT always allowed
		{"GET", "/api/v1/accounts/current/export", false}, // old placeholder removed (specRef: M3)
		// POST /profiles/export is exact — sub-paths are NOT always allowed
		{"POST", "/api/v1/profiles/export/extra", false},

		// NOT always allowed — require LimitedAccess
		{"GET", "/api/v1/profiles/abc123", false},
		{"GET", "/api/v1/blocklists", false},
		{"PATCH", "/api/v1/accounts", false},
		{"POST", "/api/v1/verify/email/otp/request", false},

		// Mutations — never always allowed
		{"POST", "/api/v1/profiles", false},
		{"PATCH", "/api/v1/profiles/abc123", false},
		{"POST", "/api/v1/profiles/abc123/blocklists", false},
		{"POST", "/api/v1/mobileconfig", false},
	}

	for _, tt := range tests {
		got := isAlwaysAllowed(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("isAlwaysAllowed(%s, %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestIsLimitedAccessAllowed(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		// Allowed during Limited Access
		{"GET", "/api/v1/profiles", true},
		{"GET", "/api/v1/profiles/abc123", true},
		{"GET", "/api/v1/profiles/abc123/logs", true},
		{"GET", "/api/v1/profiles/abc123/logs/devices", true},
		{"GET", "/api/v1/profiles/abc123/logs/top", true},
		{"GET", "/api/v1/profiles/abc123/logs/clients", true},
		{"GET", "/api/v1/profiles/abc123/logs/blocklists", true},
		{"GET", "/api/v1/profiles/abc123/logs/download", true},
		{"GET", "/api/v1/profiles/abc123/statistics", true},
		{"GET", "/api/v1/blocklists", true},
		{"GET", "/api/v1/services", true},
		// GET /webauthn/passkeys moved to alwaysAllowed — not tested here
		{"POST", "/api/v1/webauthn/passkey/add/begin", true},
		{"POST", "/api/v1/webauthn/passkey/add/finish", true},
		{"DELETE", "/api/v1/webauthn/passkey/abc123", true},
		{"POST", "/api/v1/webauthn/passkey/reauth/begin", true},
		{"POST", "/api/v1/webauthn/passkey/reauth/finish", true},
		{"PATCH", "/api/v1/accounts", true},
		{"POST", "/api/v1/accounts/mfa/totp/enable", true},
		{"POST", "/api/v1/accounts/mfa/totp/enable/confirm", true},
		{"POST", "/api/v1/accounts/mfa/totp/disable", true},
		{"DELETE", "/api/v1/sessions", true},
		{"POST", "/api/v1/verify/email/otp/request", true},
		{"POST", "/api/v1/verify/email/otp/confirm", true},

		// NOT allowed — blocked mutations
		{"POST", "/api/v1/profiles", false},          // create profile (POST prefix matches GET prefix, but method differs)
		{"DELETE", "/api/v1/profiles/abc123", false}, // delete profile
		{"PATCH", "/api/v1/profiles/abc123", false},  // update profile settings (turn-off only bodies pass isTurnOffOnly)
		{"POST", "/api/v1/profiles/abc123/blocklists", false},
		{"DELETE", "/api/v1/profiles/abc123/blocklists", false},
		{"POST", "/api/v1/profiles/abc123/services", false},
		{"DELETE", "/api/v1/profiles/abc123/services", false},
		{"POST", "/api/v1/profiles/abc123/custom_rules", false},
		{"POST", "/api/v1/profiles/abc123/custom_rules/batch", false},
		{"DELETE", "/api/v1/profiles/prof1/custom_rules/rule1", false},
		{"POST", "/api/v1/mobileconfig", false},
		{"POST", "/api/v1/mobileconfig/short", false},
	}

	for _, tt := range tests {
		got := isLimitedAccessAllowed(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("isLimitedAccessAllowed(%s, %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

type fixedSubscription struct{ sub *model.Subscription }

func (f fixedSubscription) GetSubscription(context.Context, string) (*model.Subscription, error) {
	return f.sub, nil
}

func subscriptionIn(status model.SubscriptionStatus) *model.Subscription {
	now := time.Now()
	switch status {
	case model.StatusLimitedAccess:
		return &model.Subscription{ActiveUntil: now.AddDate(0, 0, -5), UpdatedAt: now.Add(-time.Hour)}
	case model.StatusInactive:
		return &model.Subscription{ActiveUntil: now.AddDate(0, 1, 0), UpdatedAt: now, Tier: model.TierStandard}
	case model.StatusPendingDelete:
		return &model.Subscription{ActiveUntil: now.AddDate(0, 1, 0), UpdatedAt: now, DeletionScheduledAt: &now}
	}
	return &model.Subscription{ActiveUntil: now.AddDate(0, 1, 0), UpdatedAt: now}
}

// guardApp echoes the request body from every handler, so a test sees whether the guard let it through intact.
func guardApp(sub *model.Subscription) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(auth.ACCOUNT_ID, "acc")
		return c.Next()
	})
	app.Use(NewSubscriptionGuard(fixedSubscription{sub}))
	app.All("/*", func(c *fiber.Ctx) error { return c.Send(c.Body()) })
	return app
}

const (
	statsOff = `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":false}]}`
	bothOff  = `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":false},{"operation":"replace","path":"/settings/logs/enabled","value":false}]}`
)

// specRef: api-endpoint-behaviour.md G27, J3, J52 — withdrawing consent and deleting stored data work in every subscription state.
func TestSubscriptionGuardAllowsConsentWithdrawalInEveryState(t *testing.T) {
	for _, status := range []model.SubscriptionStatus{model.StatusLimitedAccess, model.StatusInactive, model.StatusPendingDelete} {
		t.Run(string(status), func(t *testing.T) {
			sub := subscriptionIn(status)
			require.Equal(t, status, sub.GetStatus())
			app := guardApp(sub)
			cases := []struct {
				method, path, body string
				want               int
			}{
				{"PATCH", "/api/v1/profiles/abc123", statsOff, fiber.StatusOK},
				{"PATCH", "/api/v1/profiles/abc123", bothOff, fiber.StatusOK},
				{"DELETE", "/api/v1/profiles/abc123/statistics", "", fiber.StatusOK},
				{"DELETE", "/api/v1/profiles/abc123/logs", "", fiber.StatusOK},
				{"GET", "/api/v1/profiles", "", fiber.StatusOK},
				// Turning on and every other change stay blocked.
				{"PATCH", "/api/v1/profiles/abc123", `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":true}]}`, fiber.StatusForbidden},
				{"PATCH", "/api/v1/profiles/abc123", `{"updates":[{"operation":"replace","path":"/settings/statistics/retention","value":"30d"}]}`, fiber.StatusForbidden},
				{"DELETE", "/api/v1/profiles/abc123", "", fiber.StatusForbidden},
			}
			for _, tc := range cases {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				resp, err := app.Test(req)
				require.NoError(t, err)
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, tc.want, resp.StatusCode, "%s %s %s", tc.method, tc.path, tc.body)
				if tc.want == fiber.StatusOK {
					assert.Equal(t, tc.body, string(body), "the handler reads the body the guard inspected")
				} else {
					assert.Contains(t, string(body), `"status":"`+string(status)+`"`)
				}
			}
		})
	}
}

// specRef: subscription-lifecycle-enforcement.md — a cut-off account still cannot read logs or statistics.
func TestSubscriptionGuardKeepsCutOffReadsBlocked(t *testing.T) {
	app := guardApp(subscriptionIn(model.StatusInactive))
	for _, path := range []string{"/api/v1/profiles/abc123", "/api/v1/profiles/abc123/logs", "/api/v1/profiles/abc123/statistics"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode, path)
	}
}

// specRef: api-endpoint-behaviour.md G27
func TestIsTurnOffOnly(t *testing.T) {
	const p = "/api/v1/profiles/abc123"
	tests := []struct {
		name, method, path, body string
		want                     bool
	}{
		{"statistics off", "PATCH", p, statsOff, true},
		{"logs off", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/logs/enabled","value":false}]}`, true},
		{"both off", "PATCH", p, bothOff, true},
		{"wrapped value", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":{"value":false}}]}`, true},
		{"turn on", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/logs/enabled","value":true}]}`, false},
		{"off mixed with on", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":false},{"operation":"replace","path":"/settings/logs/enabled","value":true}]}`, false},
		{"off mixed with another path", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":false},{"operation":"replace","path":"/name","value":"x"}]}`, false},
		{"sub-option", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/logs/log_domains","value":false}]}`, false},
		{"non-replace operation", "PATCH", p, `{"updates":[{"operation":"remove","path":"/settings/statistics/enabled","value":false}]}`, false},
		{"unparsable value", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled","value":"maybe"}]}`, false},
		{"missing value", "PATCH", p, `{"updates":[{"operation":"replace","path":"/settings/statistics/enabled"}]}`, false},
		{"empty updates", "PATCH", p, `{"updates":[]}`, false},
		{"malformed body", "PATCH", p, `{"updates":`, false},
		{"empty body", "PATCH", p, ``, false},
		{"sub-path", "PATCH", p + "/custom_rules/r1", statsOff, false},
		{"no profile id", "PATCH", "/api/v1/profiles/", statsOff, false},
		{"profile list", "PATCH", "/api/v1/profiles", statsOff, false},
		{"other method", "POST", p, statsOff, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTurnOffOnly(tt.method, tt.path, []byte(tt.body)))
		})
	}
}
