package requests

import (
	"github.com/ivpn/dns/api/model"
)

type AccountUpdates struct {
	Updates []model.AccountUpdate `json:"updates" validate:"required,dive"`
}

// AccountEmailUpdate represents a structured payload for email change inside a JSON-Patch style update.
// Exactly one of CurrentPassword or ReauthToken must be provided.
type AccountEmailUpdate struct {
	CurrentPassword *string `json:"current_password" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
	NewEmail        string  `json:"new_email" validate:"required,email"`
}

// AccountPasswordSetVerification is the value of the password test operation for an account
// that has no password: a passkey reauth token issued for the password_set purpose.
type AccountPasswordSetVerification struct {
	ReauthToken *string `json:"reauth_token"`
}

// AccountDeletionRequest represents the request to delete an account
type AccountDeletionRequest struct {
	DeletionCode    string  `json:"deletion_code" validate:"required"`
	CurrentPassword *string `json:"current_password" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}
