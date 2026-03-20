package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/ivpn/dns/blocklists/cache"
	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/db/mongodb"
	"github.com/ivpn/dns/blocklists/service"
	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

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
	defer sentryWriter.Close()

	log.Logger = log.Output(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stderr}, sentryWriter))

	updater, err := updater.New(appConfig.Updater.Type)
	if err != nil {
		log.Panic().Err(err).Msg("failed to create updater")
	}

	storeI, err := store.New(store.DbTypeMongoDb, appConfig.DB)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create database struct")
	}
	db, err := mongodb.New(storeI, appConfig.DB)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create database instance")
	}

	cache, err := cache.NewCache(appConfig.Cache, cache.CacheTypeRedis)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create cache")
	}

	service := service.New(*appConfig, db, cache, updater)
	sources, err := service.ReadSources()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to read sources")
	}
	if err = service.Setup(sources); err != nil {
		log.Panic().Err(err).Msg("Failed to setup service")
	}

	service.Trigger(sources)
	service.PurgeStale(sources)

	updater.Start()
	defer updater.Stop()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	exitChan := make(chan int)

	go safelyRun(
		func() {
			for {
				s := <-signalChan
				switch s {
				case syscall.SIGHUP:
					log.Info().Msg("SIGHUP signal detected, re-read configuration")
					log.Error().Msg("Not implemented yet")
					exitChan <- 1
				case syscall.SIGINT: // Ctrl+C
					log.Info().Msg("SIGINT signal detected, stopping")
					updater.Stop()
					exitChan <- 0
				case syscall.SIGTERM:
					log.Info().Msg("SIGTERM signal detected, terminating app gracefully")
					updater.Stop()
					exitChan <- 0
				case syscall.SIGQUIT:
					log.Info().Msg("SIGQUIT signal detected, stop and core dump")
					updater.Stop()
					exitChan <- 0
				default:
					log.Warn().Msgf("Unknown signal")
					exitChan <- 1
				}
			}
		},
	)

	code := <-exitChan
	os.Exit(code)
}

// safelyRun wraps each goroutine with panic recovery to ensure the application continues even if a panic occurs
func safelyRun(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic details
			log.Error().Interface("panic", r).Msg("Recovered from panic in goroutine")
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
			// This may cause stack overflow, needs to be tested
			go safelyRun(func() {
				fn()
			})
		}
	}()
	fn()
}
