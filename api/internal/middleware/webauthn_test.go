package middleware

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/require"
)

type testWebAuthnUser struct{ creds []webauthn.Credential }

func (u testWebAuthnUser) WebAuthnID() []byte                         { return []byte("account-id") }
func (u testWebAuthnUser) WebAuthnName() string                       { return "user@example.com" }
func (u testWebAuthnUser) WebAuthnDisplayName() string                { return "user@example.com" }
func (u testWebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func newTestWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := NewWebAuthn(config.Config{
		API:    &config.APIConfig{ApiAllowOrigin: "https://app.test.local"},
		Server: &config.ServerConfig{Name: "modDNS Test", FQDN: "test.local"},
	})
	require.NoError(t, err)
	return wa
}

// specRef: F11
func TestNewWebAuthnCeremoniesExpire(t *testing.T) {
	wa := newTestWebAuthn(t)
	user := testWebAuthnUser{creds: []webauthn.Credential{{ID: []byte{0x01}}}}

	_, login, err := wa.BeginLogin(user)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(model.CeremonyTimeout), login.Expires, 5*time.Second)

	_, registration, err := wa.BeginRegistration(user)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(model.CeremonyTimeout), registration.Expires, 5*time.Second)
}
