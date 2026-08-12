package server

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/cache/memory"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only the fields the context-miss path may touch: the guard must answer
// before any filtering or caching dependency is reached.
func newReqCtxMissServer(t *testing.T, memCache *mocks.MemoryCache) *Server {
	t.Helper()
	return &Server{
		InMemoryCache: memCache,
		LoggerFactory: logging.NewDefaultFactory(),
		Metrics:       noopMetrics{},
	}
}

func newReqCtxMissDNSContext() *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	return &proxy.DNSContext{
		Req:   req,
		Addr:  netip.MustParseAddrPort("192.0.2.1:53"),
		Proto: proxy.ProtoUDP,
	}
}

func requireServFail(t *testing.T, req *dns.Msg, res *dns.Msg) {
	t.Helper()
	require.NotNil(t, res, "a context miss must produce a response, not a dropped query")
	assert.Equal(t, dns.RcodeServerFailure, res.Rcode)
	assert.True(t, res.Response, "QR flag must be set")
	assert.Equal(t, req.Id, res.Id, "response ID must match request")
}

// requestCtxMissReturns enumerates every GetRequestCtx result that leaves the
// handler without a usable context, including the legacy (nil, nil) shape a
// cache implementation could regress to.
var requestCtxMissReturns = []struct {
	name string
	err  error
}{
	{"sentinel miss", memory.ErrRequestCtxNotFound},
	{"nil context with nil error", nil},
	{"cache failure", errors.New("cache unavailable")},
}

// Without a request context the query cannot be filtered, so it must fail
// closed: SERVFAIL, never an unfiltered resolve.
func TestRequestHandlerContextMissAnswersServFail(t *testing.T) {
	for _, tt := range requestCtxMissReturns {
		t.Run(tt.name, func(t *testing.T) {
			memCache := mocks.NewMemoryCache(t)
			memCache.EXPECT().GetRequestCtx("0").Return(nil, tt.err)
			s := newReqCtxMissServer(t, memCache)
			dctx := newReqCtxMissDNSContext()

			err := s.RequestHandler()(nil, dctx)

			require.NoError(t, err)
			requireServFail(t, dctx.Req, dctx.Res)
		})
	}
}

// A resolved answer whose context is gone has not passed IP-phase filtering,
// so it must be replaced with SERVFAIL rather than delivered.
func TestResponseHandlerContextMissAnswersServFail(t *testing.T) {
	for _, tt := range requestCtxMissReturns {
		t.Run(tt.name, func(t *testing.T) {
			memCache := mocks.NewMemoryCache(t)
			memCache.EXPECT().GetRequestCtx("0_response").Return(nil, tt.err)
			s := newReqCtxMissServer(t, memCache)
			dctx := newReqCtxMissDNSContext()

			res := new(dns.Msg)
			res.SetReply(dctx.Req)
			res.Answer = []dns.RR{&dns.A{
				Hdr: dns.RR_Header{Name: dctx.Req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 30},
				A:   net.IPv4(192, 0, 2, 10),
			}}
			dctx.Res = res

			s.ResponseHandler()(dctx, nil)

			requireServFail(t, dctx.Req, dctx.Res)
			assert.Empty(t, dctx.Res.Answer, "upstream answer must not survive a context miss")
		})
	}
}

// The resolve error passed into ResponseHandler must not be dereferenced
// against a missing context.
func TestResponseHandlerContextMissWithResolveError(t *testing.T) {
	memCache := mocks.NewMemoryCache(t)
	memCache.EXPECT().GetRequestCtx("0_response").Return(nil, memory.ErrRequestCtxNotFound)
	s := newReqCtxMissServer(t, memCache)
	dctx := newReqCtxMissDNSContext()

	s.ResponseHandler()(dctx, errors.New("upstream failure"))

	requireServFail(t, dctx.Req, dctx.Res)
}
