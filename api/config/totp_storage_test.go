package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md E10 - the API does not start without a valid storage key.
func TestTOTPStorageKeyRequired(t *testing.T) {
	for _, v := range []string{"", "not-base64!", "AAAAAAAAAAAAAAAAAAAAAA=="} {
		t.Run(v, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("TOTP_STORAGE_KEY", v)
			_, err := New()
			require.Error(t, err)
			require.Contains(t, err.Error(), "TOTP_STORAGE_KEY")
		})
	}
}

func TestTOTPStorageConfig(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := New()
	require.NoError(t, err)
	require.Len(t, cfg.Service.TOTPStorageKey, 32)
	require.False(t, cfg.Service.MigrateTOTPStorage, "storage migration is opt-in")

	t.Setenv("MIGRATE_TOTP_STORAGE", "true")
	cfg, err = New()
	require.NoError(t, err)
	require.True(t, cfg.Service.MigrateTOTPStorage)
}
