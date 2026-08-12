package server

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newRateLimitServer builds a minimal Server for testing prepareRequest
// rate-limit response modes. The rate limiter is configured with rate=1,
// burst=1 so the second call for the same key is always rejected.
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
			PerIPEnabled:      true,
			PerIPRate:         1,
			PerIPBurst:        1,
			PerProfileEnabled: true,
			PerProfileRate:    1,
			PerProfileBurst:   1,
		}, nil),
		LoggerFactory: logging.NewDefaultFactory(),
		Metrics:       noopMetrics{},
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

func TestPrepareRequest_IPRateLimit_Drop(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseDrop, config.RateLimitResponseRefuse)

	// First request passes (consumes the single token).
	_, _, err := s.prepareRequest(context.Background(), nil, newDNSContext())
	// Fails on profile/clientID extraction — that's fine, we only care about
	// IP rate-limit not firing yet.
	require.NotErrorIs(t, err, errRateLimitedIP)

	// Second request from same IP should be dropped: an error, no response.
	_, errResp, err := s.prepareRequest(context.Background(), nil, newDNSContext())
	require.ErrorIs(t, err, errRateLimitedIP)
	assert.Nil(t, errResp, "drop mode must not build a response")
}

func TestPrepareRequest_IPRateLimit_Refuse(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseRefuse, config.RateLimitResponseRefuse)

	// Consume the single token.
	_, _, _ = s.prepareRequest(context.Background(), nil, newDNSContext())

	// Second request should get a REFUSED response and no error.
	dctx := newDNSContext()
	_, errResp, err := s.prepareRequest(context.Background(), nil, dctx)
	require.NoError(t, err)
	require.NotNil(t, errResp)
	assert.Equal(t, dns.RcodeRefused, errResp.Rcode)
	assert.True(t, errResp.Response, "QR flag must be set")
}

// ServeDNS must translate a prepareRequest drop into [proxy.ErrDrop] so the
// vendor proxy sends nothing, while preserving the cause for logging.
func TestServeDNS_DropReturnsErrDrop(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseDrop, config.RateLimitResponseRefuse)

	_, _, _ = s.prepareRequest(context.Background(), nil, newDNSContext())

	dctx := newDNSContext()
	err := s.ServeDNS(context.Background(), nil, dctx)
	require.ErrorIs(t, err, proxy.ErrDrop)
	require.ErrorIs(t, err, errRateLimitedIP)
	assert.Nil(t, dctx.Res, "dropped requests must not carry a response")
}

// ServeDNS must answer refuse-mode rejections itself and report no error.
func TestServeDNS_RefuseAnswersRefused(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseRefuse, config.RateLimitResponseRefuse)

	_, _, _ = s.prepareRequest(context.Background(), nil, newDNSContext())

	dctx := newDNSContext()
	err := s.ServeDNS(context.Background(), nil, dctx)
	require.NoError(t, err)
	require.NotNil(t, dctx.Res)
	assert.Equal(t, dns.RcodeRefused, dctx.Res.Rcode)
}

// newProfileRateLimitServer builds a Server that reaches the per-profile
// rate-limit layer: per-IP limiting is disabled and the profile limiter is
// rate=1, burst=1 so the second call for the same profile is rejected.
func newProfileRateLimitServer(c *mocks.Cache) *Server {
	return &Server{
		Config: &config.Config{
			Server:   &config.ServerConfig{},
			Upstream: &config.UpstreamConfig{Default: "default"},
			RateLimit: &config.RateLimitConfig{
				PerProfileEnabled:  true,
				PerProfileRate:     1,
				PerProfileBurst:    1,
				PerProfileResponse: config.RateLimitResponseRefuse,
			},
		},
		Cache:                c,
		ProfileSettingsCache: gocache.New(time.Minute, time.Minute),
		LoggerFactory:        logging.NewDefaultFactory(),
		RateLimiter: ratelimit.New(ratelimit.Config{
			PerProfileEnabled: true,
			PerProfileRate:    1,
			PerProfileBurst:   1,
		}, nil),
		Metrics: noopMetrics{},
	}
}

// newDoHDNSContext carries profileID via the DoH path, the simplest route
// through clientIDFromDNSContext in tests.
func newDoHDNSContext(profileID string) *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
	return &proxy.DNSContext{
		Req:         req,
		Addr:        netip.MustParseAddrPort("192.0.2.1:443"),
		Proto:       proxy.ProtoHTTPS,
		HTTPRequest: &http.Request{URL: &url.URL{Path: "/dns-query/" + profileID}},
	}
}

func TestPrepareRequest_UnknownProfileNeverProfileRateLimited(t *testing.T) {
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, "unknownprofile1").
		Return(&model.ProfileSettings{PrivacyErr: errors.New("no [privacy] settings found for profile")}, nil)
	s := newProfileRateLimitServer(c)

	// Far past the burst of 1: every call must fail on existence, and the
	// profile rate limiter must never fire for a profile that does not exist.
	for i := range 5 {
		_, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext("unknownprofile1"))
		require.ErrorIs(t, err, errProfileIdNotFound, "call %d", i)
		require.NotErrorIs(t, err, errRateLimitedProfile, "call %d", i)
		require.Nil(t, errResp, "call %d", i)
	}
}

func TestPrepareRequest_ValidProfileRateLimit_Refuse(t *testing.T) {
	s := newProfileRateLimitServer(mocks.NewCache(t))

	fetchErr := errors.New("settings unavailable")
	s.ProfileSettingsCache.Set("validprofile1", &model.ProfileSettings{
		Privacy:                map[string]string{},
		LogsErr:                fetchErr,
		DNSSECErr:              fetchErr,
		RebindingProtectionErr: fetchErr,
		AdvancedErr:            fetchErr,
	}, gocache.DefaultExpiration)

	// First request consumes the single token.
	reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext("validprofile1"))
	require.NoError(t, err)
	require.Nil(t, errResp)
	require.NotNil(t, reqCtx)

	// Second request must be refused by the profile layer.
	_, errResp, err = s.prepareRequest(context.Background(), nil, newDoHDNSContext("validprofile1"))
	require.NoError(t, err)
	require.NotNil(t, errResp)
	assert.Equal(t, dns.RcodeRefused, errResp.Rcode)
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
