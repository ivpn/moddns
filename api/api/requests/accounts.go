package requests

import (
	"github.com/ivpn/dns/api/model"
)

type AccountUpdates struct {
	Updates []model.AccountUpdate `json:"updates" validate:"required,dive"`
}

// ReauthRequest asks for a reauth token for one privileged operation, issued for
// the current password plus, when 2FA is on, its code. For totp_disable and
// backup_codes_regenerate the operation asks for the code itself, so it is not
// checked here.
type ReauthRequest struct {
	Purpose         string `json:"purpose" validate:"required,oneof=email_change account_deletion profile_export profile_import password_set passkey_add passkey_delete backup_codes_regenerate totp_disable totp_enable sessions_delete"`
	CurrentPassword string `json:"current_password" validate:"required"` //nolint:gosec // G117 - intentional sensitive field
	OTP             string `json:"otp,omitempty" validate:"omitempty,max=64"`
}

// AccountEmailUpdate represents a structured payload for email change inside a JSON-Patch style update.
type AccountEmailUpdate struct {
	ReauthToken *string `json:"reauth_token"`
	NewEmail    string  `json:"new_email" validate:"required,email"`
}

// AccountPasswordSetVerification is the value of the password test operation: a reauth
// token issued for the password_set purpose.
type AccountPasswordSetVerification struct {
	ReauthToken *string `json:"reauth_token"`
}

// AccountDeletionRequest represents the request to delete an account
type AccountDeletionRequest struct {
	DeletionCode string  `json:"deletion_code" validate:"required"`
	ReauthToken  *string `json:"reauth_token"`
}

// SessionsDeleteRequest verifies the account owner before every other session
// is signed out.
type SessionsDeleteRequest struct {
	ReauthToken *string `json:"reauth_token"`
}
