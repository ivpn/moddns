package requests

// TotpReq takes an authenticator code, which may be entered with spaces or
// hyphens between the digits.
type TotpReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=16"`
}

// TotpDisableReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces.
type TotpDisableReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=64"`
}
