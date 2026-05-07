package middleware

import (
	"testing"
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
		{"GET", "/api/v1/accounts/current/export", true},
		{"POST", "/api/v1/verify/reset-password", true},
		{"POST", "/api/v1/accounts/reset-password", true},
		{"GET", "/api/v1/webauthn/passkeys", true},

		// POST /accounts is exact — sub-paths are NOT always allowed
		{"POST", "/api/v1/accounts/mfa/totp/enable", false},

		// GET /accounts/current is exact — sub-paths are NOT always allowed
		{"GET", "/api/v1/accounts/current/export", true}, // export IS explicitly listed

		// NOT always allowed — require LimitedAccess
		{"GET", "/api/v1/profiles", false},
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
		{"GET", "/api/v1/profiles/abc123/logs/download", true},
		{"DELETE", "/api/v1/profiles/abc123/logs", true},
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
		{"DELETE", "/api/v1/profiles/abc123", false}, // delete profile (not suffix /logs)
		{"PATCH", "/api/v1/profiles/abc123", false},  // update profile settings
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
