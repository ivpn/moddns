package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db"
	_ "github.com/ivpn/dns/api/docs"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/internal/idgen"
	"github.com/ivpn/dns/api/internal/middleware"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/libs/urlshort"
)

// APIServer represents an API server
type APIServer struct {
	App       *fiber.App
	Service   service.Service // TODO: Should be service.Servicer interface
	Config    *config.Config
	Validator *validator.APIValidator
	Db        db.Db
	Cache     cache.Cache
	IdGen     idgen.Generator
	Mailer    email.Mailer
	Shortener *urlshort.URLShortener
}

// NewServer inititiates database connection and sets up API endpoints
func NewServer(config *config.Config, service service.Service, db db.Db, cache cache.Cache, idGen idgen.Generator, apiValidator *validator.APIValidator, email email.Mailer, shortener *urlshort.URLShortener) (*APIServer, error) {
	app := fiber.New(fiber.Config{
		ServerHeader: "modDNS API",
		AppName:      "modDNS API",
		BodyLimit:    1024 * 1024, // 1 MB
	})

	server := &APIServer{
		App:       app,
		Service:   service,
		Config:    config,
		Validator: apiValidator,
		Db:        db,
		Cache:     cache,
		IdGen:     idGen,
		Mailer:    email,
		Shortener: shortener,
	}

	middleware.InitLimitConfig(config.API)
	server.setupMiddlewares()

	return server, nil
}

func (s *APIServer) setupMiddlewares() {
	s.App.Use(middleware.SentryFiber())
	s.App.Use(middleware.Recover())
	s.App.Use(requestid.New())
	s.App.Use(logger.New())
	s.App.Use(helmet.New(helmet.Config{
		HSTSMaxAge:            31536000,
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		PermissionPolicy:      "camera=(), microphone=(), geolocation=()",
	}))
	s.App.Use(middleware.NewAccepts(fiber.MIMEApplicationJSON))
	s.App.Use(
		healthcheck.New(
			healthcheck.Config{
				LivenessEndpoint:  "/health/live",
				ReadinessEndpoint: "/health/ready",
				ReadinessProbe: func(c *fiber.Ctx) bool {
					ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
					defer cancel()
					return s.Db.GetClient().Ping(ctx, nil) == nil
				},
			},
		),
	)
}

