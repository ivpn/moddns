package logging

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
)

// specRef: logging-behaviour.md A2
func TestNewContextLogger_Enabled(t *testing.T) {
	config := LoggingConfig{
		Enabled:   true,
		Level:     zerolog.InfoLevel,
		ProfileID: "test-profile",
	}

	logger := NewContextLogger(config)

	if !logger.IsEnabled() {
		t.Error("Expected logger to be enabled")
	}

	if logger.Config().ProfileID != "test-profile" {
		t.Errorf("Expected ProfileID to be 'test-profile', got '%s'", logger.Config().ProfileID)
	}
}

func TestNewContextLogger_Disabled(t *testing.T) {
	config := LoggingConfig{
		Enabled: false,
		Level:   zerolog.InfoLevel,
	}

	logger := NewContextLogger(config)

	if logger.IsEnabled() {
		t.Error("Expected logger to be disabled")
	}
}

func TestNewDisabledLogger(t *testing.T) {
	logger := NewDisabledLogger()

	if logger.IsEnabled() {
		t.Error("Expected disabled logger to be disabled")
	}
}

func TestNewEnabledLogger(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.InfoLevel)

	if !logger.IsEnabled() {
		t.Error("Expected enabled logger to be enabled")
	}

	if logger.Config().ProfileID != "test-profile" {
		t.Errorf("Expected ProfileID to be 'test-profile', got '%s'", logger.Config().ProfileID)
	}
}

func TestContextLogger_LoggingOutput(t *testing.T) {
	// Test that enabled logger creates events and disabled logger doesn't
	enabledLogger := NewEnabledLogger("test-profile", zerolog.InfoLevel)
	if !enabledLogger.IsEnabled() {
		t.Error("Expected enabled logger to be enabled")
	}

	disabledLogger := NewDisabledLogger()
	if disabledLogger.IsEnabled() {
		t.Error("Expected disabled logger to be disabled")
	}

	// Test that the loggers have different configurations
	enabledConfig := enabledLogger.Config()
	disabledConfig := disabledLogger.Config()

	if enabledConfig.Enabled == disabledConfig.Enabled {
		t.Error("Expected enabled and disabled loggers to have different Enabled values")
	}
}

func TestContextLogger_WithField(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.InfoLevel)
	newLogger := logger.WithField("key", "value")

	if newLogger.Config().ProfileID != "test-profile" {
		t.Error("Expected new logger to retain ProfileID")
	}

	if !newLogger.IsEnabled() {
		t.Error("Expected new logger to retain enabled state")
	}
}

func TestContextLogger_WithFields(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.InfoLevel)

	fields := map[string]interface{}{
		"string_field": "value",
		"int_field":    42,
		"bool_field":   true,
	}

	newLogger := logger.WithFields(fields)

	if newLogger.Config().ProfileID != "test-profile" {
		t.Error("Expected new logger to retain ProfileID")
	}

	if !newLogger.IsEnabled() {
		t.Error("Expected new logger to retain enabled state")
	}
}

func TestContextLogger_Level(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.InfoLevel)
	newLogger := logger.Level(zerolog.ErrorLevel)

	if newLogger.Config().Level != zerolog.ErrorLevel {
		t.Errorf("Expected level to be ErrorLevel, got %v", newLogger.Config().Level)
	}

	if newLogger.Config().ProfileID != "test-profile" {
		t.Error("Expected new logger to retain ProfileID")
	}
}

// specRef: logging-behaviour.md A1
func TestContextLogger_DisabledOutput(t *testing.T) {
	// Test that disabled logger produces no output at all
	disabledLogger := NewDisabledLogger()

	// Try to capture any potential output
	var discardedBytes int64

	// Create a custom writer that counts bytes
	counter := &byteCounter{}

	// Override the logger's output temporarily
	originalLogger := disabledLogger.logger
	disabledLogger.logger = zerolog.New(counter)

	// Try various logging methods
	disabledLogger.Trace().Msg("trace message")
	disabledLogger.Debug().Msg("debug message")
	disabledLogger.Info().Msg("info message")
	disabledLogger.Warn().Msg("warn message")
	disabledLogger.Error().Msg("error message")

	discardedBytes = counter.count
	disabledLogger.logger = originalLogger

	if discardedBytes > 0 {
		t.Errorf("Expected disabled logger to discard all output, but %d bytes were written", discardedBytes)
	}
}

// Helper type to count written bytes
type byteCounter struct {
	count int64
}

func (bc *byteCounter) Write(p []byte) (n int, err error) {
	bc.count += int64(len(p))
	return len(p), nil
}

func TestContextLogger_AllLogLevels(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.TraceLevel)

	// Test that all log level methods are available and don't panic
	logger.Trace().Msg("trace")
	logger.Debug().Msg("debug")
	logger.Info().Msg("info")
	logger.Warn().Msg("warn")
	logger.Error().Msg("error")

	// Note: Not testing Fatal and Panic as they would terminate the test
}

func TestContextLogger_ErrMethod(t *testing.T) {
	logger := NewEnabledLogger("test-profile", zerolog.InfoLevel)

	// Test that Err method works without panicking
	testErr := io.ErrUnexpectedEOF

	// These calls should not panic
	logger.Err(testErr).Msg("test error")
	logger.Err(nil).Msg("no error")

	// Test with disabled logger - should also not panic
	disabledLogger := NewDisabledLogger()
	disabledLogger.Err(testErr).Msg("disabled error")
	disabledLogger.Err(nil).Msg("disabled no error")

	// If we get here without panicking, the test passes
}
