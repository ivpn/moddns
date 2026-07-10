package server

import (
	"context"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/ameshkov/dnscrypt/v2"
	"github.com/ivpn/dns/proxy/config"
	"github.com/miekg/dns"
)

// cfgFromResolverConfig mirrors a generated dnscrypt.ResolverConfig into the
// env-derived config.DNSCryptConfig our code consumes.
func cfgFromResolverConfig(rc dnscrypt.ResolverConfig) *config.DNSCryptConfig {
	return &config.DNSCryptConfig{
		ProviderName: rc.ProviderName,
		PrivateKey:   rc.PrivateKey,
		ResolverSk:   rc.ResolverSk,
		ResolverPk:   rc.ResolverPk,
	}
}

func TestNewDNSCryptCert_Valid(t *testing.T) {
	rc, err := dnscrypt.GenerateResolverConfig("2.dnscrypt-cert.test.moddns.net", nil)
	if err != nil {
		t.Fatalf("GenerateResolverConfig: %v", err)
	}

	cert, err := newDNSCryptCert(cfgFromResolverConfig(rc))
	if err != nil {
		t.Fatalf("newDNSCryptCert: %v", err)
	}
	if !cert.VerifyDate() {
		t.Error("cert.VerifyDate() = false, want a currently-valid cert")
	}

	pub, err := hex.DecodeString(rc.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if !cert.VerifySignature(pub) {
		t.Error("cert.VerifySignature() = false, want cert signed by the provider key")
	}
}

func TestNewDNSCryptCert_BadPrivateKey(t *testing.T) {
	cfg := &config.DNSCryptConfig{
		ProviderName: "2.dnscrypt-cert.test.moddns.net",
		PrivateKey:   "not-hex",
	}
	if _, err := newDNSCryptCert(cfg); err == nil {
		t.Fatal("newDNSCryptCert with invalid private key: want error, got nil")
	}
}

func TestDNSCryptProviderPublicKey(t *testing.T) {
	rc, err := dnscrypt.GenerateResolverConfig("2.dnscrypt-cert.test.moddns.net", nil)
	if err != nil {
		t.Fatalf("GenerateResolverConfig: %v", err)
	}

	pub, err := dnsCryptProviderPublicKey(cfgFromResolverConfig(rc))
	if err != nil {
		t.Fatalf("dnsCryptProviderPublicKey: %v", err)
	}
	if pub != rc.PublicKey {
		t.Errorf("derived public key = %s, want %s", pub, rc.PublicKey)
	}

	if _, err := dnsCryptProviderPublicKey(&config.DNSCryptConfig{PrivateKey: "abcd"}); err == nil {
		t.Error("dnsCryptProviderPublicKey with short key: want error, got nil")
	}
}

// TestDNSCryptServer_RoundTrip stands up a real DNSCrypt UDP listener built from
// our newDNSCryptCert output and exchanges an encrypted query with the vendored
// dnscrypt.Client. It proves the cert helper + the proxy.Config DNSCrypt fields
// our newProxyConfig sets actually produce a working listener, and that requests
// enter the shared pipeline tagged with Proto == ProtoDNSCrypt.
func TestDNSCryptServer_RoundTrip(t *testing.T) {
	rc, err := dnscrypt.GenerateResolverConfig("2.dnscrypt-cert.test.moddns.net", nil)
	if err != nil {
		t.Fatalf("GenerateResolverConfig: %v", err)
	}
	cert, err := newDNSCryptCert(cfgFromResolverConfig(rc))
	if err != nil {
		t.Fatalf("newDNSCryptCert: %v", err)
	}

	// Stub upstream — never dialed because the RequestHandler short-circuits.
	ups, err := upstream.AddressToUpstream("127.0.0.1:53", &upstream.Options{})
	if err != nil {
		t.Fatalf("AddressToUpstream: %v", err)
	}

	// Buffered channel publishes the observed proto with a happens-before edge
	// (the handler runs on a listener goroutine).
	protoCh := make(chan proxy.Proto, 1)
	conf := &proxy.Config{
		UpstreamConfig: &proxy.UpstreamConfig{Upstreams: []upstream.Upstream{ups}},
		RequestHandler: func(_ *proxy.Proxy, d *proxy.DNSContext) error {
			select {
			case protoCh <- d.Proto:
			default:
			}
			resp := &dns.Msg{}
			resp.SetReply(d.Req)
			rr, rrErr := dns.NewRR("example.com. 60 IN A 1.2.3.4")
			if rrErr != nil {
				return rrErr
			}
			resp.Answer = []dns.RR{rr}
			d.Res = resp
			return nil
		},
		DNSCryptResolverCert:  cert,
		DNSCryptProviderName:  rc.ProviderName,
		DNSCryptUDPListenAddr: []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: 0}},
	}

	p, err := proxy.New(conf)
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("proxy.Start: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()

	addr := p.Addr(proxy.ProtoDNSCrypt)
	if addr == nil {
		t.Fatal("proxy.Addr(ProtoDNSCrypt) = nil, listener not bound")
	}

	stamp, err := rc.CreateStamp(addr.String())
	if err != nil {
		t.Fatalf("CreateStamp: %v", err)
	}

	client := &dnscrypt.Client{Net: "udp", Timeout: 5 * time.Second}
	ri, err := client.DialStamp(stamp)
	if err != nil {
		t.Fatalf("client.DialStamp: %v", err)
	}

	m := &dns.Msg{}
	m.SetQuestion("example.com.", dns.TypeA)
	resp, err := client.Exchange(m, ri)
	if err != nil {
		t.Fatalf("client.Exchange: %v", err)
	}

	select {
	case gotProto := <-protoCh:
		if gotProto != proxy.ProtoDNSCrypt {
			t.Errorf("handler saw Proto = %q, want %q", gotProto, proxy.ProtoDNSCrypt)
		}
	default:
		t.Error("request handler was never invoked")
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("answer count = %d, want 1", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok || a.A.String() != "1.2.3.4" {
		t.Errorf("answer = %v, want A 1.2.3.4", resp.Answer[0])
	}
}
