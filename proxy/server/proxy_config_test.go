package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/dnssec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSelfSignedCert generates a throwaway certificate/key pair on disk so
// newTLSConfig can load it.
func writeSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "proxy-config-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}

// newTestServerConfig is the minimal config newProxyConfig accepts, with the
// given DoH settings.
func newTestServerConfig(t *testing.T, doh *config.DoHConfig) *config.Config {
	t.Helper()
	certPath, keyPath := writeSelfSignedCert(t)

	return &config.Config{
		Server: &config.ServerConfig{},
		Upstream: &config.UpstreamConfig{
			Upstreams: map[string]string{"test": "127.0.0.1:5399"},
			Default:   "test",
		},
		DNSCache:       &config.DNSCacheConfig{},
		TLS:            &config.TLSConfig{CertPaths: []string{certPath}, KeyPaths: []string{keyPath}},
		PlainDNS:       &config.PlainDNSConfig{},
		DoH:            doh,
		DoT:            &config.DoTConfig{},
		DoQ:            &config.DoQConfig{},
		TrustedProxies: []string{"10.5.0.0/16"},
	}
}

// specRef: proxy-request-admission-behaviour.md #Q2 #Q10
func TestNewProxyConfig_ConcurrencyAndAnyLimits(t *testing.T) {
	s := &Server{
		Upstreams: map[string]*proxy.CustomUpstreamConfig{},
		edeStore:  &dnssec.EDEStore{},
	}
	serverConfig := newTestServerConfig(t, &config.DoHConfig{ListenAddr: 443})
	serverConfig.Server.MaxGoroutines = 1234

	conf, err := s.newProxyConfig(serverConfig)
	require.NoError(t, err)

	assert.Equal(t, uint(1234), conf.MaxGoroutines)
	assert.True(t, conf.RefuseAny)
	assert.Same(t, s, conf.RequestHandler)

	// DoH must run with request read/write timeouts; without them idle
	// keep-alive connections would be held open indefinitely.
	require.NotNil(t, conf.HTTPConfig)
	require.Len(t, conf.HTTPConfig.ListenAddresses, 1)
	assert.EqualValues(t, 443, conf.HTTPConfig.ListenAddresses[0].Port())
	assert.Equal(t, dohTimeout, conf.HTTPConfig.ReadTimeout)
	assert.Equal(t, dohTimeout, conf.HTTPConfig.WriteTimeout)
}

// specRef: proxy-request-admission-behaviour.md #Q10
func TestNewProxyConfig_NoDoHListener(t *testing.T) {
	s := &Server{
		Upstreams: map[string]*proxy.CustomUpstreamConfig{},
		edeStore:  &dnssec.EDEStore{},
	}
	serverConfig := newTestServerConfig(t, &config.DoHConfig{})

	conf, err := s.newProxyConfig(serverConfig)
	require.NoError(t, err)
	assert.Nil(t, conf.HTTPConfig, "no DoH port configured must not enable the DoH server")
}
