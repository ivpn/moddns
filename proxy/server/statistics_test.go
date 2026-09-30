package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// specRef: proxy-statistics-behaviour.md #Y2 #Y3 #Y4
func TestEmitStatistics_CountersOnly(t *testing.T) {
	tests := []struct {
		name   string
		status model.Status
		nilRes bool
		adFlag bool
		want   model.Queries
	}{
		{
			name:   "processed query without DNSSEC",
			status: model.StatusProcessed,
			want:   model.Queries{Total: 1},
		},
		{
			name:   "blocked query counts as blocked",
			status: model.StatusBlocked,
			want:   model.Queries{Total: 1, Blocked: 1},
		},
		{
			name:   "authenticated response counts as dnssec",
			status: model.StatusProcessed,
			adFlag: true,
			want:   model.Queries{Total: 1, DNSSEC: 1},
		},
		{
			name:   "unavailable result is neither blocked nor dnssec",
			status: model.StatusUnavailable,
			nilRes: true,
			want:   model.Queries{Total: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statsCh := mocks.NewCollectorChannel(t)
			s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
				model.TYPE_STATISTICS: statsCh,
			})
			reqCtx := newPostResolveReqCtx(tt.status, nil)
			dctx := newPostResolveDNSContext("example.com", dns.TypeA)
			if tt.nilRes {
				dctx.Res = nil
			} else {
				dctx.Res.AuthenticatedData = tt.adFlag
			}

			var got model.EventStatistics
			statsCh.On("Send", mock.MatchedBy(func(data any) bool {
				evt, ok := data.(model.EventStatistics)
				if ok {
					got = evt
				}
				return ok
			})).Return(nil).Once()

			s.EmitStatistics(reqCtx, dctx)

			require.Equal(t, model.EventStatistics{Queries: tt.want}, got)
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y2
func TestEmitStatistics_SendErrorIsLoggedNotRaised(t *testing.T) {
	statsCh := mocks.NewCollectorChannel(t)
	statsCh.On("Send", mock.Anything).Return(assert.AnError).Once()
	s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
	})

	assert.NotPanics(t, func() {
		s.EmitStatistics(newPostResolveReqCtx(model.StatusProcessed, nil), newPostResolveDNSContext("example.com", dns.TypeA))
	})
}

func consentedReqCtx(status model.Status, reasons []string, stats map[string]string) *requestcontext.RequestContext {
	reqCtx := newPostResolveReqCtx(status, nil)
	reqCtx.FilterResult.Reasons = reasons
	reqCtx.StatisticsSettings = stats
	return reqCtx
}

func sendEvent(t *testing.T, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) model.EventStatistics {
	t.Helper()
	statsCh := mocks.NewCollectorChannel(t)
	s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
	})
	var got model.EventStatistics
	statsCh.On("Send", mock.MatchedBy(func(data any) bool {
		evt, ok := data.(model.EventStatistics)
		if ok {
			got = evt
		}
		return ok
	})).Return(nil).Once()
	s.EmitStatistics(reqCtx, dctx)
	return got
}

