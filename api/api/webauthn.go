package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/passkey"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebAuthn cookie names
const (
	WebAuthnTempCookie = "webauthn_temp"
	SessionDuration    = 15 * time.Minute
)

// Request/Response types for WebAuthn
type WebAuthnRegisterBeginRequest struct {
	Email string `json:"email" validate:"required,email"`
	SubID string `json:"subid" validate:"required,uuid4"`
}

type WebAuthnLoginBeginRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type WebAuthnResponse struct {
	Message string `json:"message"`
}

// @Summary Begin passkey registration
// @Description Start WebAuthn registration process for new passkey
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body WebAuthnRegisterBeginRequest true "Registration request"
// @Success 200 {object} protocol.CredentialCreation
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/register/begin [post]
func (s *APIServer) beginRegistration() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		var req WebAuthnRegisterBeginRequest
		if err := c.BodyParser(&req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errs := s.Validator.Validate(req)
		if len(errs) > 0 {
			tags := make([]string, 0)
			for _, err := range errs {
				tags = append(tags, err.Tag)
			}
			return HandleError(c, ErrValidationFailed, "validation failed", tags...)
		}

		acc, err := s.Service.GetUnfinishedSignupOrPostAccount(c.Context(), req.Email, "", req.SubID)
		if err != nil {
			return HandleError(c, err, ErrFailedToRegisterAccount.Error())
		}

		options, token, err := s.Service.BeginRegistration(c.Context(), acc)
		if err != nil {
			return HandleError(c, err, "Failed to begin registration")
		}

		// Set temporary session cookie
		c.Cookie(&fiber.Cookie{
			Name:     WebAuthnTempCookie,
			Value:    token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   int(SessionDuration.Seconds()),
			Expires:  time.Now().Add(SessionDuration),
		})

		return c.Status(200).JSON(options)

	}
	return handler
}

// @Summary Finish passkey registration
// @Description Complete WebAuthn registration process
// @Tags Authentication
// @Accept json
// @Produce json
// @Success 201 "Registration completed successfully"
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/register/finish [post]
func (s *APIServer) finishRegistration() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		// Get cookie token
		token := c.Cookies(WebAuthnTempCookie)

		// Clear temporary session cookie
		defer ClearCookies(c, WebAuthnTempCookie)

		// Finish registration
		httpReq, err := adaptor.ConvertRequest(c, true)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": passkey.ErrFinishRegistration,
			})
		}

		if err = s.Service.FinishRegistration(c.Context(), token, httpReq); err != nil {
			return HandleError(c, err, "Failed to finish registration")
		}

		return c.SendStatus(201)
	}

	return handler
}

// @Summary Begin passkey login
// @Description Start WebAuthn login process
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body WebAuthnLoginBeginRequest true "Login request"
// @Success 200 {object} protocol.CredentialCreation
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/login/begin [post]
func (s *APIServer) beginLogin() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		var req WebAuthnLoginBeginRequest
		if err := c.BodyParser(&req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errs := s.Validator.Validate(req)
		if len(errs) > 0 {
			tags := make([]string, 0)
			for _, err := range errs {
				tags = append(tags, err.Tag)
			}
			return HandleError(c, ErrValidationFailed, "validation failed", tags...)
		}

		options, token, err := s.Service.BeginLogin(c.Context(), req.Email)
		if err != nil {
			return HandleError(c, passkey.ErrBeginLogin, err.Error())
		}

		// Set temporary session cookie
		c.Cookie(&fiber.Cookie{
			Name:     WebAuthnTempCookie,
			Value:    token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   int(SessionDuration.Seconds()),
			Expires:  time.Now().Add(SessionDuration),
		})

		return c.Status(200).JSON(options)
	}
	return handler
}

