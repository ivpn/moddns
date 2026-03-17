package middleware

import "github.com/gofiber/fiber/v2"

// NewNoCache returns middleware that sets Cache-Control and Pragma headers
// to prevent caching of API responses.
func NewNoCache() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Set("Pragma", "no-cache")
		return c.Next()
	}
}
