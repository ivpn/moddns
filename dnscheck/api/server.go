package api

import (
	"github.com/dnscheck/cache"
	"github.com/dnscheck/config"

	"github.com/dnscheck/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// APIServer represents an API server
type APIServer struct {
	App       *fiber.App
	Config    *config.Config
	Validator *APIValidator
	Cache     cache.Cache
}

// NewServer inititiates database connection and sets up API endpoints
func NewServer(config *config.Config, cache cache.Cache) *APIServer {
	app := fiber.New(fiber.Config{
		ServerHeader: "DNSCHECK API",
		AppName:      "DNSCHECK API",
	})

	validate := validator.New(validator.WithRequiredStructEnabled())
	apiValidator := &APIValidator{
		Validator: validate,
	}

	return &APIServer{
		App:       app,
		Config:    config,
		Validator: apiValidator,
		Cache:     cache,
	}
}

// RegisterRoutes registers API endpoints
func (s *APIServer) RegisterRoutes() {
	s.App.Use(requestid.New())
	s.App.Use(logger.New())
	s.App.Use(limiter.New(
		limiter.Config{
			Max: 100,
		},
	))
	s.App.Use(helmet.New(helmet.Config{
		HSTSMaxAge:            31536000,
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		PermissionPolicy:      "camera=(), microphone=(), geolocation=()",
	}))
	s.App.Use(middleware.NewAPICORS(*s.Config.API))

	s.App.Get("/", s.DnsCheck())
}
