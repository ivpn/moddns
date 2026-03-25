package server

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- helpers ---------------------------------------------------------------------

const testPostResolveProfileID = "test-profile-123"
const testPostResolveDeviceID = "test-device-456"

func newPostResolveReqCtx(status model.Status, logsSettings map[string]string) *requestcontext.RequestContext {
	logger := logging.NewDefaultFactory().ForRequest(logging.LoggingConfig{
		Enabled:      true,
		LogDomains:   true,
		LogClientIPs: true,
	})
	return &requestcontext.RequestContext{
		ProfileId:    testPostResolveProfileID,
		DeviceId:     testPostResolveDeviceID,
		LogsSettings: logsSettings,
		FilterResult: model.FilterResult{Status: status},
		Logger:       logger,
		LoggerConfig: logger.Config(),
	}
}

func newPostResolveDNSContext(qname string, qtype uint16) *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn(qname), qtype)
	res := new(dns.Msg)
	res.SetReply(req)
	return &proxy.DNSContext{
		Req:  req,
		Res:  res,
		Addr: netip.MustParseAddrPort("192.168.1.100:12345"),
	}
}

func newPostResolveServer(t *testing.T, ipFilter *mocks.Filter, cacheMock *mocks.Cache, channels map[string]channel.CollectorChannel) *Server {
	t.Helper()
	return &Server{
		IPFilter:          ipFilter,
		Cache:             cacheMock,
		CollectorChannels: channels,
		LoggerFactory:     logging.NewDefaultFactory(),
		Metrics:           noopMetrics{},
	}
}

func awaitWG(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// setupStatsBackground mocks the async EmitStatistics path (cache lookup + Send)
// and wires wg.Done into the Send call so callers can synchronise.
func setupStatsBackground(cacheMock *mocks.Cache, statsCh *mocks.CollectorChannel, wg *sync.WaitGroup) {
	cacheMock.On("GetProfileStatisticsSettings", mock.Anything, testPostResolveProfileID).
		Return(map[string]string{"enabled": "false"}, nil).Maybe()
	wg.Add(1)
	statsCh.On("Send", mock.Anything).Run(func(_ mock.Arguments) { wg.Done() }).Return(nil).Once()
}

// answerIP extracts the IP from the first Answer RR (A or AAAA).
func answerIP(t *testing.T, rr dns.RR) net.IP {
	t.Helper()
	switch v := rr.(type) {
	case *dns.A:
		return v.A
	case *dns.AAAA:
		return v.AAAA
	default:
		t.Fatalf("unexpected RR type: %T", rr)
		return nil
	}
}

// --- tests -----------------------------------------------------------------------

func TestPostResolve_IPFilterDispatch(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  model.Status
		expectIPFilter bool
	}{
		{
			name:           "StatusProcessed_IPFilterExecuted",
			initialStatus:  model.StatusProcessed,
			expectIPFilter: true,
		},
		{
			name:           "StatusBlocked_IPFilterSkipped",
			initialStatus:  model.StatusBlocked,
			expectIPFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipFilter := mocks.NewFilter(t)
			cacheMock := mocks.NewCache(t)
			statsCh := mocks.NewCollectorChannel(t)
			var wg sync.WaitGroup
			setupStatsBackground(cacheMock, statsCh, &wg)

			s := newPostResolveServer(t, ipFilter, cacheMock, map[string]channel.CollectorChannel{
				model.TYPE_STATISTICS: statsCh,
			})

			reqCtx := newPostResolveReqCtx(tt.initialStatus, nil)
			dctx := newPostResolveDNSContext("example.com", dns.TypeA)

			if tt.expectIPFilter {
				ipFilter.On("Execute", reqCtx, dctx).Return(nil)
			}

			s.postResolve(reqCtx, dctx)
			require.True(t, awaitWG(&wg, time.Second), "background goroutines did not finish")

			if tt.expectIPFilter {
				ipFilter.AssertCalled(t, "Execute", reqCtx, dctx)
			} else {
				ipFilter.AssertNotCalled(t, "Execute")
			}
		})
	}
}

