package errors

import "errors"

var (
	ErrAccountNotFound           = errors.New("account not found")
	ErrProfileNotFound           = errors.New("profile not found")
	ErrProfileIdAlreadyExists    = errors.New("profile ID already exists")
	ErrSubscriptionNotFound      = errors.New("subscription not found")
	ErrProfileSettingsNotFound   = errors.New("profile settings not found")
	ErrCustomRuleNotFound        = errors.New("custom rule not found")
	ErrSubscriptionAlreadyExists = errors.New("subscription already exists")
)
