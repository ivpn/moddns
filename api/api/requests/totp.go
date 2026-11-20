package requests

type TotpReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=8"`
}

// TotpDisableReq takes an authenticator code or a backup code, which may be
// entered grouped with hyphens or spaces.
type TotpDisableReq struct {
	OTP string `json:"otp" validate:"required,min=6,max=64"`
}