// RegisterRoutes registers API endpoints
func (s *APIServer) RegisterRoutes() {
	api := s.App.Group("/api")
	v1 := api.Group("/v1")
	v1.Use(middleware.NewNoCache())

	api.Use(middleware.NewAPICORS(*s.Config.API))
	api.Use(middleware.NewIPCORS(*s.Config.API))

	v1.Post("/login", middleware.NewLimit(10, 1*time.Minute), s.login())

	// PSK-provisioning subscription endpoint (outside v1 auth chain)
	subscriptions := s.App.Group("/api/v1/subscription")
	subscriptions.Use(middleware.NewPSK(*s.Config.API))
	subscriptions.Post("/add", middleware.NewLimit(10, 1*time.Minute), s.addSubscription())

	accounts := v1.Group("/accounts")
	profiles := v1.Group("/profiles")
	verify := v1.Group("/verify")
	mobileconfig := v1.Group("/mobileconfig")
	sessions := v1.Group("/sessions")
	blocklists := v1.Group("/blocklists")
	webauthn := v1.Group("/webauthn")
	sub := v1.Group("/sub")

	// Unrestricted account endpoints
	accounts.Post("", middleware.NewLimit(20, 1*time.Minute), s.registerAccount())
	accounts.Post("/reset-password", middleware.NewLimit(10, 1*time.Minute), s.sendResetPasswordEmail())

	// WebAuthn endpoints (unrestricted for registration and login)
	webauthn.Post("/register/begin", middleware.NewLimit(10, 1*time.Minute), s.beginRegistration())
	webauthn.Post("/register/finish", middleware.NewLimit(10, 1*time.Minute), s.finishRegistration())
	webauthn.Post("/login/begin", middleware.NewLimit(10, 1*time.Minute), s.beginLogin())
	webauthn.Post("/login/finish", middleware.NewLimit(10, 1*time.Minute), s.finishLogin())

	// Unrestricted short URL endpoint
	v1.Get("/short/:code", middleware.NewLimit(10, 1*time.Minute), s.downloadMobileConfigFromLink())

	// Verification endpoints
	verify.Post("/reset-password", middleware.NewLimit(10, 1*time.Minute), s.verifyPasswordReset())

	// Protected endpoints start here (note: only v1 group is protected)
	v1.Use(middleware.NewAuth(&s.Service, s.Config.API, func(c *fiber.Ctx) bool { return true }))

	// Subscription (protected) endpoint (session auth only)
	sub.Get("", middleware.NewLimit(40, 1*time.Minute), s.getSubscription())

	// Email verification OTP (requires auth)
	verify.Post("/email/otp/request", middleware.NewLimit(10, 1*time.Minute), s.requestEmailVerificationOTP())
	verify.Post("/email/otp/confirm", middleware.NewLimit(10, 1*time.Minute), s.verifyEmailOTP())

	blocklists.Get("", middleware.NewLimit(20, 1*time.Minute), s.getBlocklists())

	// Protected WebAuthn endpoints (require authentication)
	webauthn.Post("/passkey/add/begin", middleware.NewLimit(10, 1*time.Minute), s.beginAddPasskey())
	webauthn.Post("/passkey/add/finish", middleware.NewLimit(10, 1*time.Minute), s.finishAddPasskey())
	// Passkey reauthentication for privileged operations
	webauthn.Post("/passkey/reauth/begin", middleware.NewLimit(10, 1*time.Minute), s.beginReauth())
	webauthn.Post("/passkey/reauth/finish", middleware.NewLimit(10, 1*time.Minute), s.finishReauth())
	webauthn.Get("/passkeys", middleware.NewLimit(20, 1*time.Minute), s.getPasskeys())
	webauthn.Delete("/passkey/:id", middleware.NewLimit(10, 1*time.Minute), s.deletePasskey())

	// MobileConfig endpoints
	mobileconfig.Post("", middleware.NewLimit(20, 1*time.Minute), s.generateMobileConfig())
	mobileconfig.Post("/short", middleware.NewLimit(20, 1*time.Minute), s.generateMobileConfigShortLink())

	// Accounts endpoints
	accounts.Post("/logout", middleware.NewLimit(20, 1*time.Minute), s.logout())
	accounts.Get("/current", middleware.NewLimit(40, 1*time.Minute), s.getAccount())
	accounts.Patch("", middleware.NewLimit(20, 1*time.Minute), s.updateAccount())
	accounts.Post("/current/deletion-code", middleware.NewLimit(10, 1*time.Minute), s.generateDeletionCode())
	accounts.Delete("/current", middleware.NewLimit(5, 10*time.Minute), s.deleteAccount())

	// 2FA endpoints
	accounts.Post("/mfa/totp/enable", middleware.NewLimit(10, 1*time.Minute), s.TotpEnable())
	accounts.Post("/mfa/totp/enable/confirm", middleware.NewLimit(10, 1*time.Minute), s.confirm2FA())
	accounts.Post("/mfa/totp/disable", middleware.NewLimit(10, 1*time.Minute), s.disable2FA())

	// Profiles endpoints
	profiles.Post("", middleware.NewLimit(30, 1*time.Minute), s.createProfile())
	profiles.Get("", middleware.NewLimit(30, 1*time.Minute), s.getProfiles())
	profiles.Get("/:id", middleware.NewLimit(30, 1*time.Minute), s.getProfile())
	profiles.Delete("/:id", middleware.NewLimit(20, 1*time.Minute), s.deleteProfile())
	profiles.Patch("/:id", middleware.NewLimit(20, 1*time.Minute), s.updateProfile())

	// Query logs endpoints
	profiles.Get("/:id/logs", middleware.NewLimit(500, 1*time.Minute), s.getProfileQueryLogs())
	profiles.Get("/:id/logs/download", middleware.NewLimit(20, 1*time.Minute), s.downloadProfileQueryLogs())
	profiles.Delete("/:id/logs", middleware.NewLimit(20, 1*time.Minute), s.deleteProfileQueryLogs())

	// Statistics endpoints
	profiles.Get("/:id/statistics", middleware.NewLimit(20, 1*time.Minute), s.getStatistics())

	// Custom rules endpoints
	profiles.Delete("/:profile_id/custom_rules/:custom_rule_id", middleware.NewLimit(20, 1*time.Minute), s.deleteProfileCustomRule())
	profiles.Post("/:id/custom_rules/batch", middleware.NewLimit(20, 1*time.Minute), s.createProfileCustomRulesBatch())
	profiles.Post("/:id/custom_rules", middleware.NewLimit(20, 1*time.Minute), s.createProfileCustomRule())

	// Blocklists endpoints
	profiles.Post("/:id/blocklists", middleware.NewLimit(20, 1*time.Minute), s.enableBlocklists())
	profiles.Delete("/:id/blocklists", middleware.NewLimit(20, 1*time.Minute), s.disableBlocklists())

	// Session endpoints (respect global disable flag via conditional wrapper)
	if s.Config.API.DisableRateLimit {
		sessions.Delete("", func(c *fiber.Ctx) error { return c.Next() }, s.deleteAllOtherSessions())
	} else {
		// Using default limiter settings for this destructive operation
		sessions.Delete("", limiter.New(), s.deleteAllOtherSessions())
	}

	docs := s.App.Group("/docs")
	docs.Use(middleware.NewBasicAuth(*s.Config.API))
	docs.Get("/*", middleware.NewLimit(10, 1*time.Minute), swagger.HandlerDefault)
}
