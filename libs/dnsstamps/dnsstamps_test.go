package dnsstamps

import (
	"bytes"
	"strings"
	"testing"
)

// helper: round-trip a stamp through encode and decode, returning the decoded form
// or fatally failing the test if either step errored.
func roundTrip(t *testing.T, in ServerStamp) ServerStamp {
	t.Helper()
	encoded := in.String()
	if !strings.HasPrefix(encoded, "sdns://") {
		t.Fatalf("encoded stamp missing sdns:// prefix: %q", encoded)
	}
	out, err := NewServerStampFromString(encoded)
	if err != nil {
		t.Fatalf("decode failed for %q: %v", encoded, err)
	}
	return out
}

func TestRoundTrip_Plain(t *testing.T) {
	// Plain stamps carry only ServerAddrStr; nothing else. Default port 53
	// is stripped by the encoder and re-added by the decoder.
	in := ServerStamp{
		Proto:         StampProtoTypePlain,
		Props:         ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog,
		ServerAddrStr: "198.51.100.10",
	}
	out := roundTrip(t, in)
	if out.Proto != StampProtoTypePlain {
		t.Errorf("Proto = %v, want Plain", out.Proto)
	}
	if out.Props != in.Props {
		t.Errorf("Props = %v, want %v", out.Props, in.Props)
	}
	// Decoder re-adds :53 (the plain DNS default port).
	if out.ServerAddrStr != "198.51.100.10:53" {
		t.Errorf("ServerAddrStr = %q, want 198.51.100.10:53", out.ServerAddrStr)
	}
}

func TestRoundTrip_DoH(t *testing.T) {
	in := ServerStamp{
		Proto:         StampProtoTypeDoH,
		Props:         ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog,
		ServerAddrStr: "1.1.1.1",
		ProviderName:  "dns.example.com",
		Path:          "/dns-query/abc123def4",
	}
	out := roundTrip(t, in)
	if out.Proto != StampProtoTypeDoH {
		t.Errorf("Proto = %v, want DoH", out.Proto)
	}
	if out.ProviderName != in.ProviderName {
		t.Errorf("ProviderName = %q, want %q", out.ProviderName, in.ProviderName)
	}
	if out.Path != in.Path {
		t.Errorf("Path = %q, want %q", out.Path, in.Path)
	}
	// Decoder re-adds :443 for DoH when only IP was given.
	if out.ServerAddrStr != "1.1.1.1:443" {
		t.Errorf("ServerAddrStr = %q, want 1.1.1.1:443", out.ServerAddrStr)
	}
}

func TestRoundTrip_DoH_WithHashes(t *testing.T) {
	hash1 := bytes.Repeat([]byte{0xAB}, 32)
	hash2 := bytes.Repeat([]byte{0xCD}, 32)
	in := ServerStamp{
		Proto:         StampProtoTypeDoH,
		Props:         ServerInformalPropertyDNSSEC,
		ServerAddrStr: "1.1.1.1",
		ProviderName:  "dns.example.com",
		Path:          "/dns-query",
		Hashes:        [][]uint8{hash1, hash2},
	}
	out := roundTrip(t, in)
	if len(out.Hashes) != 2 {
		t.Fatalf("got %d hashes, want 2", len(out.Hashes))
	}
	if !bytes.Equal(out.Hashes[0], hash1) || !bytes.Equal(out.Hashes[1], hash2) {
		t.Errorf("hashes did not round-trip identically")
	}
}

func TestRoundTrip_DoT(t *testing.T) {
	// DoT default port in this library is 843 — explicitly-set 853 must round-trip.
	in := ServerStamp{
		Proto:         StampProtoTypeTLS,
		Props:         ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog,
		ServerAddrStr: "1.1.1.1:853",
		ProviderName:  "dns.example.com",
	}
	out := roundTrip(t, in)
	if out.Proto != StampProtoTypeTLS {
		t.Errorf("Proto = %v, want TLS", out.Proto)
	}
	if out.ServerAddrStr != "1.1.1.1:853" {
		t.Errorf("ServerAddrStr = %q, want 1.1.1.1:853 (explicit port must survive encode)", out.ServerAddrStr)
	}
	if out.ProviderName != in.ProviderName {
		t.Errorf("ProviderName = %q, want %q", out.ProviderName, in.ProviderName)
	}
}

func TestRoundTrip_DoQ(t *testing.T) {
	in := ServerStamp{
		Proto:         StampProtoTypeDoQ,
		Props:         ServerInformalPropertyDNSSEC,
		ServerAddrStr: "1.1.1.1:853",
		ProviderName:  "doq.example.com",
	}
	out := roundTrip(t, in)
	if out.Proto != StampProtoTypeDoQ {
		t.Errorf("Proto = %v, want DoQ", out.Proto)
	}
	if out.ServerAddrStr != "1.1.1.1:853" {
		t.Errorf("ServerAddrStr = %q, want 1.1.1.1:853", out.ServerAddrStr)
	}
}

