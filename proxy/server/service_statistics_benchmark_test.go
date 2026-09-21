package server

import (
	"testing"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
)

// discardChannel accepts every event so the benchmark measures EmitServiceStatistics
// itself, not the collector.
type discardChannel struct{}

func (discardChannel) Send(any) error        { return nil }
func (discardChannel) Receive() (any, error) { return nil, nil }

// BenchmarkEmitServiceStatistics is the per-query cost of the statistics path.
func BenchmarkEmitServiceStatistics(b *testing.B) {
	s := &Server{CollectorChannels: map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: discardChannel{},
	}}
	reqCtx := newPostResolveReqCtx(model.StatusBlocked, nil)
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)
	dctx.Res.AuthenticatedData = true

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.EmitServiceStatistics(reqCtx, dctx)
	}
}
