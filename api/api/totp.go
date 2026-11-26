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
// @Failure 401 {object} ErrResponse
// @Failure 429 {object} ErrResponse
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
			return handleSessionTotpError(c, err, ErrFailedToConfirm2FA.Error())
		}
		return c.Status(200).JSON(backup)
	}
	return handler
}

// @Summary Disable TOTP
// @Description Turn 2FA off. The account owner is verified first with the current password or a passkey reauth token issued for the totp_disable purpose, then the change is authorised by an authenticator code or a backup code.
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.TotpDisableReq true "Disable TOTP request"
// @Success 200 {object} model.Account
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
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
		acc, err := s.Service.TotpDisable(c.UserContext(), accountId, *p)
		if err != nil {
			return handleSessionTotpError(c, err, ErrFailedToDisable2FA.Error())
		}
		return c.Status(200).JSON(acc)
	}
	return handler
}

// @Summary Regenerate TOTP backup codes
// @Description Replace every backup code with a new set. The account owner is verified first with the current password or a passkey reauth token issued for the backup_codes_regenerate purpose, then the change is authorised by an authenticator code or a backup code. The new codes are returned only in this response.
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.TotpRegenerateReq true "Regenerate backup codes request"
// @Success 200 {object} model.TOTPBackup
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/mfa/totp/backup-codes/regenerate [post]
func (s *APIServer) regenerateBackupCodes() fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := new(requests.TotpRegenerateReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToRegenerateBackupCodes.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		backup, err := s.Service.TotpRegenerateBackupCodes(c.UserContext(), auth.GetAccountID(c), *p)
		if err != nil {
			return handleSessionTotpError(c, err, ErrFailedToRegenerateBackupCodes.Error())
		}
		return c.Status(200).JSON(backup)
	}
}

// handleSessionTotpError answers a refused 2FA code on a signed-in request. The
// session is valid, so a wrong code is a bad request body: 401 would read as an
// expired session (same rule as the reauth errors in errors.go).
func handleSessionTotpError(c *fiber.Ctx, err error, msg string) error {
	var totpErr *account.TOTPError
	if errors.As(err, &totpErr) && !errors.Is(err, account.ErrTOTPTooManyAttempts) {
		log.Ctx(c.UserContext()).Warn().Err(err).Msg(msg)
		return c.Status(fiber.StatusBadRequest).JSON(ErrResponse{Error: err.Error()})
	}
	return HandleError(c, err, msg)
}
