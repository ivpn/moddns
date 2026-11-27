package requests

// TotpReq takes an authenticator code, which may be entered with spaces or
// hyphens between the digits.
type TotpReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=16"`
}

// TotpEnableReq verifies the account owner before 2FA set-up starts with a
// reauth token of purpose totp_enable.
type TotpEnableReq struct {
	ReauthToken *string `json:"reauth_token"`
}

// TotpDisableReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces, and a reauth token of purpose
// totp_disable.
type TotpDisableReq struct {
	OTP         string  `json:"otp" validate:"required,min=6,max=64"`
	ReauthToken *string `json:"reauth_token"`
}

// TotpRegenerateReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces, and a reauth token of purpose
// backup_codes_regenerate.
type TotpRegenerateReq struct {
	OTP         string  `json:"otp" validate:"required,min=6,max=64"`
	ReauthToken *string `json:"reauth_token"`
}
