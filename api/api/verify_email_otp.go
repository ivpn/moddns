package api

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/auth"
)

// @Summary Request email verification OTP
// @Description Generates and sends a 6-digit OTP to verify the authenticated user's email
// @Tags Verification
// @Success 204
// @Failure 401 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/verify/email/otp/request [post]
func (s *APIServer) requestEmailVerificationOTP() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, ErrUnauthorized.Error())
		}
		if err := s.Service.RequestEmailVerificationOTP(c.UserContext(), accountId); err != nil {
			return HandleError(c, err, "failed to request email verification otp")
		}
		return c.SendStatus(http.StatusNoContent)
	}
}

type verifyEmailOTPBody struct {
	OTP string `json:"otp" validate:"required,len=6"`
}

// @Summary Confirm email verification OTP
// @Description Verifies the 6-digit OTP provided by the authenticated user
// @Tags Verification
// @Accept json
// @Produce json
// @Param body body verifyEmailOTPBody true "OTP verification request"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 410 {object} ErrResponse
// @Failure 422 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/verify/email/otp/confirm [post]
func (s *APIServer) verifyEmailOTP() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, ErrUnauthorized.Error())
		}
		body := new(verifyEmailOTPBody)
		if err := c.BodyParser(body); err != nil {
			return HandleError(c, ErrInvalidRequestBody, ErrInvalidRequestBody.Error())
		}
		if errMsgs := s.Validator.ValidateRequest(c, body, "invalid email verification otp"); len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, ErrInvalidRequestBody.Error(), errMsgs...)
		}
		if err := s.Service.VerifyEmailOTP(c.UserContext(), accountId, body.OTP); err != nil {
			return HandleError(c, err, "failed to verify email otp")
		}
		return c.SendStatus(http.StatusNoContent)
	}
}
