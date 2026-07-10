package config

import "testing"

// setMinimalProxyEnv sets the env vars New() requires to construct a Config,
// so a test can focus on the fields under test.
func setMinimalProxyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DNS_UPSTREAMS", "default=127.0.0.1:53")
	t.Setenv("DNS_UPSTREAMS_DEFAULT", "default")
	t.Setenv("EMITTER_SINK_TYPE", "mongodb")
}

func TestNew_DNSCryptConfig_Parsed(t *testing.T) {
	setMinimalProxyEnv(t)
	t.Setenv("DNSCRYPT_UDP_LISTEN_ADDR", "5443")
	t.Setenv("DNSCRYPT_TCP_LISTEN_ADDR", "5443")
	t.Setenv("DNSCRYPT_PROVIDER_NAME", "2.dnscrypt-cert.dns.moddns.net")
	t.Setenv("DNSCRYPT_PRIVATE_KEY", "AABB")
	t.Setenv("DNSCRYPT_RESOLVER_SECRET", "CCDD")
	t.Setenv("DNSCRYPT_RESOLVER_PUBLIC", "EEFF")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if cfg.DNSCrypt == nil {
		t.Fatal("cfg.DNSCrypt = nil, want populated config")
	}
	dc := cfg.DNSCrypt
	if dc.UDPListenAddr != 5443 || dc.TCPListenAddr != 5443 {
		t.Errorf("listen addrs = udp:%d tcp:%d, want 5443/5443", dc.UDPListenAddr, dc.TCPListenAddr)
	}
	if dc.ProviderName != "2.dnscrypt-cert.dns.moddns.net" {
		t.Errorf("ProviderName = %q", dc.ProviderName)
	}
	if dc.PrivateKey != "AABB" || dc.ResolverSk != "CCDD" || dc.ResolverPk != "EEFF" {
		t.Errorf("key material not parsed: %+v", dc)
	}
}

func TestNew_DNSCryptConfig_DisabledByDefault(t *testing.T) {
	setMinimalProxyEnv(t)

	cfg, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if cfg.DNSCrypt == nil {
		t.Fatal("cfg.DNSCrypt = nil, want zero-value config")
	}
	if cfg.DNSCrypt.UDPListenAddr != 0 || cfg.DNSCrypt.TCPListenAddr != 0 {
		t.Errorf("DNSCrypt listeners should be off by default, got udp:%d tcp:%d",
			cfg.DNSCrypt.UDPListenAddr, cfg.DNSCrypt.TCPListenAddr)
	}
}
