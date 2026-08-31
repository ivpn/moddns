package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog/log"
)

// RequestLogger embeds a child of the global logger carrying the request ID
// into the request's user context. Handlers and services log through it with
// log.Ctx(ctx), so every line of a request is correlated. Must be registered
// after the requestid middleware.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid, _ := c.Locals(requestid.ConfigDefault.ContextKey).(string)
		logger := log.Logger.With().Str("request_id", rid).Logger()
		c.SetUserContext(logger.WithContext(c.UserContext()))
		return c.Next()
	}
}
