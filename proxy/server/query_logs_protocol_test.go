package server

import (
	"net/http"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// specRef: proxy-request-admission-behaviour.md #Q18
func TestEmitQueryLog_Protocol(t *testing.T) {
	tests := []struct {
		name       string
		proto      proxy.Proto
		protoMajor int
		want       string
	}{
		{name: "doh over http3", proto: proxy.ProtoHTTPS, protoMajor: 3, want: "doh3"},
		{name: "doh over http2", proto: proxy.ProtoHTTPS, protoMajor: 2, want: "https"},
		{name: "doh over http1.1 from nginx", proto: proxy.ProtoHTTPS, protoMajor: 1, want: "https"},
		{name: "doh without request", proto: proxy.ProtoHTTPS, want: "https"},
		{name: "dot", proto: proxy.ProtoTLS, want: "tls"},
		{name: "doq", proto: proxy.ProtoQUIC, want: "quic"},
		{name: "udp", proto: proxy.ProtoUDP, want: "udp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logsCh := mocks.NewCollectorChannel(t)
			s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
				model.TYPE_QUERY_LOGS: logsCh,
			})
			var got model.EventQueryLog
			logsCh.On("Send", mock.MatchedBy(func(data any) bool {
				evt, ok := data.(model.EventQueryLog)
				if ok {
					got = evt
				}
				return ok
			})).Return(nil).Once()

			reqCtx := newPostResolveReqCtx(model.StatusProcessed, map[string]string{"enabled": "true", "retention": "1d"})
			dctx := newPostResolveDNSContext("example.com", dns.TypeA)
			dctx.Proto = tt.proto
			if tt.protoMajor != 0 {
				dctx.HTTPRequest = &http.Request{ProtoMajor: tt.protoMajor}
			}

			s.EmitQueryLog(reqCtx, dctx)

			assert.Equal(t, tt.want, got.QueryLog.Protocol)
		})
	}
}
