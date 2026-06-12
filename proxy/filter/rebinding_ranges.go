package filter

import (
	"net"

	"github.com/ivpn/dns/proxy/config"
)

// mustCIDR parses a CIDR at init time; it panics on a malformed literal, which
// can only happen if a constant below is edited incorrectly.
func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("filter: invalid rebinding CIDR " + s + ": " + err.Error())
	}
	return n
}

// alwaysPrivateRanges are the IP ranges always treated as private/local for DNS
// rebinding protection. A public name resolving into any of these is a rebinding
// attempt. Mirrors unbound's private-address defaults plus loopback/link-local.
// Go's net.IP.IsPrivate() alone is insufficient (it misses 127/8, link-local,
// unspecified, IPv4-mapped IPv6), so the set is explicit.
var alwaysPrivateRanges = []*net.IPNet{
	// IPv4
	mustCIDR("0.0.0.0/8"),      // "this" network / unspecified
	mustCIDR("10.0.0.0/8"),     // RFC1918 private
	mustCIDR("127.0.0.0/8"),    // loopback
	mustCIDR("169.254.0.0/16"), // link-local
	mustCIDR("172.16.0.0/12"),  // RFC1918 private
	mustCIDR("192.168.0.0/16"), // RFC1918 private
	// IPv6
	mustCIDR("::/128"),    // unspecified
	mustCIDR("::1/128"),   // loopback
	mustCIDR("fc00::/7"),  // unique local addresses
	mustCIDR("fe80::/10"), // link-local
}

// cgnatRange is RFC6598 carrier-grade NAT space. Opt-in (default off) because
// many ISPs legitimately return CGNAT addresses to carrier-NAT'd users — this
// matches Hagezi, dnsmasq --stop-dns-rebinding, and unbound private-address defaults.
var cgnatRange = mustCIDR("100.64.0.0/10")

// nat64Range is the well-known NAT64 prefix (RFC6052). Opt-in (default off) to
// avoid breaking legitimate NAT64 networks.
var nat64Range = mustCIDR("64:ff9b::/96")

// isPrivateRebindingIP reports whether ip falls into a range that, when returned for
// a public name, indicates a DNS rebinding attempt. IPv4-mapped IPv6 addresses
// (::ffff:a.b.c.d) are unwrapped to their IPv4 form before checking — otherwise
// a mapped private address would bypass the IPv4 ranges (a gap the Hagezi list has).
func isPrivateRebindingIP(ip net.IP, cfg *config.RebindingConfig) bool {
	if ip == nil {
		return false
	}
	// Unwrap IPv4-mapped IPv6 so ::ffff:192.168.1.1 is checked as 192.168.1.1.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	for _, n := range alwaysPrivateRanges {
		if n.Contains(ip) {
			return true
		}
	}
	if cfg != nil && cfg.BlockCGNAT && cgnatRange.Contains(ip) {
		return true
	}
	if cfg != nil && cfg.BlockNAT64 && nat64Range.Contains(ip) {
		return true
	}
	return false
}
