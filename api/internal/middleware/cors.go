package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/ivpn/dns/api/config"
	"github.com/rs/zerolog/log"
)

func NewAPICORS(cfg config.APIConfig) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     cfg.ApiAllowOrigin,
		AllowCredentials: true,
		// Content-Disposition: mobileconfig short-link + export downloads.
		// X-modDNS-Export-Truncated: lets the export UI warn when rules were trimmed.
		ExposeHeaders: "Content-Disposition,X-modDNS-Export-Truncated",
	})
}

func NewIPCORS(cfg config.APIConfig) fiber.Handler {

	return func(c *fiber.Ctx) error {
		log.Trace().Str("IP", c.IP()).Msg("Remote IP address of request")
		if cfg.ApiAllowIP != "*" && c.IP() != cfg.ApiAllowIP {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Next()
	}
}
