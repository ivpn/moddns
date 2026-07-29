package filter

import (
	"net"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/requestcontext"
)

// benchRebindingCtx builds a DNS context with `count` A answers, the last of which
// is private when `private` is true (worst case: must scan the whole answer set).
func benchRebindingCtx(count int, private bool) *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	answers := make([]dns.RR, 0, count)
	for i := 0; i < count; i++ {
		ip := net.IPv4(1, 1, 1, byte(i%256))
		if private && i == count-1 {
			ip = net.ParseIP("192.168.1.1")
		}
		answers = append(answers, &dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   ip,
		})
	}
	res.Answer = answers
	return &proxy.DNSContext{Req: req, Res: res}
}

func BenchmarkFilterRebinding(b *testing.B) {
	cases := []struct {
		name    string
		answers int
		private bool
	}{
		{"NoMatch_1", 1, false},
		{"NoMatch_10", 10, false},
		{"Match_1", 1, true},
		{"Match_10", 10, true},
	}

	loggerFactory := logging.NewFactory(zerolog.Disabled)
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			f := &IPFilter{RebindingConfig: defaultRebindingConfig()}
			reqCtx := &requestcontext.RequestContext{
				ProfileId:                   "bench",
				Logger:                      loggerFactory.ForProfile("bench", false),
				RebindingProtectionSettings: map[string]string{"enabled": "1"},
			}
			dctx := benchRebindingCtx(tc.answers, tc.private)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res, err := f.filterRebinding(reqCtx, dctx)
				require.NoError(b, err)
				require.NotNil(b, res)
			}
		})
	}
}
