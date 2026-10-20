package requests

type WebAuthnReauthBeginRequest struct {
	Purpose string `json:"purpose" validate:"required,oneof=email_change account_deletion profile_export profile_import password_set passkey_add passkey_delete"`
}

// WebAuthnDeletePasskeyRequest verifies the account owner before a passkey is deleted.
// Exactly one of CurrentPassword or ReauthToken must be provided.
type WebAuthnDeletePasskeyRequest struct {
	CurrentPassword *string `json:"current_password,omitempty" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token,omitempty" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}

// WebAuthnAddPasskeyBeginRequest verifies the account owner before a passkey is added.
// Exactly one of CurrentPassword or ReauthToken must be provided.
type WebAuthnAddPasskeyBeginRequest struct {
	CurrentPassword *string `json:"current_password,omitempty" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token,omitempty" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}