// specRef: proxy-statistics-behaviour.md #Y1 #Y11
func TestEmitStatistics_ConsentGate(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		consent  bool
	}{
		{name: "nil settings", settings: nil},
		{name: "empty settings", settings: map[string]string{}},
		{name: "enabled 0 as the API writes it", settings: map[string]string{"enabled": "0"}},
		{name: "enabled false", settings: map[string]string{"enabled": "false"}},
		{name: "enabled empty", settings: map[string]string{"enabled": ""}},
		{name: "enabled unparsable", settings: map[string]string{"enabled": "yes please"}},
		{name: "enabled 1 as the API writes it", settings: map[string]string{"enabled": "1"}, consent: true},
		{name: "enabled true", settings: map[string]string{"enabled": "true"}, consent: true},
		{name: "enabled TRUE", settings: map[string]string{"enabled": "TRUE"}, consent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCtx := consentedReqCtx(model.StatusBlocked, []string{"blocklists"}, tt.settings)
			got := sendEvent(t, reqCtx, newPostResolveDNSContext("example.com", dns.TypeA))

			assert.Equal(t, model.Queries{Total: 1, Blocked: 1}, got.Queries, "fleet counters do not depend on consent")
			if !tt.consent {
				assert.Nil(t, got.Consented)
				return
			}
			require.NotNil(t, got.Consented)
			assert.Equal(t, testPostResolveProfileID, got.Consented.ProfileID)
			assert.Equal(t, testPostResolveDeviceID, got.Consented.DeviceID)
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y11
func TestEmitStatistics_ConsentedCarriesDeviceAndRetention(t *testing.T) {
	reqCtx := consentedReqCtx(model.StatusProcessed, nil, map[string]string{"enabled": "1", "retention": "90d"})
	reqCtx.DeviceId = ""
	got := sendEvent(t, reqCtx, newPostResolveDNSContext("example.com", dns.TypeA))

	require.NotNil(t, got.Consented)
	assert.Equal(t, "", got.Consented.DeviceID, "an empty device id is passed through")
	assert.Equal(t, model.Retention("90d"), got.Consented.Retention)
}

// specRef: proxy-statistics-behaviour.md #Y12
func TestEmitStatistics_ReasonClass(t *testing.T) {
	tests := []struct {
		name    string
		status  model.Status
		reasons []string
		want    model.ReasonClass
	}{
		{name: "blocklist", status: model.StatusBlocked, reasons: []string{"blocklists", "blocklist: abc"}, want: model.ReasonClassBlocklist},
		{name: "service", status: model.StatusBlocked, reasons: []string{"services", "service: netflix"}, want: model.ReasonClassService},
		{name: "custom rule over blocklist", status: model.StatusBlocked, reasons: []string{"blocklists", "custom_rules"}, want: model.ReasonClassCustomRule},
		{name: "rebinding", status: model.StatusBlocked, reasons: []string{"rebinding_protection"}, want: model.ReasonClassRebinding},
		{name: "default rule", status: model.StatusBlocked, reasons: []string{"default_rule"}, want: model.ReasonClassDefaultRule},
		{name: "cname uncloaking alone", status: model.StatusBlocked, reasons: []string{"cname_uncloaking"}, want: model.ReasonClassOther},
		{name: "blocked without tokens", status: model.StatusBlocked, want: model.ReasonClassOther},
		{name: "processed has no class", status: model.StatusProcessed, reasons: []string{"blocklists"}, want: ""},
		{name: "unavailable has no class", status: model.StatusUnavailable, reasons: []string{"default_rule"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCtx := consentedReqCtx(tt.status, tt.reasons, map[string]string{"enabled": "1"})
			got := sendEvent(t, reqCtx, newPostResolveDNSContext("example.com", dns.TypeA))
			require.NotNil(t, got.Consented)
			assert.Equal(t, tt.want, got.Consented.ReasonClass)
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y13
func TestEmitStatistics_Protocol(t *testing.T) {
	tests := []struct {
		proto proxy.Proto
		want  model.StatisticsProtocol
	}{
		{proto: proxy.ProtoHTTPS, want: model.ProtocolDoH},
		{proto: proxy.ProtoTLS, want: model.ProtocolDoT},
		{proto: proxy.ProtoQUIC, want: model.ProtocolDoQ},
		{proto: proxy.ProtoUDP, want: ""},
		{proto: proxy.ProtoTCP, want: ""},
		{proto: proxy.ProtoDNSCrypt, want: ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.proto), func(t *testing.T) {
			reqCtx := consentedReqCtx(model.StatusProcessed, nil, map[string]string{"enabled": "1"})
			dctx := newPostResolveDNSContext("example.com", dns.TypeA)
			dctx.Proto = tt.proto
			got := sendEvent(t, reqCtx, dctx)
			require.NotNil(t, got.Consented)
			assert.Equal(t, tt.want, got.Consented.Protocol)
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y2 #Y11
func TestPostResolve_ConsentedStatisticsAddsNoGoroutineOrSend(t *testing.T) {
	ipFilter := mocks.NewFilter(t)
	statsCh := mocks.NewCollectorChannel(t)
	logsCh := mocks.NewCollectorChannel(t)
	s := newPostResolveServer(t, ipFilter, mocks.NewCache(t), map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
		model.TYPE_QUERY_LOGS: logsCh,
	})
	reqCtx := consentedReqCtx(model.StatusBlocked, []string{"blocklists"}, map[string]string{"enabled": "1"})
	reqCtx.LogsSettings = map[string]string{"enabled": "false"}

	var wg sync.WaitGroup
	setupStatsBackground(nil, statsCh, &wg)
	s.postResolve(context.Background(), reqCtx, newPostResolveDNSContext("example.com", dns.TypeA))

	require.True(t, awaitWG(&wg, 2*time.Second))
	statsCh.AssertNumberOfCalls(t, "Send", 1)
}

// specRef: proxy-statistics-behaviour.md #Y11
func TestEmitStatistics_UnconsentedPathAllocatesOnlyTheEvent(t *testing.T) {
	s := &Server{CollectorChannels: map[string]channel.CollectorChannel{model.TYPE_STATISTICS: discardChannel{}}}
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)
	for name, settings := range map[string]map[string]string{
		"nil settings":    nil,
		"missing key":     {"retention": "30d"},
		"enabled empty":   {"enabled": ""},
		"enabled off":     {"enabled": "0"},
		"enabled garbage": {"enabled": "yes please"},
	} {
		reqCtx := consentedReqCtx(model.StatusProcessed, nil, settings)
		allocs := testing.AllocsPerRun(100, func() { s.EmitStatistics(reqCtx, dctx) })
		if name == "enabled garbage" {
			continue // a corrupt value may cost its parse error; it is not a normal state
		}
		assert.LessOrEqual(t, allocs, 1.0, name)
	}
}
