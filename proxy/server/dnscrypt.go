package server

import (
	"crypto/ed25519"
	"fmt"

	"github.com/ameshkov/dnscrypt/v2"
	"github.com/ivpn/dns/proxy/config"
)

// newDNSCryptCert builds the signed resolver certificate for the DNSCrypt
// server listener from the configured key material.
//
// The certificate is (re)generated at startup from the persistent keys in cfg,
// so every instance booting with the same env produces an equivalent cert:
// signed by the same long-term key and advertising the same short-term
// ResolverPk that all instances hold the ResolverSk for. This is what makes the
// listener safe behind an anycast/load-balanced address.
func newDNSCryptCert(cfg *config.DNSCryptConfig) (*dnscrypt.Cert, error) {
	rc := dnscrypt.ResolverConfig{
		ProviderName: cfg.ProviderName,
		PrivateKey:   cfg.PrivateKey,
		ResolverSk:   cfg.ResolverSk,
		ResolverPk:   cfg.ResolverPk,
		EsVersion:    dnscrypt.XSalsa20Poly1305,
	}
	return rc.CreateCert()
}

// dnsCryptProviderPublicKey derives the long-term provider public key (hex) from
// the configured signing key. This is the value that goes into the DNSCrypt
// sdns:// stamp handed to clients (Phase 3), so we log it at startup. It is a
// public key — safe to log.
func dnsCryptProviderPublicKey(cfg *config.DNSCryptConfig) (string, error) {
	priv, err := dnscrypt.HexDecodeKey(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decoding dnscrypt private key: %w", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("dnscrypt private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(priv))
	}
	pub := ed25519.PrivateKey(priv).Public().(ed25519.PublicKey)
	return dnscrypt.HexEncodeKey(pub), nil
}
