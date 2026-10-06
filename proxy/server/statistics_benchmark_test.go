package server

import (
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
)

// discardChannel accepts every event so the benchmark measures EmitStatistics
// itself, not the collector.
type discardChannel struct{}

func (discardChannel) Send(any) error        { return nil }
func (discardChannel) Receive() (any, error) { return nil, nil }

// BenchmarkEmitStatistics is the per-query cost of the statistics path.
func BenchmarkEmitStatistics(b *testing.B) {
	s := &Server{CollectorChannels: map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: discardChannel{},
	}}
	reqCtx := newPostResolveReqCtx(model.StatusBlocked, nil)
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)
	dctx.Res.AuthenticatedData = true

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.EmitStatistics(reqCtx, dctx)
	}
}

// BenchmarkEmitStatisticsConsented is the per-query cost when the profile has
// statistics enabled.
func BenchmarkEmitStatisticsConsented(b *testing.B) {
	s := &Server{CollectorChannels: map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: discardChannel{},
	}}
	reqCtx := newPostResolveReqCtx(model.StatusBlocked, nil)
	reqCtx.StatisticsSettings = map[string]string{"enabled": "1", "retention": "90d"}
	reqCtx.FilterResult.Reasons = []string{"blocklists", "blocklist: hagezi_pro", "cname_uncloaking"}
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)
	dctx.Proto = proxy.ProtoHTTPS
	dctx.Res.AuthenticatedData = true

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.EmitStatistics(reqCtx, dctx)
	}
}

// BenchmarkEmitStatisticsSettingsOff is the non-consented path with the settings
// map a real request carries.
func BenchmarkEmitStatisticsSettingsOff(b *testing.B) {
	s := &Server{CollectorChannels: map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: discardChannel{},
	}}
	reqCtx := newPostResolveReqCtx(model.StatusBlocked, nil)
	reqCtx.StatisticsSettings = map[string]string{"enabled": "0"}
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)
	dctx.Res.AuthenticatedData = true

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.EmitStatistics(reqCtx, dctx)
	}
}
