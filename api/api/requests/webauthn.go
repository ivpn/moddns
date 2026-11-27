package requests

type WebAuthnReauthBeginRequest struct {
	Purpose string `json:"purpose" validate:"required,oneof=email_change account_deletion profile_export profile_import password_set passkey_add passkey_delete backup_codes_regenerate totp_disable totp_enable sessions_delete"`
}

// WebAuthnDeletePasskeyRequest verifies the account owner before a passkey is deleted.
type WebAuthnDeletePasskeyRequest struct {
	ReauthToken *string `json:"reauth_token"`
}

// WebAuthnAddPasskeyBeginRequest verifies the account owner before a passkey is added.
type WebAuthnAddPasskeyBeginRequest struct {
	ReauthToken *string `json:"reauth_token"`
}
