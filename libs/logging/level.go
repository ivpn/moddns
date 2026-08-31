package logging

import (
	"strings"

	"github.com/rs/zerolog"
)

// ParseLevel maps a log-level string (typically a LOG_LEVEL env value) to a
// zerolog level. Input is trimmed and case-insensitive. Unknown or empty
// values return (InfoLevel, false) so callers can warn about misconfiguration
// while still starting with a safe default.
func ParseLevel(s string) (zerolog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel, true
	case "debug":
		return zerolog.DebugLevel, true
	case "info":
		return zerolog.InfoLevel, true
	case "warn", "warning":
		return zerolog.WarnLevel, true
	case "error":
		return zerolog.ErrorLevel, true
	case "fatal":
		return zerolog.FatalLevel, true
	case "panic":
		return zerolog.PanicLevel, true
	case "disabled":
		return zerolog.Disabled, true
	default:
		return zerolog.InfoLevel, false
	}
}