func TestPostResolve_ResponseContent(t *testing.T) {
	tests := []struct {
		name           string
		qtype          uint16
		upstreamRR     string
		ipFilterBlocks bool
		wantIP         string // exact IP; empty means expect IsUnspecified
	}{
		{
			name:           "Processed_PreservesUpstreamAnswer",
			qtype:          dns.TypeA,
			upstreamRR:     "example.com. 300 IN A 1.2.3.4",
			ipFilterBlocks: false,
			wantIP:         "1.2.3.4",
		},
		{
			name:           "IPBlock_A_RewritesToZero",
			qtype:          dns.TypeA,
			upstreamRR:     "malware.example.com. 300 IN A 198.51.100.1",
			ipFilterBlocks: true,
			wantIP:         "",
		},
		{
			name:           "IPBlock_AAAA_RewritesToEmptyV6",
			qtype:          dns.TypeAAAA,
			upstreamRR:     "malware.example.com. 300 IN AAAA 2001:db8::1",
			ipFilterBlocks: true,
			wantIP:         "",
		},
		{
			name:           "IPBlock_HTTPS_ReturnsEmptyAnswer",
			qtype:          dns.TypeHTTPS,
			upstreamRR:     "example.com. 300 IN HTTPS 1 . alpn=h2",
			ipFilterBlocks: true,
			wantIP:         "NODATA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipFilter := mocks.NewFilter(t)
			cacheMock := mocks.NewCache(t)
			statsCh := mocks.NewCollectorChannel(t)
			var wg sync.WaitGroup
			setupStatsBackground(cacheMock, statsCh, &wg)

			s := newPostResolveServer(t, ipFilter, cacheMock, map[string]channel.CollectorChannel{
				model.TYPE_STATISTICS: statsCh,
			})

			reqCtx := newPostResolveReqCtx(model.StatusProcessed, nil)
			dctx := newPostResolveDNSContext("example.com", tt.qtype)
			rr, err := dns.NewRR(tt.upstreamRR)
			require.NoError(t, err)
			dctx.Res.Answer = []dns.RR{rr}

			if tt.ipFilterBlocks {
				ipFilter.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					rCtx := args.Get(0).(*requestcontext.RequestContext)
					rCtx.FilterResult.Status = model.StatusBlocked
					rCtx.FilterResult.Reasons = []string{"ip_blocked"}
				}).Return(nil)
			} else {
				ipFilter.On("Execute", mock.Anything, mock.Anything).Return(nil)
			}

			s.postResolve(reqCtx, dctx)
			require.True(t, awaitWG(&wg, time.Second))

			if tt.wantIP == "NODATA" {
				assert.Empty(t, dctx.Res.Answer, "blocked HTTPS response should have empty answer (NODATA)")
				assert.True(t, dctx.Res.Response, "response flag should be set")
			} else {
				require.Len(t, dctx.Res.Answer, 1)
				ip := answerIP(t, dctx.Res.Answer[0])
				if tt.wantIP == "" {
					assert.True(t, ip.IsUnspecified(), "blocked response should be unspecified, got %s", ip)
				} else {
					assert.Equal(t, tt.wantIP, ip.String())
				}
			}
		})
	}
}

func TestPostResolve_CacheHit_EmitsStats(t *testing.T) {
	ipFilter := mocks.NewFilter(t)
	cacheMock := mocks.NewCache(t)
	statsCh := mocks.NewCollectorChannel(t)

	s := newPostResolveServer(t, ipFilter, cacheMock, map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
	})

	reqCtx := newPostResolveReqCtx(model.StatusProcessed, nil)
	dctx := newPostResolveDNSContext("example.com", dns.TypeA)

	ipFilter.On("Execute", mock.Anything, mock.Anything).Return(nil)
	cacheMock.On("GetProfileStatisticsSettings", mock.Anything, testPostResolveProfileID).
		Return(map[string]string{"enabled": "false"}, nil)

	received := make(chan model.EventStatistics, 1)
	statsCh.On("Send", mock.MatchedBy(func(data any) bool {
		if evt, ok := data.(model.EventStatistics); ok {
			received <- evt
			return true
		}
		return false
	})).Return(nil).Once()

	s.postResolve(reqCtx, dctx)

	select {
	case evt := <-received:
		assert.Equal(t, testPostResolveProfileID, evt.Statistics.ProfileID)
		assert.Equal(t, testPostResolveDeviceID, evt.Statistics.DeviceId)
		assert.Equal(t, 1, evt.Statistics.Queries.Total)
		assert.Equal(t, 0, evt.Statistics.Queries.Blocked)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for statistics event")
	}
}

func TestPostResolve_CacheHit_EmitsQueryLog(t *testing.T) {
	ipFilter := mocks.NewFilter(t)
	cacheMock := mocks.NewCache(t)
	statsCh := mocks.NewCollectorChannel(t)
	logsCh := mocks.NewCollectorChannel(t)

	s := newPostResolveServer(t, ipFilter, cacheMock, map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
		model.TYPE_QUERY_LOGS: logsCh,
	})

	logsSettings := map[string]string{
		"enabled":         "true",
		"log_domains":     "true",
		"log_clients_ips": "true",
		"retention":       "1d",
	}
	reqCtx := newPostResolveReqCtx(model.StatusProcessed, logsSettings)
	dctx := newPostResolveDNSContext("logged.example.com", dns.TypeA)

	ipFilter.On("Execute", mock.Anything, mock.Anything).Return(nil)
	cacheMock.On("GetProfileStatisticsSettings", mock.Anything, testPostResolveProfileID).
		Return(map[string]string{"enabled": "false"}, nil).Maybe()
	statsCh.On("Send", mock.Anything).Return(nil).Maybe()

	received := make(chan model.EventQueryLog, 1)
	logsCh.On("Send", mock.MatchedBy(func(data any) bool {
		if evt, ok := data.(model.EventQueryLog); ok {
			received <- evt
			return true
		}
		return false
	})).Return(nil).Once()

	s.postResolve(reqCtx, dctx)

	select {
	case evt := <-received:
		assert.Equal(t, testPostResolveProfileID, evt.QueryLog.ProfileID)
		assert.Equal(t, testPostResolveDeviceID, evt.QueryLog.DeviceId)
		assert.Equal(t, string(model.StatusProcessed), evt.QueryLog.Status)
		assert.Equal(t, "logged.example.com.", evt.QueryLog.DNSRequest.Domain)
		assert.Equal(t, "A", evt.QueryLog.DNSRequest.QueryType)
		assert.Equal(t, "192.168.1.100", evt.QueryLog.ClientIP)
		assert.Equal(t, model.RetentionOneDay, evt.Metadata.Retention)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query log event")
	}
}
