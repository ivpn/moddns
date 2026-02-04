package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/rs/zerolog/log"
)

// @Summary Delete all other sessions
// @Description Delete all sessions for the current account except the current session
// @Tags Sessions
// @Produce json
// @Security ApiKeyAuth
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
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

		err := s.Service.DeleteSessionsByAccountIDExceptCurrent(c.Context(), accountID, currentToken)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete all sessions")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete other sessions",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
