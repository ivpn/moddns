package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	adlog "github.com/AdguardTeam/golibs/log"
	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/ivpn/dns/proxy/cache/memory"
	"github.com/ivpn/dns/proxy/collector"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/internal/metrics"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/server"
	"github.com/ivpn/dns/proxy/utils"
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
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	serverConfig, err := config.New()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load server configuration")
	}

	// Set logging level for zerolog from configuration
	zerologLevel := utils.ParseZerologLevel(serverConfig.Log.ZerologLevel)
	zerolog.SetGlobalLevel(zerologLevel)

	// Set logging level for underlying AdGuard log library from configuration
	adlogLevel := utils.ParseAdGuardLogLevel(serverConfig.Log.AdGuardLogLevel)
	adlog.SetLevel(adlogLevel)

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              serverConfig.Sentry.DSN,
		Environment:      serverConfig.Sentry.Environment,
		Release:          serverConfig.Sentry.Release,
		TracesSampleRate: 1.0,
		AttachStacktrace: true,
		EnableTracing:    true,
	}); err != nil {
		log.Panic().Err(err).Msg("Failed to initialize Sentry")
	}

	// Configure Zerolog to use Sentry as a writer
	sentryWriter, err := sentryzerolog.New(sentryzerolog.Config{
		ClientOptions: sentry.ClientOptions{
			Dsn:         serverConfig.Sentry.DSN,
			Environment: serverConfig.Sentry.Environment,
			Release:     serverConfig.Sentry.Release,
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

	emitterI, err := emitter.NewEmitter(serverConfig.Emitter.SinkConfig)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create emitter")
	}

	defer func() {
		shutdown(nil, emitterI, sentryWriter, nil)
	}()

	quit := make(chan struct{})
	queryLogsCollector, err := collector.NewCollector(serverConfig.CollectorQueryLogs, model.TYPE_QUERY_LOGS, quit, emitterI)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create query logs collector")
	}

	statsCollector, err := collector.NewCollector(serverConfig.CollectorStatistics, model.TYPE_STATISTICS, quit, emitterI)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create statistics collector")
	}

	collectorChannels := map[string]channel.CollectorChannel{
		model.TYPE_QUERY_LOGS: queryLogsCollector.GetChannel(),
		model.TYPE_STATISTICS: statsCollector.GetChannel(),
	}

	server, err := server.NewServer(serverConfig, collectorChannels)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create server")
	}

	// log in memory cache stats
	go safelyRun(func() {
		ticker := time.NewTicker(memory.StatsLoggingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				server.InMemoryCache.Stats()
			case <-quit:
				return
			}
		}
	})

	go safelyRun(func() {
		_ = queryLogsCollector.Collect()
	})

	go safelyRun(func() {
		_ = statsCollector.Collect()
	})

	var metricsServer *metrics.Server
	if serverConfig.Metrics.Port > 0 {
		metricsServer = metrics.New(serverConfig.Metrics.Port)
		go safelyRun(func() {
			if err := metricsServer.Start(); err != nil {
				log.Error().Err(err).Msg("Metrics server error")
			}
		})
	}

	go safelyRun(func() {
		ctx := context.Background()
		if err := server.Proxy.Start(ctx); err != nil {
			log.Panic().Err(err).Msg("Failed to start proxy")
		}
	})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	<-signals
	close(quit)
	shutdown(server, emitterI, sentryWriter, metricsServer)
}

// safelyRun wraps each goroutine with panic recovery to ensure the application continues even if a panic occurs
func safelyRun(fn func()) {
	go func() {
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
	}()
}

func shutdown(server *server.Server, emitterI emitter.Emitter, sentryWriter *sentryzerolog.Writer, metricsServer *metrics.Server) {
	log.Info().Msg("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to shutdown metrics server")
		}
	}

	if server != nil {
		if err := server.Proxy.Shutdown(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to shutdown proxy")
		}
	}

	if err := sentryWriter.Close(); err != nil {
		log.Warn().Err(err).Msg("Failed to flush Sentry writer")
	}

	log.Info().Msg("Disconnecting from database")
	if err := emitterI.Disconnect(); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect from database")
	}
}
