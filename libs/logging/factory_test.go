package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory(zerolog.WarnLevel)

	if factory.defaultLevel != zerolog.WarnLevel {
		t.Errorf("Expected default level to be WarnLevel, got %v", factory.defaultLevel)
	}
}

func TestNewDefaultFactory(t *testing.T) {
	factory := NewDefaultFactory()

	if factory.defaultLevel != zerolog.DebugLevel {
		t.Errorf("Expected default level to be DebugLevel, got %v", factory.defaultLevel)
	}
}

func TestFactory_ForProfile(t *testing.T) {
	factory := NewFactory(zerolog.DebugLevel)

	// Test enabled profile logger
	logger := factory.ForProfile("test-profile", true)

	if !logger.IsEnabled() {
		t.Error("Expected profile logger to be enabled")
	}

	if logger.Config().ProfileID != "test-profile" {
		t.Errorf("Expected ProfileID to be 'test-profile', got '%s'", logger.Config().ProfileID)
	}

	if logger.Config().Level != zerolog.DebugLevel {
		t.Errorf("Expected level to be DebugLevel, got %v", logger.Config().Level)
	}

	// Test disabled profile logger
	disabledLogger := factory.ForProfile("disabled-profile", false)

	if disabledLogger.IsEnabled() {
		t.Error("Expected disabled profile logger to be disabled")
	}
}

func TestFactory_ForSystem(t *testing.T) {
	factory := NewFactory(zerolog.ErrorLevel)
	logger := factory.ForSystem()

	if !logger.IsEnabled() {
		t.Error("Expected system logger to always be enabled")
	}

	if logger.Config().ProfileID != "" {
		t.Errorf("Expected system logger to have no ProfileID, got '%s'", logger.Config().ProfileID)
	}

	if logger.Config().Level != zerolog.ErrorLevel {
		t.Errorf("Expected level to be ErrorLevel, got %v", logger.Config().Level)
	}
}

func TestFactory_ForRequest(t *testing.T) {
	factory := NewFactory(zerolog.InfoLevel)

	config := LoggingConfig{
		Enabled:   true,
		Level:     zerolog.TraceLevel,
		ProfileID: "request-profile",
	}

	logger := factory.ForRequest(config)

	if !logger.IsEnabled() {
		t.Error("Expected request logger to be enabled")
	}

	if logger.Config().ProfileID != "request-profile" {
		t.Errorf("Expected ProfileID to be 'request-profile', got '%s'", logger.Config().ProfileID)
	}

	if logger.Config().Level != zerolog.TraceLevel {
		t.Errorf("Expected level to be TraceLevel, got %v", logger.Config().Level)
	}
}

// specRef: logging-behaviour.md A4
func TestFactory_ForRequest_DefaultLevel(t *testing.T) {
	factory := NewFactory(zerolog.WarnLevel)

	// Config without level should use factory's default
	config := LoggingConfig{
		Enabled:   true,
		ProfileID: "request-profile",
	}

	logger := factory.ForRequest(config)

	if logger.Config().Level != zerolog.WarnLevel {
		t.Errorf("Expected level to be WarnLevel (factory default), got %v", logger.Config().Level)
	}
}

func TestFactory_Disabled(t *testing.T) {
	factory := NewFactory(zerolog.InfoLevel)
	logger := factory.Disabled()

	if logger.IsEnabled() {
		t.Error("Expected disabled logger to be disabled")
	}
}

func TestFactory_WithLevel(t *testing.T) {
	factory := NewFactory(zerolog.InfoLevel)

	logger := factory.WithLevel(zerolog.PanicLevel, "test-profile", true)

	if !logger.IsEnabled() {
		t.Error("Expected logger to be enabled")
	}

	if logger.Config().Level != zerolog.PanicLevel {
		t.Errorf("Expected level to be PanicLevel, got %v", logger.Config().Level)
	}

	if logger.Config().ProfileID != "test-profile" {
		t.Errorf("Expected ProfileID to be 'test-profile', got '%s'", logger.Config().ProfileID)
	}

	// Test disabled logger with custom level
	disabledLogger := factory.WithLevel(zerolog.PanicLevel, "test-profile", false)

	if disabledLogger.IsEnabled() {
		t.Error("Expected logger to be disabled")
	}
}

func TestFactory_InterfaceCompliance(t *testing.T) {
	// This test ensures Factory implements FactoryInterface
	var factory FactoryInterface = NewDefaultFactory()

	// Test all interface methods
	_ = factory.ForProfile("test", true)
	_ = factory.ForSystem()
	_ = factory.ForRequest(LoggingConfig{Enabled: true})
	_ = factory.Disabled()
	_ = factory.WithLevel(zerolog.InfoLevel, "test", true)
}