// @Summary Finish passkey login
// @Description Complete WebAuthn login process
// @Tags Authentication
// @Accept json
// @Produce json
// @Param x-sessions-remove header string false "Remove all other active sessions during login"
// @Success 201 "Login completed successfully"
// @Failure 400 {object} ErrResponse
// @Failure 429 {object} ErrResponse "Session limit reached"
// @Router /api/v1/webauthn/login/finish [post]
func (s *APIServer) finishLogin() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		tmpToken := c.Cookies(WebAuthnTempCookie)
		if tmpToken == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		// Clear temporary session cookie on client side
		defer ClearCookies(c, WebAuthnTempCookie)

		// Convert request to HTTP request
		httpReq, err := adaptor.ConvertRequest(c, true)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": passkey.ErrFinishLogin,
			})
		}

		account, token, purpose, err := s.Service.FinishLogin(c.Context(), tmpToken, httpReq, true)
		if err != nil {
			return HandleError(c, err, "Failed to finish login")
		}
		if purpose != "" {
			log.Warn().Str("purpose", purpose).Msg("unexpected non-empty purpose returned from FinishLogin with session save")
		}

		// Check if sessions should be removed
		if auth.GetHeaderSessionsRemove(c) {
			// Delete all existing sessions except the newly created one
			if err := s.Service.DeleteSessionsByAccountIDExceptCurrent(c.Context(), account.ID.Hex(), token); err != nil {
				return HandleError(c, err, "Failed to remove existing sessions")
			}
		} else {
			// Only check session limit if we're not removing existing sessions
			count, err := s.Service.CountSessionsByAccountID(c.Context(), account.ID.Hex())
			if err != nil {
				return HandleError(c, err, "Failed to count sessions")
			}
			if count >= s.Config.API.SessionLimit {
				// Delete the just-created session since we're rejecting the login
				if err := s.Service.DeleteSession(c.Context(), token); err != nil {
					log.Err(err).Msg("failed to delete session after limit reached")
				}
				return HandleError(c, ErrSessionsLimitReached, ErrSessionsLimitReached.Error())
			}
		}

		expires := time.Now().Add(s.Config.API.SessionExpirationTime)
		// Set the new token in encrypted cookie
		c.Cookie(&fiber.Cookie{
			Name:     auth.AUTH_COOKIE,
			Value:    token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   int(s.Config.API.SessionExpirationTime.Seconds()),
			Expires:  expires,
		})

		return c.SendStatus(201)
	}
	return handler
}

// @Summary Add new passkey
// @Description Add a new passkey to authenticated account
// @Tags Authentication
// @Accept json
// @Produce json
// @Success 200 {object} interface{}
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/passkey/add/begin [post]
func (s *APIServer) beginAddPasskey() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		// Get authenticated account
		account, err := s.Service.GetAccount(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, "Failed to get account")
		}

		// Initialize WebAuthn registration ceremony for additional credential
		options, token, err := s.Service.BeginAddPasskey(c.Context(), account)
		if err != nil {
			return HandleError(c, err, "Failed to begin add passkey")
		}

		// Set temporary session cookie
		c.Cookie(&fiber.Cookie{
			Name:     WebAuthnTempCookie,
			Value:    token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   int(SessionDuration.Seconds()),
			Expires:  time.Now().Add(SessionDuration),
		})

		return c.Status(200).JSON(options)
	}
	return handler
}

// @Summary Begin reauthentication via passkey
// @Description Initiate a WebAuthn assertion to elevate privileges (e.g., email change)
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body requests.WebAuthnReauthBeginRequest true "Reauth begin request"
// @Success 200 {object} protocol.CredentialAssertion
// @Failure 400 {object} ErrResponse
// @Failure 429 {object} ErrResponse "Rate limited"
// @Router /api/v1/webauthn/passkey/reauth/begin [post]
func (s *APIServer) beginReauth() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		var req requests.WebAuthnReauthBeginRequest
		if err := c.BodyParser(&req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}
		errs := s.Validator.Validate(req)
		if len(errs) > 0 {
			tags := make([]string, 0)
			for _, err := range errs {
				tags = append(tags, err.Tag)
			}
			return HandleError(c, ErrValidationFailed, "validation failed", tags...)
		}

		options, token, err := s.Service.BeginReauth(c.Context(), req.Purpose, accountId)
		if err != nil {
			return HandleError(c, err, err.Error())
		}

		c.Cookie(&fiber.Cookie{Name: WebAuthnTempCookie, Value: token, HTTPOnly: true, Secure: true, SameSite: fiber.CookieSameSiteLaxMode, MaxAge: int(SessionDuration.Seconds()), Expires: time.Now().Add(SessionDuration)})

		return c.Status(200).JSON(options)
	}
	return handler
}

