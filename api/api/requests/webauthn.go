package requests

type WebAuthnReauthBeginRequest struct {
	Purpose string `json:"purpose" validate:"required,oneof=email_change account_deletion profile_export profile_import"`
}
