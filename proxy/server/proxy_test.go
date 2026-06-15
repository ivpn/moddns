package server

import (
	"net/netip"
	"testing"
)

// TestTrustedProxySet_MatchesV4Mapped verifies the trusted-proxy matcher accepts an IPv4 proxy
// address whether it arrives native or IPv4-mapped (::ffff:…), and still rejects untrusted addrs.
// The listeners are dual-stack (Go binds the unspecified address to "::"), so an IPv4 proxy/peer
// reaches the trusted-proxy check as ::ffff:a.b.c.d and must still match a plain IPv4 CIDR.
func TestTrustedProxySet_MatchesV4Mapped(t *testing.T) {
	set := trustedProxySet([]netip.Prefix{netip.MustParsePrefix("10.5.0.0/16")})

	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"10.5.0.1", true},         // native IPv4
		{"::ffff:10.5.0.1", true},  // IPv4-mapped (dual-stack peer) — must also match
		{"10.6.0.1", false},        // outside the trusted CIDR
		{"::ffff:10.6.0.1", false}, // mapped, outside CIDR
		{"2001:db8::1", false},     // native IPv6, untrusted
	} {
		if got := set.Contains(netip.MustParseAddr(tc.addr)); got != tc.want {
			t.Errorf("Contains(%s) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
