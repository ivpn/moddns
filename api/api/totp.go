package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/account"
	"github.com/rs/zerolog/log"
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
		totp, err := s.Service.TotpEnable(c.UserContext(), accountId)
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

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToConfirm2FA.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		accountId := auth.GetAccountID(c)
		backup, err := s.Service.TotpConfirm(c.UserContext(), accountId, p.OTP)
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
// @Param body body requests.TotpDisableReq true "Disable TOTP request"
// @Success 200 {object} model.Account
// @Failure 400 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/mfa/totp/disable [post]
func (s *APIServer) disable2FA() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.TotpDisableReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToDisable2FA.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		accountId := auth.GetAccountID(c)
		acc, err := s.Service.TotpDisable(c.UserContext(), accountId, p.OTP)
		var totpErr *account.TOTPError
		if errors.As(err, &totpErr) && !errors.Is(err, account.ErrTOTPTooManyAttempts) {
			// The session is valid; a wrong code is a bad request body. 401 would
			// read as an expired session (same rule as the reauth errors in errors.go).
			log.Ctx(c.UserContext()).Warn().Err(err).Msg(ErrFailedToDisable2FA.Error())
			return c.Status(fiber.StatusBadRequest).JSON(ErrResponse{Error: err.Error()})
		}
		if err != nil {
			return HandleError(c, err, ErrFailedToDisable2FA.Error())
		}
		return c.Status(200).JSON(acc)
	}
	return handler
}
