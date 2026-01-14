package main

import (
	"context"
	"os"
	"time"

	"github.com/ivpn/dns/api/api"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db/mongodb"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/internal/idgen"
	"github.com/ivpn/dns/api/internal/middleware"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/libs/store"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/ivpn/dns/libs/urlshort"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	shortUrlTTL         = 5 * time.Minute
	shortUrlLogInterval = 1 * time.Hour
)

// @title           modDNS REST API
// @version         1.0
// @description     modDNS REST API
func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	appConfig, err := config.New()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to read app configuration")
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              appConfig.Sentry.DSN,
		Environment:      appConfig.Sentry.Environment,
		Release:          appConfig.Sentry.Release,
		TracesSampleRate: 1.0,
		AttachStacktrace: true,
		EnableTracing:    true,
	}); err != nil {
		log.Panic().Err(err).Msg("Failed to initialize Sentry")
	}

	// Configure Zerolog to use Sentry as a writer
	sentryWriter, err := sentryzerolog.New(sentryzerolog.Config{
		ClientOptions: sentry.ClientOptions{
			Dsn:         appConfig.Sentry.DSN,
			Environment: appConfig.Sentry.Environment,
			Release:     appConfig.Sentry.Release,
		},
		Options: sentryzerolog.Options{
			Levels:          []zerolog.Level{zerolog.WarnLevel, zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel},
			WithBreadcrumbs: true,
			FlushTimeout:    3 * time.Second,
		},
	})
	if err != nil {
		log.Panic().Err(err).Msg("failed to create sentry writer")
	}
	defer func() {
		log.Info().Msg("Flushing Sentry writer")
		sentryWriter.Close()
	}()

	log.Logger = log.Output(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stderr}, sentryWriter))

	storeI, err := store.New(store.DbTypeMongoDb, appConfig.DB)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create database struct")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := mongodb.New(ctx, storeI, appConfig)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create MongoDB instance")
	}

	if err = db.Migrate(); err != nil {
		log.Panic().Err(err).Msg("Failed to run migrations")
	}

	// cache create, load data on startup
	cache, err := cache.NewCache(appConfig.Cache, cache.CacheTypeRedis)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create cache")
	}

	idGen, err := idgen.NewGenerator(idgen.TypeSqids, appConfig.API.ProfileIDMinLength)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create ID generator")
	}

	defer func() {
		log.Info().Msg("Disconnecting from database")
		if err = db.Disconnect(); err != nil {
			log.Error().Err(err).Msg("Failed to disconnect from database")
		}
	}()

	mailer, err := email.NewMailer(appConfig.Server.FrontendDomain, appConfig.Email.SenderType, appConfig.Email.InboxId, appConfig.Email.AuthToken)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create email sender")
	}

	// Create a URL shortener with a 5 minute TTL for URLs
	shortener := urlshort.NewURLShortener(
		urlshort.WithDefaultTTL(shortUrlTTL),
		urlshort.WithShortLength(8),
		urlshort.WithStatsLogging(shortUrlLogInterval),
	)

	apiValidator, err := validator.NewAPIValidator()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create API validator")
	}

	webAuthn := middleware.NewWebAuthn(*appConfig)
	service := service.New(*appConfig, db, cache, idGen, apiValidator, mailer, shortener, webAuthn)

	server, err := api.NewServer(appConfig, service, db, cache, idGen, apiValidator, mailer, shortener)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create API server")
	}
	server.RegisterRoutes()
	err = server.App.Listen(appConfig.API.Port)
	log.Panic().Err(err).Msg("Failed to start REST API")
}
