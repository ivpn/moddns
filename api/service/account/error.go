package account

import (
	"errors"

	"github.com/ivpn/dns/api/internal/reauth"
)

// ServiceAccountError is a custom error type for account service errors
type ServiceAccountError struct {
	Code    string
	Message error
}

// Error implements Error interface
func (e *ServiceAccountError) Error() string {
	return e.Message.Error()
}

// Create constructor functions for each error
func NewServiceAccountError(msg string) *ServiceAccountError {
	return &ServiceAccountError{
		Code:    "SERVICE_ACCOUNT_ERROR",
		Message: errors.New(msg),
	}
}

var (
	ErrFailedToCreateAccount = NewServiceAccountError("failed to create account")
	ErrAccountAlreadyExists  = NewServiceAccountError("account with this email already exists")
	// Generic user-facing failure for registration scenarios (cache missing, duplicate subscription, finished account reuse)
	ErrUnableToCreateAccount     = NewServiceAccountError("Unable to create account.")
	ErrAccountIdMissing          = NewServiceAccountError("account_id missing")
	ErrInvalidVerificationToken  = NewServiceAccountError("invalid verification token")
	ErrEmailNotVerified          = NewServiceAccountError("email not verified")
	ErrEmailAlreadyVerified      = NewServiceAccountError("email already verified")
	ErrEmailOTPRateLimited       = NewServiceAccountError("email verification otp rate limited")
	ErrEmailOTPManyAttempts      = NewServiceAccountError("too many invalid email verification attempts")
	ErrEmailOTPMissing           = NewServiceAccountError("email verification otp missing")
	ErrEmailSuppressed           = NewServiceAccountError("Email delivery is not possible until your address is verified.")
	ErrFailedToResetPassword     = NewServiceAccountError("failed to reset password")
	ErrPasswordsDoNotMatch       = NewServiceAccountError("passwords do not match")
	ErrPasswordTooSimple         = NewServiceAccountError("password does not meet complexity requirements")
	ErrTokenExpired              = NewServiceAccountError("token expired")
	ErrInvalidDeletionCode       = NewServiceAccountError("invalid deletion code")
	ErrDeletionCodeExpired       = NewServiceAccountError("deletion code expired")
	ErrInvalidUpdateOperation    = NewServiceAccountError("invalid update operation")
	ErrInvalidEmailUpdatePayload = NewServiceAccountError("invalid email update payload")
	ErrMissingEmailUpdateFields  = NewServiceAccountError("missing current_password or new_email")
	// ErrInvalidCurrentPassword aliases reauth.ErrInvalidPassword so the
	// unified reauth helper, the legacy account password-test path, and any
	// caller doing errors.Is(...) all see the same sentinel. As with the
	// reauth-token errors, it is a plain errors.New(...) — not a
	// *ServiceAccountError — and is mapped to HTTP 400 in api/api/errors.go.
	ErrInvalidCurrentPassword = reauth.ErrInvalidPassword
	ErrInvalidNewEmail        = NewServiceAccountError("invalid new email")
	ErrSameEmailAddress       = NewServiceAccountError("new email address is the same as the current one")

	// Reauth sentinels are re-exported from internal/reauth so the unified
	// helper and every legacy call site refer to the same error values. These
	// are plain errors.New(...) — NOT *ServiceAccountError — which means
	// HandleError's *ServiceAccountError 400 branch does not match them; they
	// are mapped explicitly to HTTP 400 in api/api/errors.go (matching the
	// pre-unification mapping for these error values).
	ErrMultipleAuthMethods = reauth.ErrMultipleAuthMethods
	ErrMissingAuthMethod   = reauth.ErrMissingAuthMethod
	ErrInvalidReauthToken  = reauth.ErrInvalidReauthToken
	ErrReauthTokenExpired  = reauth.ErrReauthTokenExpired

	ErrReauthRateLimited    = NewServiceAccountError("reauth rate limited")
	ErrSignupWebhook        = NewServiceAccountError("Unable to call signup webhook.")
	ErrPasswordTestRequired = NewServiceAccountError("password test operation required before replace")
)

// TOTPError is a custom error type for 2FA errors
type TOTPError struct {
	Code    string
	Message error
}

// Error implements Error interface
func (e *TOTPError) Error() string {
	return e.Message.Error()
}

// Create constructor functions for each error
func NewTOTPError(msg string) *TOTPError {
	return &TOTPError{
		Code:    "TOTP_ERROR",
		Message: errors.New(msg),
	}
}

var (
	ErrCreateOTP             = NewTOTPError("could not create OTP")
	ErrSaveOTP               = NewTOTPError("could not save OTP")
	ErrSendOTP               = NewTOTPError("could not send OTP")
	ErrExpiredOTP            = NewTOTPError("expired OTP")
	ErrIncorrectOTP          = NewTOTPError("incorrect OTP")
	ErrTotpDisabled          = NewTOTPError("2FA is disabled")
	ErrGetTotp               = NewTOTPError("could not get 2FA code")
	ErrTotpBackupAlreadyUsed = NewTOTPError("2FA backup is already used")
	ErrTotpBackupNotFound    = NewTOTPError("2FA backup not found")
	ErrTotpSetBackup         = NewTOTPError("could not set 2FA backup")
	ErrTotpDisable           = NewTOTPError("could not disable 2FA")
	ErrInvalidTOTPCode       = NewTOTPError("invalid 2FA code")
	ErrTOTPAlreadyConfigured = NewTOTPError("2FA already configured")
	ErrTOTPNotConfigured     = NewTOTPError("2FA is not configured")
	ErrTOTPAlreadyDisabled   = NewTOTPError("2FA already disabled")
	ErrTOTPRequired          = NewTOTPError("TOTP is required")
)
