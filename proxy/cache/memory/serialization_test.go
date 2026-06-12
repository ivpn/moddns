package memory

import (
	"context"
	"testing"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestContextSerialization(t *testing.T) {
	// Create a test cache
	cacheCfg := &cache.Config{}
	profileIDCache, err := NewBigcache(cacheCfg)
	require.NoError(t, err)

	// Create a logger factory and logger
	factory := logging.NewFactory(zerolog.DebugLevel)
	logger := factory.ForProfile("test-profile", false) // disabled logger

	// Create a request context with the logger
	reqCtx := requestcontext.NewRequestContext(
		context.Background(),
		nil,
		"test-profile",
		"", // deviceId
		map[string]string{"privacy": "setting"},
		map[string]string{"logs": "setting"},
		map[string]string{"dnssec": "enabled"},
		map[string]string{"enabled": "false"},
		map[string]string{"advanced": "setting"},
		logger,
	)

	// Verify the logger is set correctly
	assert.False(t, reqCtx.Logger.IsEnabled(), "Logger should be disabled")
	assert.Equal(t, "test-profile", reqCtx.LoggerConfig.ProfileID, "Logger config should have correct profile ID")
	assert.False(t, reqCtx.LoggerConfig.Enabled, "Logger config should show enabled=false")

	// UpstreamName must survive the cache round-trip — EmitQueryLog needs it to pick
	// the recursor for the DNSSEC-failure CD probe. Regression guard: it was json:"-".
	reqCtx.UpstreamName = "knot"

	// Test serialization by setting in cache
	requestID := "test-request-123"
	err = profileIDCache.SetRequestCtx(requestID, reqCtx)
	assert.NoError(t, err, "Should be able to serialize request context with logger")

	// Test deserialization by getting from cache
	retrievedCtx, err := profileIDCache.GetRequestCtx(requestID)
	require.NoError(t, err, "Should be able to deserialize request context")
	require.NotNil(t, retrievedCtx, "Retrieved context should not be nil")

	// Verify the basic fields are preserved
	assert.Equal(t, "test-profile", retrievedCtx.ProfileId, "Profile ID should be preserved")
	assert.Equal(t, map[string]string{"privacy": "setting"}, retrievedCtx.PrivacySettings, "Privacy settings should be preserved")
	assert.Equal(t, map[string]string{"dnssec": "enabled"}, retrievedCtx.DNSSECSettings, "DNSSEC settings should be preserved")
	assert.Equal(t, map[string]string{"advanced": "setting"}, retrievedCtx.AdvancedSettings, "Advanced settings should be preserved")
	assert.Equal(t, "knot", retrievedCtx.UpstreamName, "UpstreamName must survive the cache round-trip (needed by the DNSSEC-failure probe)")

	// Verify the logger is recreated correctly
	require.NotNil(t, retrievedCtx.Logger, "Logger should be recreated")
	assert.False(t, retrievedCtx.Logger.IsEnabled(), "Recreated logger should be disabled")
	assert.Equal(t, "test-profile", retrievedCtx.LoggerConfig.ProfileID, "Logger config should be preserved")
	assert.False(t, retrievedCtx.LoggerConfig.Enabled, "Logger config enabled flag should be preserved")

	// Test that the recreated logger behaves correctly (no logs when disabled)
	// This should not panic and should not log anything
	assert.NotPanics(t, func() {
		retrievedCtx.Logger.Info().Str("test", "value").Msg("This should not log")
		retrievedCtx.Logger.Error().Str("error", "test").Msg("This should not log")
	}, "Recreated disabled logger should not panic")
}

func TestRequestContextSerializationWithEnabledLogger(t *testing.T) {
	// Create a test cache
	cacheCfg := &cache.Config{}
	profileIDCache, err := NewBigcache(cacheCfg)
	require.NoError(t, err)

	// Create a logger factory and enabled logger
	factory := logging.NewFactory(zerolog.DebugLevel)
	logger := factory.ForProfile("enabled-profile", true) // enabled logger

	// Create a request context with the enabled logger
	reqCtx := requestcontext.NewRequestContext(
		context.Background(),
		nil,
		"enabled-profile",
		"", // deviceId
		map[string]string{"privacy": "setting"},
		map[string]string{"logs": "setting"},
		map[string]string{"dnssec": "enabled"},
		map[string]string{"enabled": "false"},
		map[string]string{"advanced": "setting"},
		logger,
	)

	// Verify the logger is set correctly
	assert.True(t, reqCtx.Logger.IsEnabled(), "Logger should be enabled")
	assert.Equal(t, "enabled-profile", reqCtx.LoggerConfig.ProfileID, "Logger config should have correct profile ID")
	assert.True(t, reqCtx.LoggerConfig.Enabled, "Logger config should show enabled=true")

	// Test serialization by setting in cache
	requestID := "test-request-enabled-456"
	err = profileIDCache.SetRequestCtx(requestID, reqCtx)
	assert.NoError(t, err, "Should be able to serialize request context with enabled logger")

	// Test deserialization by getting from cache
	retrievedCtx, err := profileIDCache.GetRequestCtx(requestID)
	require.NoError(t, err, "Should be able to deserialize request context")
	require.NotNil(t, retrievedCtx, "Retrieved context should not be nil")

	// Verify the logger is recreated correctly
	require.NotNil(t, retrievedCtx.Logger, "Logger should be recreated")
	assert.True(t, retrievedCtx.Logger.IsEnabled(), "Recreated logger should be enabled")
	assert.Equal(t, "enabled-profile", retrievedCtx.LoggerConfig.ProfileID, "Logger config should be preserved")
	assert.True(t, retrievedCtx.LoggerConfig.Enabled, "Logger config enabled flag should be preserved")

	// Test that the recreated logger behaves correctly (logs when enabled)
	assert.NotPanics(t, func() {
		retrievedCtx.Logger.Info().Str("test", "value").Msg("This should log when enabled")
	}, "Recreated enabled logger should not panic")
}

func TestRequestContextCacheMiss(t *testing.T) {
	// Create a test cache
	cacheCfg := &cache.Config{}
	profileIDCache, err := NewBigcache(cacheCfg)
	require.NoError(t, err)

	// Try to get a non-existent request context
	retrievedCtx, err := profileIDCache.GetRequestCtx("non-existent-request")
	assert.NoError(t, err, "Cache miss should not return an error")
	assert.Nil(t, retrievedCtx, "Non-existent request context should return nil")
}