// @Summary Finish reauthentication via passkey
// @Description Complete WebAuthn assertion and issue a short-lived reauth token
// @Tags Authentication
// @Accept json
// @Produce json
// @Success 201 {object} responses.WebAuthnReauthFinishResponse
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/passkey/reauth/finish [post]
func (s *APIServer) finishReauth() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}
		tmpToken := c.Cookies(WebAuthnTempCookie)
		if tmpToken == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}
		defer ClearCookies(c, WebAuthnTempCookie)

		httpReq, err := adaptor.ConvertRequest(c, true)
		if err != nil {
			return HandleError(c, passkey.ErrFinishLogin, err.Error())
		}

		reauthToken, err := s.Service.FinishReauth(c.Context(), tmpToken, httpReq)
		// Finish reauth assertion (authenticates user without creating a session)
		if err != nil {
			return HandleError(c, err, "Failed to finish reauth")
		}

		return c.Status(201).JSON(
			responses.WebAuthnReauthFinishResponse{
				ReauthToken: reauthToken.Value,
				ExpiresAt:   reauthToken.ExpiresAt,
			},
		)
	}
	return handler
}

// @Summary Complete adding new passkey
// @Description Complete adding a new passkey to authenticated account
// @Tags Authentication
// @Accept json
// @Produce json
// @Success 201 "Passkey addition completed successfully"
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/passkey/add/finish [post]
func (s *APIServer) finishAddPasskey() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		tmpToken := c.Cookies(WebAuthnTempCookie)
		if tmpToken == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		// Clear temporary session cookie
		defer ClearCookies(c, WebAuthnTempCookie)

		// Convert request to HTTP request
		httpReq, err := adaptor.ConvertRequest(c, true)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": passkey.ErrFinishAddPasskey,
			})
		}

		err = s.Service.FinishAddPasskey(c.Context(), tmpToken, httpReq)
		if err != nil {
			return HandleError(c, err, "Failed to finish add passkey")
		}

		return c.SendStatus(201)
	}
	return handler
}

// @Summary Get user passkeys
// @Description Get list of passkeys for authenticated user
// @Tags Authentication
// @Produce json
// @Success 200 {array} model.Credential
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/passkeys [get]
func (s *APIServer) getPasskeys() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		// Convert string ID to ObjectID
		accIDPrimitive, err := primitive.ObjectIDFromHex(accountId)
		if err != nil {
			return HandleError(c, err, "invalid account ID")
		}

		// Get passkeys for account
		passkeys, err := s.Service.GetPasskeysForAccount(c.Context(), accIDPrimitive)
		if err != nil {
			return HandleError(c, err, "Failed to get passkeys")
		}

		return c.Status(200).JSON(passkeys)
	}
	return handler
}

// @Summary Delete passkey
// @Description Delete a specific passkey
// @Tags Authentication
// @Param id path string true "Credential ID"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Router /api/v1/webauthn/passkey/{id} [delete]
func (s *APIServer) deletePasskey() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if accountId == "" {
			return HandleError(c, ErrUnauthorized, "unauthorized")
		}

		credentialID := c.Params("id")
		if credentialID == "" {
			return HandleError(c, ErrInvalidRequestBody, "credential ID required")
		}

		// Convert string ID to ObjectID
		accIDPrimitive, err := primitive.ObjectIDFromHex(accountId)
		if err != nil {
			return HandleError(c, err, "invalid account ID")
		}

		credIDPrimitive, err := primitive.ObjectIDFromHex(credentialID)
		if err != nil {
			return HandleError(c, err, "invalid credential ID")
		}

		// Delete the passkey
		err = s.Service.DeletePasskeyByID(c.Context(), credIDPrimitive, accIDPrimitive)
		if err != nil {
			return HandleError(c, err, "Failed to delete passkey")
		}

		return c.SendStatus(204)
	}
	return handler
}
