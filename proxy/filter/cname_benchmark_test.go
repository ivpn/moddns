package filter

import (
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

// Benchmarks for the CNAME uncloaking stage. The cost that matters is the one
// paid on EVERY response: extractCNAMETargets walking the answer section, and
// the filterCNAME early exit for CNAME-free answers. In production, responses
// that do carry CNAMEs additionally pay Redis lookups (blocklists / custom
// rules), which dominate and are not measured here.

var benchTargetsSink []string

func BenchmarkExtractCNAMETargets(b *testing.B) {
	cases := []struct {
		name     string
		response *dns.Msg
	}{
		{"NoCNAME_2Answers", buildCNAMEChainResponse("plain.example.com", dns.TypeA, nil, "1.2.3.4")},
		{"Chain_1Hop", buildCNAMEChainResponse("m.shop.example", dns.TypeA, []string{"t.tracker.net"}, "1.2.3.4")},
		{"Chain_3Hops", buildCNAMEChainResponse("m.shop.example", dns.TypeA, []string{"a.cdn.example", "b.cdn.example", "t.tracker.net"}, "1.2.3.4")},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchTargetsSink = extractCNAMETargets(tc.response.Answer, "m.shop.example.")
			}
		})
	}
}

// BenchmarkFilterCNAME_EarlyExit measures the full stage cost for the common
// case — an answer without CNAME records. No cache is wired: any cache access
// on this path would nil-panic, which doubles as a guard that the early exit
// stays Redis-free (spec F/U1).
func BenchmarkFilterCNAME_EarlyExit(b *testing.B) {
	f := NewIPFilter(&proxy.Proxy{}, nil, nil, nil, nil)
	res := buildCNAMEChainResponse("plain.example.com", dns.TypeA, nil, "1.2.3.4")
	req := new(dns.Msg)
	req.SetQuestion("plain.example.com.", dns.TypeA)
	dnsCtx := &proxy.DNSContext{Req: req, Res: res}
	reqCtx := &requestcontext.RequestContext{ProfileId: "bench"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := f.filterCNAME(reqCtx, dnsCtx)
		if err != nil || result == nil {
			b.Fatal("unexpected filterCNAME result")
		}
	}
}
