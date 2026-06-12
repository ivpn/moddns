package filter

import (
	"net"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

// dnsCtxNameA builds an A-record DNS context for an arbitrary question name.
func dnsCtxNameA(t *testing.T, name, ipStr string) *proxy.DNSContext {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion(name, dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ipStr)},
	}
	return &proxy.DNSContext{Req: req, Res: res}
}

// dnsCtxPTR builds a PTR query/response (no A/AAAA records to inspect).
func dnsCtxPTR(t *testing.T) *proxy.DNSContext {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion("1.0.168.192.in-addr.arpa.", dns.TypePTR)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.PTR{Hdr: dns.RR_Header{Name: "1.0.168.192.in-addr.arpa.", Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60}, Ptr: "router.lan."},
	}
	return &proxy.DNSContext{Req: req, Res: res}
}

func defaultRebindingConfig() *config.RebindingConfig {
	return &config.RebindingConfig{
		Enabled:       true,
		BlockCGNAT:    false,
		BlockNAT64:    false,
		AllowSuffixes: []string{".local", ".lan", ".home.arpa", ".internal"},
	}
}

// TestFilterRebinding covers the DNS rebinding protection IP sub-filter (TierRebinding,
// T150). Rows trace to docs/specs/proxy-filtering-behaviour.md "DNS rebinding
// protection" decision table.
func TestFilterRebinding(t *testing.T) {
	enabled := map[string]string{"enabled": "true"}

	tests := []struct {
		name     string
		tableRef string
		cfg      *config.RebindingConfig
		settings map[string]string
		dctx     *proxy.DNSContext
		want     model.Decision
	}{
		// Always-private IPv4 ranges → block when opt-in on.
		{"R1 private 10/8 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "10.0.0.5"), model.DecisionBlock},
		{"R1 private 192.168/16 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "192.168.1.1"), model.DecisionBlock},
		{"R1 private 172.16/12 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "172.16.5.5"), model.DecisionBlock},
		{"R1 loopback 127/8 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "127.0.0.1"), model.DecisionBlock},
		{"R1 link-local 169.254/16 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "169.254.1.1"), model.DecisionBlock},
		{"R1 unspecified 0/8 blocked", "R1", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "0.0.0.0"), model.DecisionBlock},

		// Always-private IPv6 ranges.
		{"R2 IPv6 loopback ::1 blocked", "R2", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "::1"), model.DecisionBlock},
		{"R2 IPv6 ULA fc00::/7 blocked", "R2", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "fd00::1"), model.DecisionBlock},
		{"R2 IPv6 link-local fe80::/10 blocked", "R2", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "fe80::1"), model.DecisionBlock},

		// IPv4-mapped IPv6 must be unwrapped and blocked.
		{"R3 IPv4-mapped private blocked", "R3", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "::ffff:192.168.1.1"), model.DecisionBlock},

		// Public IPs pass.
		{"R4 public IPv4 passes", "R4", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "1.1.1.1"), model.DecisionNone},
		{"R4 public IPv6 passes", "R4", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "2606:4700:4700::1111"), model.DecisionNone},

		// Per-profile opt-in OFF (default) → never blocks even for private IP.
		{"R5 opt-in off (empty) passes", "R5", defaultRebindingConfig(), map[string]string{}, dnsCtxWithAAnswer(t, "192.168.1.1"), model.DecisionNone},
		{"R5 opt-in off (false) passes", "R5", defaultRebindingConfig(), map[string]string{"enabled": "false"}, dnsCtxWithAAnswer(t, "192.168.1.1"), model.DecisionNone},

		// Global master switch OFF → never blocks.
		{"R6 master switch off passes", "R6", &config.RebindingConfig{Enabled: false}, enabled, dnsCtxWithAAnswer(t, "192.168.1.1"), model.DecisionNone},
		{"R6 nil config passes", "R6", nil, enabled, dnsCtxWithAAnswer(t, "192.168.1.1"), model.DecisionNone},

		// CGNAT 100.64/10 — opt-in.
		{"R7 CGNAT off (default) passes", "R7", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "100.64.0.1"), model.DecisionNone},
		{"R7 CGNAT on blocks", "R7", &config.RebindingConfig{Enabled: true, BlockCGNAT: true}, enabled, dnsCtxWithAAnswer(t, "100.64.0.1"), model.DecisionBlock},

		// NAT64 64:ff9b::/96 — opt-in.
		{"R8 NAT64 off (default) passes", "R8", defaultRebindingConfig(), enabled, dnsCtxWithAAnswer(t, "64:ff9b::1.2.3.4"), model.DecisionNone},
		{"R8 NAT64 on blocks", "R8", &config.RebindingConfig{Enabled: true, BlockNAT64: true}, enabled, dnsCtxWithAAnswer(t, "64:ff9b::1.2.3.4"), model.DecisionBlock},

		// Operator allow-suffix → private IP allowed for split-horizon names.
		{"R9 .local suffix passes", "R9", defaultRebindingConfig(), enabled, dnsCtxNameA(t, "router.local.", "192.168.1.1"), model.DecisionNone},
		{"R9 .lan suffix passes", "R9", defaultRebindingConfig(), enabled, dnsCtxNameA(t, "nas.lan.", "10.0.0.2"), model.DecisionNone},
		{"R9 non-allowed suffix blocked", "R9", defaultRebindingConfig(), enabled, dnsCtxNameA(t, "evil.com.", "192.168.1.1"), model.DecisionBlock},

		// PTR query — no A/AAAA records → none.
		{"R10 PTR query passes", "R10", defaultRebindingConfig(), enabled, dnsCtxPTR(t), model.DecisionNone},

		// Nil guards.
		{"R11 nil dctx passes", "R11", defaultRebindingConfig(), enabled, nil, model.DecisionNone},
		{"R11 nil Res passes", "R11", defaultRebindingConfig(), enabled, &proxy.DNSContext{Req: new(dns.Msg)}, model.DecisionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCtx := newTestReqCtx(t, "rebinding-test")
			reqCtx.RebindingProtectionSettings = tt.settings
			f := &IPFilter{RebindingConfig: tt.cfg}

			res, err := f.filterRebinding(reqCtx, tt.dctx)
			assert.NoError(t, err, "row %s", tt.tableRef)
			assert.NotNil(t, res, "row %s", tt.tableRef)
			assert.Equal(t, tt.want, res.Decision, "row %s: %s", tt.tableRef, tt.name)
			assert.Equal(t, TierRebinding, res.Tier, "row %s: tier", tt.tableRef)
			if tt.want == model.DecisionBlock {
				assert.Contains(t, res.Reasons, REASON_REBINDING, "row %s: reason", tt.tableRef)
			}
		})
	}
}

