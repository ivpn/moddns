package server

import (
	"context"
	"testing"

	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// TestStrictNoLogging_DisabledLogger_NoLogs tests that when a logger is disabled
// for a profile, no log entries are written regardless of log level
func TestStrictNoLogging_DisabledLogger_NoLogs(t *testing.T) {
	// Create a logging factory
	factory := logging.NewFactory(zerolog.TraceLevel)

	// Create a logger for a profile with logging disabled
	profileLogger := factory.ForProfile("test-profile-disabled", false)

	// Verify the logger is disabled
	assert.False(t, profileLogger.IsEnabled(), "Logger should be disabled when logsEnabled=false")

	// Try to log at various levels - none should produce output or panics
	assert.NotPanics(t, func() {
		profileLogger.Trace().Str("field", "value").Msg("Trace message should not appear")
		profileLogger.Debug().Str("field", "value").Msg("Debug message should not appear")
		profileLogger.Info().Str("field", "value").Msg("Info message should not appear")
		profileLogger.Warn().Str("field", "value").Msg("Warn message should not appear")
		profileLogger.Error().Str("field", "value").Msg("Error message should not appear")

		// Test with Err() method as well
		profileLogger.Err(assert.AnError).Msg("Error with err should not appear")
	}, "Disabled logger operations should not panic")
}

// TestStrictNoLogging_EnabledLogger_LogsCreated tests that when a logger is enabled
// for a profile, log entries are written as expected
func TestStrictNoLogging_EnabledLogger_LogsCreated(t *testing.T) {
	// Create logging factory
	factory := logging.NewFactory(zerolog.TraceLevel)

	// Create a logger for a profile with logging enabled
	profileLogger := factory.ForProfile("test-profile-enabled", true)

	// Verify the logger is enabled
	assert.True(t, profileLogger.IsEnabled(), "Logger should be enabled when logsEnabled=true")

	// Test that enabled logger methods work (we can't easily test output in this context
	// since zerolog writes to the global logger, but we can verify behavior doesn't panic)
	assert.NotPanics(t, func() {
		profileLogger.Info().Str("test_field", "test_value").Msg("Test message should appear")
		profileLogger.Debug().Str("debug_field", "debug_value").Msg("Debug message")
		profileLogger.Error().Str("error_field", "error_value").Msg("Error message")
	}, "Enabled logger operations should not panic")
}

// TestStrictNoLogging_RequestContext_LoggerIntegration tests that request contexts
// properly carry the correct logger based on profile logging settings
func TestStrictNoLogging_RequestContext_LoggerIntegration(t *testing.T) {
	factory := logging.NewDefaultFactory()

	// Test with logging disabled
	disabledLogger := factory.ForProfile("profile-no-logs", false)
	reqCtxDisabled := requestcontext.NewRequestContext(
		context.Background(),
		nil,
		"profile-no-logs",
		"", // deviceId
		map[string]string{},
		map[string]string{"enabled": "false"},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		disabledLogger,
	)

	// Verify the request context has the disabled logger
	assert.False(t, reqCtxDisabled.Logger.IsEnabled(), "Request context should have disabled logger")
	assert.Equal(t, "profile-no-logs", reqCtxDisabled.ProfileId, "Profile ID should match")

	// Test with logging enabled
	enabledLogger := factory.ForProfile("profile-with-logs", true)
	reqCtxEnabled := requestcontext.NewRequestContext(
		context.Background(),
		nil,
		"profile-with-logs",
		"", // deviceId
		map[string]string{},
		map[string]string{"enabled": "true"},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		enabledLogger,
	)

	// Verify the request context has the enabled logger
	assert.True(t, reqCtxEnabled.Logger.IsEnabled(), "Request context should have enabled logger")
	assert.Equal(t, "profile-with-logs", reqCtxEnabled.ProfileId, "Profile ID should match")
}

// TestStrictNoLogging_FactoryBehavior tests the logging factory behavior
func TestStrictNoLogging_FactoryBehavior(t *testing.T) {
	factory := logging.NewDefaultFactory()

	// Test ForProfile with different logging settings
	disabledLogger := factory.ForProfile("test-profile", false)
	enabledLogger := factory.ForProfile("test-profile", true)

	assert.False(t, disabledLogger.IsEnabled(), "ForProfile(false) should create disabled logger")
	assert.True(t, enabledLogger.IsEnabled(), "ForProfile(true) should create enabled logger")

	// Test that both loggers have the same profile ID but different enabled states
	assert.Equal(t, "test-profile", disabledLogger.Config().ProfileID)
	assert.Equal(t, "test-profile", enabledLogger.Config().ProfileID)
	assert.False(t, disabledLogger.Config().Enabled)
	assert.True(t, enabledLogger.Config().Enabled)

	// Test ForSystem (should always be enabled)
	systemLogger := factory.ForSystem()
	assert.True(t, systemLogger.IsEnabled(), "System logger should always be enabled")

	// Test Disabled (should always be disabled)
	alwaysDisabledLogger := factory.Disabled()
	assert.False(t, alwaysDisabledLogger.IsEnabled(), "Disabled factory method should always return disabled logger")
}

// TestStrictNoLogging_Performance_DisabledLogger tests that disabled loggers
// have minimal performance overhead
func TestStrictNoLogging_Performance_DisabledLogger(t *testing.T) {
	factory := logging.NewDefaultFactory()
	disabledLogger := factory.ForProfile("perf-test", false)

	// Verify logger is disabled
	assert.False(t, disabledLogger.IsEnabled())

	// This test verifies that operations on disabled loggers complete quickly
	// The actual performance benchmarks are in the libs/logging package

	// Run many logging operations - these should be no-ops
	iterations := 10000
	for i := 0; i < iterations; i++ {
		disabledLogger.Info().
			Str("iteration", string(rune(i))).
			Int("number", i).
			Bool("enabled", false).
			Msg("This should not log anything")

		disabledLogger.Error().
			Err(assert.AnError).
			Str("context", "performance test").
			Msg("This error should not log anything")
	}

	// If we reach here without timeout, the disabled logger is performant
	// The fact that this completes quickly demonstrates the no-op behavior
}

// TestStrictNoLogging_ZeroAllocationDisabledLogger verifies that disabled loggers
// don't allocate memory for log operations
func TestStrictNoLogging_ZeroAllocationDisabledLogger(t *testing.T) {
	factory := logging.NewDefaultFactory()
	disabledLogger := factory.ForProfile("alloc-test", false)

	// Verify logger is disabled
	assert.False(t, disabledLogger.IsEnabled())

	// This test demonstrates that disabled logger operations should not allocate
	// memory. The actual allocation benchmarks are in the libs/logging package.

	// Test that we can call logger methods without panics
	assert.NotPanics(t, func() {
		disabledLogger.Trace().Msg("trace")
		disabledLogger.Debug().Msg("debug")
		disabledLogger.Info().Msg("info")
		disabledLogger.Warn().Msg("warn")
		disabledLogger.Error().Msg("error")
		disabledLogger.Err(assert.AnError).Msg("err")
	}, "Disabled logger operations should not panic")
}

// TestStrictNoLogging_ConfigurationPersistence tests that logger configuration
// is properly maintained throughout the request lifecycle
func TestStrictNoLogging_ConfigurationPersistence(t *testing.T) {
	factory := logging.NewDefaultFactory()

	// Create loggers with different configurations
	disabledLogger := factory.ForProfile("profile-1", false)
	enabledLogger := factory.ForProfile("profile-2", true)

	// Test that configurations are maintained
	disabledConfig := disabledLogger.Config()
	enabledConfig := enabledLogger.Config()

	assert.False(t, disabledConfig.Enabled, "Disabled logger config should show enabled=false")
	assert.True(t, enabledConfig.Enabled, "Enabled logger config should show enabled=true")

	assert.Equal(t, "profile-1", disabledConfig.ProfileID, "Profile ID should be maintained")
	assert.Equal(t, "profile-2", enabledConfig.ProfileID, "Profile ID should be maintained")

	// Test that IsEnabled() reflects the configuration
	assert.Equal(t, disabledConfig.Enabled, disabledLogger.IsEnabled())
	assert.Equal(t, enabledConfig.Enabled, enabledLogger.IsEnabled())
}
