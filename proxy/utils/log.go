package utils

import (
	adlog "github.com/AdguardTeam/golibs/log"
	"github.com/ivpn/dns/libs/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ParseAdGuardLogLevel converts a string log level to adlog.Level
func ParseAdGuardLogLevel(levelStr string) adlog.Level {
	switch levelStr {
	case "error":
		return adlog.ERROR
	case "info":
		return adlog.INFO
	case "debug":
		return adlog.DEBUG
	default:
		log.Warn().Str("level", levelStr).Msg("Invalid AdGuard log level, defaulting to INFO")
		return adlog.INFO
	}
}

// ParseZerologLevel converts a string log level to zerolog.Level via the
// shared libs/logging parser, warning on unrecognized non-empty values.
func ParseZerologLevel(levelStr string) zerolog.Level {
	level, ok := logging.ParseLevel(levelStr)
	if !ok && levelStr != "" {
		log.Warn().Str("level", levelStr).Msg("Invalid zerolog log level, defaulting to INFO")
	}
	return level
}
