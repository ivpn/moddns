package passkey

import "errors"

// PasskeyError is a custom error type for passkey errors
type PasskeyError struct {
	Code    string
	Message error
}

// Error implements Error interface
func (e *PasskeyError) Error() string {
	return e.Message.Error()
}

// Create constructor functions for each error
func NewPasskeyError(msg string) *PasskeyError {
	return &PasskeyError{
		Code:    "PASSKEY_ERROR",
		Message: errors.New(msg),
	}
}

// Success message constants (informational only)
// #nosec G101 -- not credentials, static user-facing status messages
var (
	BeginRegistrationSuccess  = "Registration process started successfully."
	FinishRegistrationSuccess = "Registration completed successfully."
	BeginLoginSuccess         = "Login process started successfully."
	FinishLoginSuccess        = "Login completed successfully."
	FinishAddPasskeySuccess   = "Add passkey completed successfully."
	GetPasskeysSuccess        = "Passkeys retrieved successfully."
)

var (
	ErrBeginRegistration  = NewPasskeyError("Unable to start registration. Please try again.")
	ErrFinishRegistration = NewPasskeyError("Unable to complete registration. Please try again.")
	ErrBeginLogin         = NewPasskeyError("Unable to start login. Please try again.")
	ErrFinishLogin        = NewPasskeyError("Unable to complete login. Please try again.")
	ErrBeginAddPasskey    = NewPasskeyError("Unable to start add passkey. Please try again.")
	ErrFinishAddPasskey   = NewPasskeyError("Unable to complete add passkey. Please try again.")
	ErrGetPasskeys        = NewPasskeyError("Unable to retrieve passkeys. Please try again.")
	// ErrGetSession             = "Unable to retrieve session. Please try again."
	// ErrSaveSession            = "Unable to save session. Please try again."
	// ErrDeleteSession          = "Unable to delete session. Please try again."
	ErrDeleteCredential = NewPasskeyError("Unable to delete credential. Please try again.")
	ErrBeginReauth      = NewPasskeyError("Unable to start reauthentication. Please try again.")
	ErrFinishReauth     = NewPasskeyError("Unable to complete reauthentication. Please try again.")
)
