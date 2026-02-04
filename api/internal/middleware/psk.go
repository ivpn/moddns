package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
)

func GetToken(c *fiber.Ctx) string {
	var tokenString string
	authorization := c.Get("Authorization")

	if after, ok := strings.CutPrefix(authorization, "Bearer "); ok {
		tokenString = after
	} else if c.Cookies(auth.AUTH_COOKIE) != "" {
		tokenString = c.Cookies(auth.AUTH_COOKIE)
	}

	return tokenString
}

func NewPSK(cfg config.APIConfig) fiber.Handler {

	return func(c *fiber.Ctx) error {
		token := GetToken(c)
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.PSK)) != 1 {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		return c.Next()
	}
}
