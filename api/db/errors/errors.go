package errors

import "errors"

var (
	ErrAccountNotFound           = errors.New("account not found")
	ErrProfileNotFound           = errors.New("profile not found")
	ErrSubscriptionNotFound      = errors.New("subscription not found")
	ErrProfileSettingsNotFound   = errors.New("profile settings not found")
	ErrCustomRuleNotFound        = errors.New("custom rule not found")
	ErrSubscriptionAlreadyExists = errors.New("subscription already exists")
	// ErrTOTPStateChanged: the 2FA state no longer matches the precondition of the write.
	ErrTOTPStateChanged = errors.New("totp state changed")
)
