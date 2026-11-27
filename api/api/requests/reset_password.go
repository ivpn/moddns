package requests

type ResetPasswordBody struct {
	Email string `json:"email" validate:"required,email"`
}

type ConfirmResetPasswordBody struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,password"`
	OTP         string `json:"otp,omitempty" validate:"omitempty,max=64"`
}
