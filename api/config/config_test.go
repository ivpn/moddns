package config

import (
	"testing"

	"github.com/rs/zerolog"
)

// specRef: logging-behaviour.md C1
func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		in   string
		want zerolog.Level
	}{
		{"trace", zerolog.TraceLevel},
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"disabled", zerolog.Disabled},
		{"DEBUG", zerolog.DebugLevel},
		{" info ", zerolog.InfoLevel},
		{"", zerolog.InfoLevel},
		{"bogus", zerolog.InfoLevel},
	}

	for _, tc := range tests {
		if got := parseLogLevel(tc.in); got != tc.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