// TestFilterRebinding_HTTPSHint verifies private IPs in HTTPS/SVCB ipv4hint are caught.
func TestFilterRebinding_HTTPSHint(t *testing.T) {
	reqCtx := newTestReqCtx(t, "rebinding-https")
	reqCtx.RebindingProtectionSettings = map[string]string{"enabled": "true"}
	f := &IPFilter{RebindingConfig: defaultRebindingConfig()}

	dctx := dnsCtxWithHTTPSAnswer(t, "evil.com.", []net.IP{net.ParseIP("192.168.1.1")}, nil)
	res, err := f.filterRebinding(reqCtx, dctx)
	assert.NoError(t, err)
	assert.Equal(t, model.DecisionBlock, res.Decision)
	assert.Contains(t, res.Reasons, REASON_REBINDING)
}

func TestIsRebindingAllowedSuffix(t *testing.T) {
	suffixes := []string{".local", ".lan", ".home.arpa", ".internal"}
	cases := []struct {
		name string
		want bool
	}{
		{"router.local.", true},
		{"a.b.home.arpa.", true},
		{"local.", true},      // bare label equal to suffix
		{"localhost.", false}, // not a .local suffix
		{"example.com.", false},
		{"notlocal.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isRebindingAllowedSuffix(c.name, suffixes))
		})
	}
}
