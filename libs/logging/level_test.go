package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

// specRef: logging-behaviour.md C1
func TestParseLevel(t *testing.T) {
	tests := []struct {
		in     string
		want   zerolog.Level
		wantOk bool
	}{
		{"trace", zerolog.TraceLevel, true},
		{"debug", zerolog.DebugLevel, true},
		{"info", zerolog.InfoLevel, true},
		{"warn", zerolog.WarnLevel, true},
		{"warning", zerolog.WarnLevel, true},
		{"error", zerolog.ErrorLevel, true},
		{"fatal", zerolog.FatalLevel, true},
		{"panic", zerolog.PanicLevel, true},
		{"disabled", zerolog.Disabled, true},
		{"DEBUG", zerolog.DebugLevel, true},
		{" info ", zerolog.InfoLevel, true},
		{"", zerolog.InfoLevel, false},
		{"bogus", zerolog.InfoLevel, false},
	}

	for _, tc := range tests {
		got, ok := ParseLevel(tc.in)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("ParseLevel(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOk)
		}
	}
}
