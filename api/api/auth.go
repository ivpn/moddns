package api

import (
	"errors"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/rs/zerolog/log"
)

// @Summary Login
// @Description Login endpoint
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body requests.LoginBody true "Login request"
// @Param x-mfa-code header string false "MFA OTP code"
// @Param x-mfa-methods header []string false "MFA methods"
// @Param x-sessions-remove header string false "Remove all active sessions before logging in"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/login [post]
func (s *APIServer) login() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.LoginBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsg := "Failed to login"
		errMsgs := s.Validator.ValidateRequest(c, p, errMsg)
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		acc, err := s.Db.GetAccountByEmail(c.Context(), p.Email)
		if err != nil {
			if errors.Is(err, dbErrors.ErrAccountNotFound) {
				// don't give too much details about account missing
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			log.Err(err).Msg("Failed to get account by email")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		if acc.Password == nil || strings.TrimSpace(p.Password) == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		if !auth.CheckPasswordHash(p.Password, *acc.Password) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		mfa := auth.GetMfaData(c)
		if err := s.Service.MfaCheck(c.Context(), acc, mfa); err != nil {
			return HandleError(c, err, account.ErrTOTPRequired.Error())
		}

		// Check if sessions should be removed
		if auth.GetHeaderSessionsRemove(c) {
			if err := s.Service.DeleteSessionsByAccountID(c.Context(), acc.ID.Hex()); err != nil {
				log.Err(err).Msg("Failed to remove existing sessions")
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		} else {
			// Only check session limit if we're not removing existing sessions
			count, err := s.Service.CountSessionsByAccountID(c.Context(), acc.ID.Hex())
			if err != nil {
				log.Err(err).Msg("Failed to count sessions")
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			if count >= s.Config.API.SessionLimit {
				return HandleError(c, ErrSessionsLimitReached, ErrSessionsLimitReached.Error())
			}
		}

		expires := time.Now().Add(s.Config.API.SessionExpirationTime)
		// Save the session
		sessionData := webauthn.SessionData{
			UserID:  acc.WebAuthnID(),
			Expires: expires,
		}

		token, err := model.GenSessionToken()
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": ErrSaveSession,
			})
		}

		err = s.Service.SaveSession(c.Context(), sessionData, token, acc.ID.Hex(), "")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": ErrSaveSession,
			})
		}

		// Set the token in encrypted cookie
		c.Cookie(&fiber.Cookie{
			Name:     auth.AUTH_COOKIE,
			Value:    token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   int(s.Config.API.SessionExpirationTime.Seconds()),
			Expires:  expires,
		})

		return c.SendStatus(fiber.StatusOK)
	}
	return handler
}

// @Summary Logout
// @Description Logout endpoint
// @Tags Authentication
// @Produce json
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/logout [post]
func (s *APIServer) logout() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		sessionToken, ok := c.Locals(auth.SESSION_TOKEN).(string)
		if !ok || sessionToken == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		if err := s.Service.DeleteSession(c.Context(), sessionToken); err != nil {
			return HandleError(c, err, ErrDeleteSession.Error())
		}

		ClearCookies(c, auth.AUTH_COOKIE)

		return c.SendStatus(fiber.StatusOK)
	}
	return handler
}

// ClearCookies clears cookies by setting them to expire in the past.
// Flags must match the original cookie attributes for the browser to clear them.
// Workaround for clearing cookies: https://github.com/gofiber/fiber/issues/1127
func ClearCookies(c *fiber.Ctx, key ...string) {
	for i := range key {
		c.Cookie(&fiber.Cookie{
			Name:     key[i],
			Expires:  time.Now().Add(-time.Hour * 24),
			Value:    "",
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
		})
	}
}
