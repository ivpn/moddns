package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
)

// @Summary Delete all other sessions
// @Description Delete all sessions for the current account except the current session. The account owner is verified first with a reauth token issued for the sessions_delete purpose.
// @Tags Sessions
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.SessionsDeleteRequest true "Verification"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/sessions [delete]
func (s *APIServer) deleteAllOtherSessions() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountID, ok := c.Locals(auth.ACCOUNT_ID).(string)
		if !ok || accountID == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		currentToken, ok := c.Locals(auth.SESSION_TOKEN).(string)
		if !ok || currentToken == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		req := new(requests.SessionsDeleteRequest)
		if err := c.BodyParser(req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}
		errMsgs := s.Validator.ValidateRequest(c, req, ErrFailedToDeleteSessions.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		if err := s.Service.DeleteOtherSessions(c.UserContext(), accountID, currentToken, *req); err != nil {
			return HandleError(c, err, ErrFailedToDeleteSessions.Error())
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
