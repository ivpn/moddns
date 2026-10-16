package middleware

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/spf13/cast"
)

// SubscriptionProvider can fetch a subscription for a given account.
type SubscriptionProvider interface {
	GetSubscription(ctx context.Context, accountID string) (*model.Subscription, error)
}

// NewSubscriptionGuard returns middleware that enforces API access rules based
// on subscription lifecycle status. Routes are classified into three tiers:
//
//   - alwaysAllowed: accessible in any subscription state (auth, logout, delete account, resync, export,
//     turning data collection off and deleting stored statistics and logs)
//   - limitedAllowed: accessible during Active, GracePeriod, AND LimitedAccess (read-only views, account mgmt)
//   - everything else: blocked during LimitedAccess, Inactive, and PendingDelete (mutations to profiles, rules, blocklists)
//
// Uses an allowlist pattern: new endpoints are blocked by default until explicitly classified.
// If provider is nil, the middleware is a no-op (allows all requests).
func NewSubscriptionGuard(provider SubscriptionProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if provider == nil {
			return c.Next()
		}

		accountID := auth.GetAccountID(c)
		if accountID == "" {
			return c.Next() // unauthenticated route — auth middleware handles this
		}

		sub, err := provider.GetSubscription(c.UserContext(), accountID)
		if err != nil {
			return c.Next() // no subscription found — allow (pre-ZLA or new accounts)
		}

		status := sub.GetStatus()
		if status == model.StatusActive || status == model.StatusGracePeriod {
			return c.Next()
		}

		method := c.Method()
		path := c.Path()
		c.Route()

		if isAlwaysAllowed(method, path) || isTurnOffOnly(method, path, c.Body()) {
			return c.Next()
		}

		if status == model.StatusLimitedAccess && isLimitedAccessAllowed(method, path) {
			return c.Next()
		}

		msg := "Your account is in limited access mode."
		if status == model.StatusInactive {
			msg = "Your account is inactive."
		}
		if status == model.StatusPendingDelete {
			msg = "Your account is scheduled for deletion."
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": msg, "status": string(status)})
	}
}

// isAlwaysAllowed returns true for routes accessible in any subscription state.
// Uses the raw request path (c.Path()), not the parameterized Fiber route.
func isAlwaysAllowed(method, path string) bool {
	always := []routeRule{
		// Auth (login, register, webauthn auth ceremonies)
		{method: "POST", prefix: "/api/v1/login"},
		{method: "POST", prefix: "/api/v1/accounts", exact: true},
		{method: "POST", prefix: "/api/v1/webauthn/register/"},
		{method: "POST", prefix: "/api/v1/webauthn/login/"},
		// PASession flow
		{method: "PUT", prefix: "/api/v1/pasession/rotate"},
		// Subscription view + resync
		{method: "GET", prefix: "/api/v1/sub", exact: true},
		{method: "PUT", prefix: "/api/v1/sub/update"},
		// Logout
		{method: "POST", prefix: "/api/v1/accounts/logout"},
		// View own account
		{method: "GET", prefix: "/api/v1/accounts/current", exact: true},
		// Account deletion
		{method: "POST", prefix: "/api/v1/accounts/current/deletion-code"},
		{method: "DELETE", prefix: "/api/v1/accounts/current", exact: true},
		// Export profiles (Article 20 portability — allowed in all subscription states)
		{method: "POST", prefix: "/api/v1/profiles/export", exact: true},
		// View passkeys (read-only, shown grayed out on account-preferences during PD)
		{method: "GET", prefix: "/api/v1/webauthn/passkeys"},
		// Password reset
		{method: "POST", prefix: "/api/v1/verify/reset-password"},
		{method: "POST", prefix: "/api/v1/accounts/reset-password"},
		// Deleting stored statistics and query logs, and the profile list to see what is
		// collected. Turning collection off goes through isTurnOffOnly.
		{method: "DELETE", prefix: "/api/v1/profiles/", suffix: "/statistics"},
		{method: "DELETE", prefix: "/api/v1/profiles/", suffix: "/logs"},
		{method: "GET", prefix: "/api/v1/profiles", exact: true},
	}

	return matchRoute(method, path, always)
}

// turnOffPaths are the profile settings a turn-off-only PATCH may set, each to false.
var turnOffPaths = map[string]bool{
	"/settings/statistics/enabled": true,
	"/settings/logs/enabled":       true,
}

// isTurnOffOnly reports whether the request is PATCH /api/v1/profiles/{id} whose every
// update replaces statistics.enabled or logs.enabled with false, which is allowed in
// any subscription state. Any other body fails closed.
func isTurnOffOnly(method, path string, body []byte) bool {
	id, ok := strings.CutPrefix(path, "/api/v1/profiles/")
	if method != fiber.MethodPatch || !ok || id == "" || strings.Contains(id, "/") {
		return false
	}
	var req struct {
		Updates []model.ProfileUpdate `json:"updates"`
	}
	if json.Unmarshal(body, &req) != nil || len(req.Updates) == 0 {
		return false
	}
	for _, u := range req.Updates {
		if u.Operation != model.UpdateOperationReplace || !turnOffPaths[u.Path] {
			return false
		}
		// The service reads the value with cast.ToBoolE, which also turns nil into false.
		v := u.Unwrapped()
		if v == nil {
			return false
		}
		if on, err := cast.ToBoolE(v); err != nil || on {
			return false
		}
	}
	return true
}

// isLimitedAccessAllowed returns true for routes allowed during LimitedAccess
// (in addition to alwaysAllowed). These are blocked during PendingDelete.
func isLimitedAccessAllowed(method, path string) bool {
	limited := []routeRule{
		// Read-only profile views
		{method: "GET", prefix: "/api/v1/profiles"},
		// Read-only catalogs
		{method: "GET", prefix: "/api/v1/blocklists"},
		{method: "GET", prefix: "/api/v1/services"},
		// Passkey management (GET passkeys moved to alwaysAllowed)
		{method: "POST", prefix: "/api/v1/webauthn/passkey/add/"},
		{method: "DELETE", prefix: "/api/v1/webauthn/passkey/"},
		// Reauth
		{method: "POST", prefix: "/api/v1/webauthn/passkey/reauth/"},
		// Account management
		{method: "PATCH", prefix: "/api/v1/accounts", exact: true},
		// 2FA
		{method: "POST", prefix: "/api/v1/accounts/mfa/totp/"},
		// Sessions
		{method: "DELETE", prefix: "/api/v1/sessions"},
		// Email verification
		{method: "POST", prefix: "/api/v1/verify/email/otp/"},
	}

	return matchRoute(method, path, limited)
}

type routeRule struct {
	method string
	prefix string
	suffix string // optional: path must also end with this
	exact  bool   // if true, path must equal prefix exactly
}

// matchRoute checks if the request matches any route rule against the raw
// request URL path (c.Path()).
func matchRoute(method, path string, rules []routeRule) bool {
	for _, r := range rules {
		if r.method != method {
			continue
		}
		if r.exact {
			if path == r.prefix {
				return true
			}
			continue
		}
		if !strings.HasPrefix(path, r.prefix) {
			continue
		}
		if r.suffix != "" && !strings.HasSuffix(path, r.suffix) {
			continue
		}
		return true
	}
	return false
}
