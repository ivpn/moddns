package requests

// TotpReq takes an authenticator code, which may be entered with spaces or
// hyphens between the digits.
type TotpReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=16"`
}

// TotpDisableReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces, and exactly one of the current
// password or a passkey reauth token of purpose totp_disable.
type TotpDisableReq struct {
	OTP             string  `json:"otp" validate:"required,min=6,max=64"`
	CurrentPassword *string `json:"current_password" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}

// TotpRegenerateReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces, and exactly one of the current
// password or a passkey reauth token of purpose backup_codes_regenerate.
type TotpRegenerateReq struct {
	OTP             string  `json:"otp" validate:"required,min=6,max=64"`
	CurrentPassword *string `json:"current_password" validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string `json:"reauth_token" validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}
