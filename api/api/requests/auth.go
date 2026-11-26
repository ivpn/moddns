package requests

type LoginBody struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"` //nolint:gosec // G117 - intentional sensitive field
	// OTP is the 2FA code, required when the account has 2FA on.
	OTP string `json:"otp,omitempty" validate:"omitempty,max=64"`
}
