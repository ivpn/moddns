package main

import (
	"os"

	"github.com/dnscheck/api"
	"github.com/dnscheck/cache"
	"github.com/dnscheck/config"
	"github.com/dnscheck/dns"

	"github.com/ivpn/dns/libs/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logLevel, _ := logging.ParseLevel(os.Getenv("LOG_LEVEL"))
	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg, err := config.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read app configuration")
	}

	cache, err := cache.New(cache.CacheTypeBigCache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create cache")
	}

	dnssrv, err := dns.New(cfg, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create DNS server")
	}

	go func() {
		// Async DNS server
		log.Info().Msgf("Starting DNS UDP server on %s", dnssrv.DNSUDP.Addr)
		if err := dnssrv.DNSUDP.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Failed to set DNS server udp listener")
		}
	}()

	go func() {
		// Async DNS server
		log.Info().Msgf("Starting DNS TCP server on %s", dnssrv.DNSTCP.Addr)
		if err := dnssrv.DNSTCP.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Failed to set DNS server tcp listener")
		}
	}()

	server := api.NewServer(cfg, cache)
	server.RegisterRoutes()
	err = server.App.Listen(cfg.API.Port)
	log.Fatal().Err(err).Msg("Failed to start REST API")
}
