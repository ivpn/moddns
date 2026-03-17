package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/ivpn/dns/api/config"
	"github.com/rs/zerolog/log"
)

// limitDisabled is set once; we rely on config passed in at startup.
var limitDisabled bool

// InitLimitConfig captures whether rate limiting should be disabled globally.
// Call this early during server initialization (e.g., before registering routes).
func InitLimitConfig(cfg *config.APIConfig) {
	if cfg == nil {
		return
	}
	limitDisabled = cfg.DisableRateLimit
	if limitDisabled {
		log.Info().Msg("Rate limiting disabled by configuration (API_DISABLE_RATE_LIMIT=true)")
	}
}

// NewLimit returns a Fiber handler implementing a simple request rate limit.
// When global disabling is active, it returns a passthrough handler to avoid
// interfering with integration tests.
func NewLimit(max int, exp time.Duration) fiber.Handler {
	if limitDisabled {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: exp,
		KeyGenerator: func(c *fiber.Ctx) string {
			// X-Forwarded-For header is taken from nginx ingress controller
			xff := c.Get(fiber.HeaderXForwardedFor)
			log.Trace().Str("ip", c.IP()).Str("x-forwarded-for", xff).Str("path", c.Path()).Msg("rate limiter client IP")
			return xff
		},
	})
}
