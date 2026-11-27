package api

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
)

// @Summary Send reset password email
// @Description Send reset password email
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.ResetPasswordBody true "Send reset password email request"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/reset-password [post]
func (s *APIServer) sendResetPasswordEmail() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.ResetPasswordBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToVerifyEmail.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, ErrInvalidRequestBody.Error(), errMsgs...)
		}

		if err := s.Service.SendResetPasswordEmail(c.UserContext(), p.Email); err != nil {
			return HandleError(c, err, account.ErrFailedToResetPassword.Error())

		}
		return c.SendStatus(http.StatusNoContent)
	}
	return handler
}

// @Summary Confirm password reset
// @Description Confirm password reset
// @Tags Verification
// @Accept json
// @Produce json
// @Param body body requests.ConfirmResetPasswordBody true "Confirm password reset request"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/verify/reset-password [post]
func (s *APIServer) verifyPasswordReset() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.ConfirmResetPasswordBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, account.ErrFailedToResetPassword.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, ErrInvalidRequestBody.Error(), errMsgs...)
		}

		mfa := &model.MfaData{OTP: p.OTP}
		if err := s.Service.VerifyPasswordReset(c.UserContext(), p.Token, p.NewPassword, mfa); err != nil {
			return HandleError(c, err, account.ErrFailedToResetPassword.Error())
		}
		return c.SendStatus(http.StatusNoContent)
	}
	return handler
}
