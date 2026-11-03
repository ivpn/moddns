package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service"
	"github.com/rs/zerolog/log"
)

func NewAuth(service service.Servicer, cfg *config.APIConfig, filter func(c *fiber.Ctx) bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Cookies(auth.AUTH_COOKIE)
		if sessionToken != "" {
			session, ok, err := service.GetSession(c.UserContext(), sessionToken)
			if err != nil {
				log.Ctx(c.UserContext()).Err(err).Msg("Failed to get session")
				return c.SendStatus(fiber.StatusServiceUnavailable)
			}
			// Login sessions never carry a WebAuthn challenge; a stored document that
			// does is ceremony state, not a login session.
			if ok && session.SessionData.Challenge == "" {
				c.Locals(auth.ACCOUNT_ID, session.AccountID)
				c.Locals(auth.SESSION_TOKEN, sessionToken)
				return c.Next()
			}
		}

		return c.SendStatus(fiber.StatusUnauthorized)
	}
}

func NewBasicAuth(cfg config.APIConfig) fiber.Handler {
	return basicauth.New(basicauth.Config{
		Users: map[string]string{
			cfg.BasicAuthUser: cfg.BasicAuthPassword,
		},
	})
}
