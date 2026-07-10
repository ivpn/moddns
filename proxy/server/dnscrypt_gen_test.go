package server

import (
	"fmt"
	"os"
	"testing"

	"github.com/ameshkov/dnscrypt/v2"
)

// TestGenerateDNSCryptKeys is a dev/ops helper (not a real test) that mints a
// DNSCrypt resolver key set and prints a ready-to-paste env block plus the
// derived provider public key and a sample sdns:// stamp.
//
// It is a no-op unless GEN_DNSCRYPT is set, so it stays out of the normal suite:
//
//	GEN_DNSCRYPT=1 go test -run TestGenerateDNSCryptKeys -v ./server
//
// Optional overrides:
//
//	DNSCRYPT_PROVIDER_NAME  provider name (default 2.dnscrypt-cert.dns.moddns.net)
//	DNSCRYPT_STAMP_ADDR     server address baked into the sample stamp (default dns.moddns.net)
//
// The same key set must be deployed to EVERY load-balanced instance (see
// config.DNSCryptConfig) — generate once, share everywhere.
func TestGenerateDNSCryptKeys(t *testing.T) {
	if os.Getenv("GEN_DNSCRYPT") == "" {
		t.Skip("set GEN_DNSCRYPT=1 to generate a DNSCrypt resolver key set")
	}

	providerName := os.Getenv("DNSCRYPT_PROVIDER_NAME")
	if providerName == "" {
		providerName = "2.dnscrypt-cert.dns.moddns.net"
	}
	stampAddr := os.Getenv("DNSCRYPT_STAMP_ADDR")
	if stampAddr == "" {
		stampAddr = "dns.moddns.net"
	}

	rc, err := dnscrypt.GenerateResolverConfig(providerName, nil)
	if err != nil {
		t.Fatalf("GenerateResolverConfig: %v", err)
	}

	// Fail loud if the generated material can't actually produce a cert/stamp.
	if _, err := rc.CreateCert(); err != nil {
		t.Fatalf("CreateCert from generated config: %v", err)
	}
	stamp, err := rc.CreateStamp(stampAddr)
	if err != nil {
		t.Fatalf("CreateStamp: %v", err)
	}

	fmt.Printf(`
# DNSCrypt resolver key set — deploy identically to ALL proxy instances.
# Keep DNSCRYPT_PRIVATE_KEY and DNSCRYPT_RESOLVER_SECRET secret.
DNSCRYPT_PROVIDER_NAME=%s
DNSCRYPT_PRIVATE_KEY=%s
DNSCRYPT_RESOLVER_SECRET=%s
DNSCRYPT_RESOLVER_PUBLIC=%s

# Provider public key (for the client sdns:// stamp, Phase 3): %s
# Sample stamp for %s: %s
`,
		rc.ProviderName,
		rc.PrivateKey,
		rc.ResolverSk,
		rc.ResolverPk,
		rc.PublicKey,
		stampAddr,
		stamp.String(),
	)
}
