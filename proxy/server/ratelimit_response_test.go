package server

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRateLimitServer builds a minimal Server for testing HandleBefore rate-limit
// response modes. The rate limiter is configured with rate=1, burst=1 so the
// second call for the same key is always rejected.
func newRateLimitServer(ipResponse, profileResponse string) *Server {
	return &Server{
		Config: &config.Config{
			Server: &config.ServerConfig{},
			RateLimit: &config.RateLimitConfig{
				PerIPEnabled:       true,
				PerIPRate:          1,
				PerIPBurst:         1,
				PerIPResponse:      ipResponse,
				PerProfileEnabled:  true,
				PerProfileRate:     1,
				PerProfileBurst:    1,
				PerProfileResponse: profileResponse,
			},
		},
		RateLimiter: ratelimit.New(ratelimit.Config{
			PerIPEnabled:    true,
			PerIPRate:       1,
			PerIPBurst:      1,
			PerProfileEnabled: true,
			PerProfileRate:    1,
			PerProfileBurst:   1,
		}, nil),
		Metrics: noopMetrics{},
	}
}

func newDNSContext() *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
	return &proxy.DNSContext{
		Req:   req,
		Addr:  netip.MustParseAddrPort("192.0.2.1:53"),
		Proto: proxy.ProtoUDP,
	}
}

func TestHandleBefore_IPRateLimit_Drop(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseDrop, config.RateLimitResponseRefuse)

	// First request passes (consumes the single token).
	dctx := newDNSContext()
	err := s.HandleBefore(nil, dctx)
	// Fails on profile/clientID extraction — that's fine, we only care about
	// IP rate-limit not firing yet.
	require.NotErrorIs(t, err, errRateLimitedIP)

	// Second request from same IP should be dropped (plain error, no BeforeRequestError).
	dctx2 := newDNSContext()
	err = s.HandleBefore(nil, dctx2)
	require.Error(t, err)

	var befErr *proxy.BeforeRequestError
	assert.False(t, errors.As(err, &befErr), "drop mode should NOT return BeforeRequestError")
	assert.ErrorIs(t, err, errRateLimitedIP)
}

func TestHandleBefore_IPRateLimit_Refuse(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseRefuse, config.RateLimitResponseRefuse)

	// Consume the single token.
	dctx := newDNSContext()
	_ = s.HandleBefore(nil, dctx)

	// Second request should get a REFUSED response.
	dctx2 := newDNSContext()
	err := s.HandleBefore(nil, dctx2)
	require.Error(t, err)

	var befErr *proxy.BeforeRequestError
	require.True(t, errors.As(err, &befErr), "refuse mode should return BeforeRequestError")
	require.NotNil(t, befErr.Response)
	assert.Equal(t, dns.RcodeRefused, befErr.Response.Rcode)
	assert.True(t, befErr.Response.Response, "QR flag must be set")
	assert.ErrorIs(t, err, errRateLimitedIP)
}

func TestRefusedResponse(t *testing.T) {
	s := &Server{}
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("test.example.com"), dns.TypeAAAA)
	req.Id = 0xABCD

	resp := s.refusedResponse(req)

	assert.Equal(t, dns.RcodeRefused, resp.Rcode)
	assert.True(t, resp.Response, "QR flag must be set")
	assert.Equal(t, req.Id, resp.Id, "response ID must match request")
	require.Len(t, resp.Question, 1)
	assert.Equal(t, req.Question[0], resp.Question[0])
}
