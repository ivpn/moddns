package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
)

// @Summary Enable TOTP
// @Description Enable TOTP
// @Tags Account
// @Accept json
// @Produce json
// @Success 200 {object} model.TOTPNew
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/mfa/totp/enable [post]
func (s *APIServer) TotpEnable() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		totp, err := s.Service.TotpEnable(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, ErrFailedToEnable2FA.Error())
		}

		return c.Status(200).JSON(totp)
	}
	return handler
}

// @Summary Confirm TOTP
// @Description Confirm TOTP
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.TotpReq true "Confirm TOTP request"
// @Success 200 {object} model.TOTPBackup
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/mfa/totp/enable/confirm [post]
func (s *APIServer) confirm2FA() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.TotpReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToRegisterAccount.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		accountId := auth.GetAccountID(c)
		backup, err := s.Service.TotpConfirm(c.Context(), accountId, p.OTP)
		if err != nil {
			return HandleError(c, err, ErrFailedToConfirm2FA.Error())
		}
		return c.Status(200).JSON(backup)
	}
	return handler
}

// @Summary Disable TOTP
// @Description Disable TOTP
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.TotpReq true "Disable TOTP request"
// @Success 200 {object} model.Account
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/mfa/totp/disable [post]
func (s *APIServer) disable2FA() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.TotpReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToRegisterAccount.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		accountId := auth.GetAccountID(c)
		acc, err := s.Service.TotpDisable(c.Context(), accountId, p.OTP)
		if err != nil {
			return HandleError(c, err, ErrFailedToDisable2FA.Error())
		}
		return c.Status(200).JSON(acc)
	}
	return handler
}