func TestRoundTrip_DNSCrypt(t *testing.T) {
	pk := bytes.Repeat([]byte{0x42}, 32) // Ed25519-shaped public key (32 bytes)
	in := ServerStamp{
		Proto:         StampProtoTypeDNSCrypt,
		Props:         ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog,
		ServerAddrStr: "1.1.1.1",
		ServerPk:      pk,
		ProviderName:  "2.dnscrypt-cert.example.com",
	}
	out := roundTrip(t, in)
	if out.Proto != StampProtoTypeDNSCrypt {
		t.Errorf("Proto = %v, want DNSCrypt", out.Proto)
	}
	if !bytes.Equal(out.ServerPk, pk) {
		t.Errorf("ServerPk did not round-trip")
	}
	if out.ProviderName != in.ProviderName {
		t.Errorf("ProviderName = %q, want %q", out.ProviderName, in.ProviderName)
	}
}

func TestPropsBitmap_AllCombinations(t *testing.T) {
	combos := []ServerInformalProperties{
		0,
		ServerInformalPropertyDNSSEC,
		ServerInformalPropertyNoLog,
		ServerInformalPropertyNoFilter,
		ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog,
		ServerInformalPropertyDNSSEC | ServerInformalPropertyNoFilter,
		ServerInformalPropertyNoLog | ServerInformalPropertyNoFilter,
		ServerInformalPropertyDNSSEC | ServerInformalPropertyNoLog | ServerInformalPropertyNoFilter,
	}
	for _, props := range combos {
		in := ServerStamp{
			Proto:         StampProtoTypeDoH,
			Props:         props,
			ServerAddrStr: "1.1.1.1",
			ProviderName:  "x",
			Path:          "/",
		}
		out := roundTrip(t, in)
		if out.Props != props {
			t.Errorf("Props=%d did not round-trip (got %d)", props, out.Props)
		}
	}
}

func TestRejectMalformed(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"missing scheme", "AwMAAAAAAAAAAAA"},
		{"unknown protocol", "sdns://fwAAAAAAAAAAAA"},
		{"too short", "sdns://"},
		{"invalid base64", "sdns://!!!"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewServerStampFromString(c.in)
			if err == nil {
				t.Errorf("expected error for %q, got nil", c.in)
			}
		})
	}
}

// TestPortDefaultStripping pins the (quirky) default-port behavior in the
// library so a future change to defaultDoTPort / defaultDoQPort / defaultDoHPort
// constants here fails the test rather than silently breaking the explicit-port
// workaround in api/service/dnsstamp/service.go.
//
// Note: defaults in this library are DoT=843, DoQ=784, DoH=443, Plain=53.
// These differ from real-world conventions (DoT=853, DoQ=853) and that is the
// whole reason api/service/dnsstamp emits explicit :853 ports in ServerAddrStr.
func TestPortDefaultStripping(t *testing.T) {
	cases := []struct {
		name     string
		proto    StampProtoType
		addr     string
		expected string // ServerAddrStr after round-trip
	}{
		{"DoH default 443 re-added when omitted", StampProtoTypeDoH, "1.1.1.1", "1.1.1.1:443"},
		{"DoT default 843 re-added when omitted", StampProtoTypeTLS, "1.1.1.1", "1.1.1.1:843"},
		{"DoT explicit 853 preserved", StampProtoTypeTLS, "1.1.1.1:853", "1.1.1.1:853"},
		{"DoQ default 784 re-added when omitted", StampProtoTypeDoQ, "1.1.1.1", "1.1.1.1:784"},
		{"DoQ explicit 853 preserved", StampProtoTypeDoQ, "1.1.1.1:853", "1.1.1.1:853"},
		{"Plain default 53 re-added when omitted", StampProtoTypePlain, "1.1.1.1", "1.1.1.1:53"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// ProviderName must be non-trivial — the library's decoder rejects
			// stamps shorter than 22 bytes (DoT/DoQ) or 17 bytes (Plain), and
			// we need a realistic provider name for the encoded form to exceed
			// that threshold.
			in := ServerStamp{
				Proto:         c.proto,
				ServerAddrStr: c.addr,
				ProviderName:  "dns.example.com",
				Path:          "/dns-query",
			}
			out := roundTrip(t, in)
			if out.ServerAddrStr != c.expected {
				t.Errorf("got %q, want %q", out.ServerAddrStr, c.expected)
			}
		})
	}
}
