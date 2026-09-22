package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/ivpn/dns/blocklists/cache"
	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/db/mongodb"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/service"
	"github.com/ivpn/dns/blocklists/updater"
	"github.com/ivpn/dns/libs/store"
	"github.com/ivpn/dns/libs/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	os.Exit(run())
}

// run holds main's body so its defers execute before os.Exit.
func run() int {
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	appConfig, err := config.New()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to read app configuration")
	}
	zerolog.SetGlobalLevel(appConfig.LogLevel)

	sentryConfig := telemetry.Config{
		DSN:         appConfig.Sentry.DSN,
		Environment: appConfig.Sentry.Environment,
		Release:     appConfig.Sentry.Release,
	}

	if err := sentry.Init(telemetry.InitOptions(sentryConfig)); err != nil {
		log.Panic().Err(err).Msg("Failed to initialize Sentry")
	}

	// Configure Zerolog to use Sentry as a writer
	sentryWriter, err := sentryzerolog.New(telemetry.LogWriterConfig(sentryConfig))
	if err != nil {
		log.Panic().Err(err).Msg("failed to create sentry writer")
	}
	defer sentryWriter.Close()

	log.Logger = log.Output(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stderr}, sentryWriter))

	// Tag every log line with this instance's name so multi-node logs show
	// which DCN node did (or skipped) the work.
	if appConfig.Server.Name != "" {
		log.Logger = log.Logger.With().Str("instance", appConfig.Server.Name).Logger()
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

	// All instances write the same Redis master, so its locker is the
	// coordination point: one winner per source per tick, one purger.
	locker := cache.Locker(updater.LockKeyPrefix)

	// Build metrics: Prometheus collectors when the metrics server is enabled,
	// otherwise a no-op implementation so instrumentation is always safe.
	var mtr metrics.Updates = metrics.NoopUpdates{}
	var metricsServer *metrics.Server
	if appConfig.Metrics.Port > 0 {
		mtr = metrics.NewPromUpdates(prometheus.DefaultRegisterer)
		metricsServer = metrics.New(appConfig.Metrics.Port, func(ctx context.Context) error {
			if err := db.GetClient().Ping(ctx, nil); err != nil {
				return err
			}
			return cache.Ping(ctx)
		})
		go safelyRun(func() {
			if err := metricsServer.Start(); err != nil {
				log.Error().Err(err).Msg("Metrics server error")
			}
		})
	}

	updater, err := updater.New(appConfig.Updater.Type, updater.NewDistributedLocker(locker, mtr))
	if err != nil {
		log.Panic().Err(err).Msg("failed to create updater")
	}

	service := service.New(*appConfig, db, cache, updater, mtr, locker)
	sources, err := service.ReadSources()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to read sources")
	}
	if err = service.Setup(sources); err != nil {
		log.Panic().Err(err).Msg("Failed to setup service")
	}

	service.CatchUp(sources)
	service.PurgeStaleCoordinated(sources)

	// Stop() runs on the signal paths before the exit code is sent.
	updater.Start()

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
	if metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Warn().Err(err).Msg("metrics server shutdown error")
		}
		cancel()
	}
	return code
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
