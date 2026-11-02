package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	adlog "github.com/AdguardTeam/golibs/log"
	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/ivpn/dns/libs/telemetry"
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
	config.WarnStatisticsSettingsTTL(serverConfig.Server.ProfileSettingsCacheTTL)

	// Set logging level for zerolog from configuration
	zerologLevel := utils.ParseZerologLevel(serverConfig.Log.ZerologLevel)
	zerolog.SetGlobalLevel(zerologLevel)

	// Set logging level for underlying AdGuard log library from configuration
	adlogLevel := utils.ParseAdGuardLogLevel(serverConfig.Log.AdGuardLogLevel)
	adlog.SetLevel(adlogLevel)

	sentryConfig := telemetry.Config{
		DSN:         serverConfig.Sentry.DSN,
		Environment: serverConfig.Sentry.Environment,
		Release:     serverConfig.Sentry.Release,
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

	emitterI, err := emitter.NewEmitter(serverConfig.Emitter.SinkConfig)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create emitter")
	}

	quit := make(chan struct{})
	var collectors sync.WaitGroup
	closeQuit := sync.OnceFunc(func() { close(quit) })
	stopCollectors := func() {
		closeQuit()
		waitFor(&collectors, collectorFlushTimeout)
	}
	defer func() {
		shutdown(nil, emitterI, sentryWriter, nil, stopCollectors)
	}()

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

	go safelyRun(trackRun(&collectors, func() {
		_ = queryLogsCollector.Collect()
	}))

	go safelyRun(trackRun(&collectors, func() {
		_ = statsCollector.Collect()
	}))

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
	shutdown(server, emitterI, sentryWriter, metricsServer, stopCollectors)
}

// collectorFlushTimeout bounds how long shutdown waits for collectors: the fleet
// and consented statistics flushes each get up to one EmitTimeout.
const collectorFlushTimeout = 2*collector.EmitTimeout + 5*time.Second

// trackRun marks fn done on wg only when it returns. A run that panics is
// restarted by safelyRun and stays counted until the restart returns.
func trackRun(wg *sync.WaitGroup, fn func()) func() {
	wg.Add(1)
	var once sync.Once
	return func() {
		fn()
		once.Do(wg.Done)
	}
}

// waitFor reports whether wg finished before the timeout.
func waitFor(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		log.Warn().Msg("Timed out waiting for collectors to flush")
		return false
	}
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

var shutdownOnce sync.Once

// shutdown stops the listeners first, then lets the collectors flush, and only
// then disconnects from the database. It runs once: a signal exit calls it twice.
func shutdown(server *server.Server, emitterI emitter.Emitter, sentryWriter *sentryzerolog.Writer, metricsServer *metrics.Server, stopCollectors func()) {
	shutdownOnce.Do(func() {
		doShutdown(server, emitterI, sentryWriter, metricsServer, stopCollectors)
	})
}

func doShutdown(server *server.Server, emitterI emitter.Emitter, sentryWriter *sentryzerolog.Writer, metricsServer *metrics.Server, stopCollectors func()) {
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
		server.Cache.Close()
	}

	stopCollectors()

	if sentryWriter != nil {
		if err := sentryWriter.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to flush Sentry writer")
		}
	}

	log.Info().Msg("Disconnecting from database")
	if err := emitterI.Disconnect(); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect from database")
	}
}
