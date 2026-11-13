package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newDoHDNSContextWithWriter(path, host string) (*proxy.DNSContext, *httptest.ResponseRecorder) {
	dctx := newDoHDNSContext("")
	dctx.HTTPRequest = &http.Request{URL: &url.URL{Path: path}, Host: host}
	w := httptest.NewRecorder()
	dctx.HTTPResponseWriter = w
	return dctx, w
}

// specRef: proxy-request-admission-behaviour.md #Q5, #Q15
func TestServeDNS_DoHWithoutProfileGetsHTTPError(t *testing.T) {
	cases := []struct {
		name, path, host string
		serverNames      []string
		want             int
	}{
		{name: "no profile in path", path: "/dns-query", want: http.StatusBadRequest},
		{name: "invalid profile ID", path: "/dns-query/not_a_valid_id!", want: http.StatusBadRequest},
		{name: "profile ID shorter than the minimum", path: "/dns-query/abc", want: http.StatusBadRequest},
		{name: "profile ID with a hyphen", path: "/dns-query/abcde-fghijk", want: http.StatusBadRequest},
		{name: "too many path segments", path: "/dns-query/validprofile1/dev/extra", want: http.StatusBadRequest},
		{name: "path other than /dns-query", path: "/foo", want: http.StatusNotFound},
		// Q15: DoH takes the profile from the path only, never from Host.
		{name: "valid profile label in Host", path: "/dns-query", host: "validprofile1.dns.example.net",
			serverNames: []string{"dns.example.net"}, want: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newProfileRateLimitServer(mocks.NewCache(t), config.RateLimitResponseRefuse)
			s.Config.Server.Names = tc.serverNames
			dctx, w := newDoHDNSContextWithWriter(tc.path, tc.host)

			err := s.ServeDNS(context.Background(), nil, dctx)

			require.ErrorIs(t, err, proxy.ErrDrop)
			assert.Nil(t, dctx.Res)
			assert.Equal(t, tc.want, w.Code)
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q6
func TestServeDNS_DoHUnknownProfileGetsBadRequest(t *testing.T) {
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, "unknownprofile1").
		Return(&model.ProfileSettings{PrivacyErr: fmt.Errorf("%w: [privacy]", cache.ErrSettingsNotFound)}, nil)
	s := newProfileRateLimitServer(c, config.RateLimitResponseRefuse)
	dctx, w := newDoHDNSContextWithWriter("/dns-query/unknownprofile1", "")

	err := s.ServeDNS(context.Background(), nil, dctx)

	require.ErrorIs(t, err, proxy.ErrDrop)
	assert.Nil(t, dctx.Res)
	assert.Equal(t, http.StatusBadRequest, w.Code, "same status as a missing profile (Q5)")
}

// specRef: proxy-request-admission-behaviour.md #Q14
func TestServeDNS_DoHDeletedProfileGetsBadRequest(t *testing.T) {
	const profileID = "deletedprofile01"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(goodSettings("allow"), nil).Once()
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(&model.ProfileSettings{PrivacyErr: fmt.Errorf("%w: [privacy]", cache.ErrSettingsNotFound)}, nil).Once()
	_, _, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)

	f.advance(staleTTL + time.Second)
	dctx, w := newDoHDNSContextWithWriter("/dns-query/"+profileID, "")
	err = f.s.ServeDNS(context.Background(), nil, dctx)

	require.ErrorIs(t, err, errProfileIdNotFound)
	assert.Nil(t, dctx.Res)
	assert.Equal(t, http.StatusBadRequest, w.Code, "same status as an unknown profile (Q6)")
}

// specRef: proxy-request-admission-behaviour.md #Q3
func TestServeDNS_DoHIPRateLimitDropGetsTooManyRequests(t *testing.T) {
	s := newRateLimitServer(config.RateLimitResponseDrop, config.RateLimitResponseRefuse)
	first, _ := newDoHDNSContextWithWriter("/dns-query", "")
	_, _, _ = s.prepareRequest(context.Background(), nil, first)

	dctx, w := newDoHDNSContextWithWriter("/dns-query/validprofile1", "")
	err := s.ServeDNS(context.Background(), nil, dctx)

	require.ErrorIs(t, err, errRateLimitedIP)
	assert.Nil(t, dctx.Res)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// specRef: proxy-request-admission-behaviour.md #Q8
func TestServeDNS_DoHProfileRateLimitDropGetsTooManyRequests(t *testing.T) {
	s := newProfileRateLimitServer(mocks.NewCache(t), config.RateLimitResponseDrop)
	seedCachedProfile(s, "validprofile1")
	first, _ := newDoHDNSContextWithWriter("/dns-query/validprofile1", "")
	_, _, err := s.prepareRequest(context.Background(), nil, first)
	require.NoError(t, err)

	dctx, w := newDoHDNSContextWithWriter("/dns-query/validprofile1", "")
	err = s.ServeDNS(context.Background(), nil, dctx)

	require.ErrorIs(t, err, errRateLimitedProfile)
	assert.Nil(t, dctx.Res)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// specRef: proxy-request-admission-behaviour.md #Q5
func TestServeDNS_DoTWithoutProfileIsDropped(t *testing.T) {
	s := newProfileRateLimitServer(mocks.NewCache(t), config.RateLimitResponseRefuse)
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
	dctx := &proxy.DNSContext{
		Req:   req,
		Addr:  netip.MustParseAddrPort("192.0.2.1:853"),
		Proto: proxy.ProtoTLS,
	}

	err := s.ServeDNS(context.Background(), nil, dctx)

	require.ErrorIs(t, err, proxy.ErrDrop)
	assert.Nil(t, dctx.Res)
}
