package middleware

import (
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
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
			session, ok, err := service.GetSession(c.Context(), sessionToken)
			if err != nil {
				log.Err(err).Msg("Failed to get session")
				return c.SendStatus(fiber.StatusServiceUnavailable)
			}
			if ok {
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

func NewWebAuthn(cfg config.Config) *webauthn.WebAuthn {
	var webAuthn *webauthn.WebAuthn
	config := &webauthn.Config{
		RPDisplayName: cfg.Server.Name,                  // Display Name for your site
		RPID:          cfg.Server.FQDN,                  // Generally the FQDN for your site
		RPOrigins:     []string{cfg.API.ApiAllowOrigin}, // The origin URLs allowed for WebAuthn requests
	}

	webAuthn, err := webauthn.New(config)
	if err != nil {
		fmt.Println(err)
	}

	return webAuthn
}
