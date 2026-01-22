package filter

import (
	"net"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/libs/servicescatalog"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type staticCatalog struct{ cat *servicescatalog.Catalog }

func (s staticCatalog) Get() (*servicescatalog.Catalog, error) { return s.cat, nil }

type staticASNLookup struct{ asn uint }

func (s staticASNLookup) ASN(_ net.IP) (uint, error) { return s.asn, nil }

func TestIPFilter_ServicesASNBlocking_BlocksWhenASNMatches(t *testing.T) {
	const (
		profileID = "profile-services"
		ipStr     = "1.1.1.1"
		asn       = uint(15169)
	)

	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return([]string{"google"}, nil)
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil)

	cat := &servicescatalog.Catalog{Services: []servicescatalog.Service{{
		ID:   "google",
		Name: "Google",
		ASNs: []uint{asn},
	}}}

	dnsProxy := &proxy.Proxy{}
	ipFilter := NewIPFilter(dnsProxy, mockCache, staticCatalog{cat: cat}, staticASNLookup{asn: asn})

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ipStr)},
	}

	dnsCtx := &proxy.DNSContext{Req: req, Res: res}
	loggerFactory := logging.NewFactory(zerolog.DebugLevel)
	testLogger := loggerFactory.ForProfile(profileID, true)
	reqCtx := &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}

	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status)
	assert.Contains(t, reqCtx.FilterResult.Reasons, REASON_SERVICES)
	assert.Contains(t, reqCtx.FilterResult.Reasons, "service: google")
}

func TestIPFilter_AllowByIPOverridesServicesASNBlocking(t *testing.T) {
	const (
		profileID = "profile-services-allow"
		ipStr     = "1.1.1.1"
		asn       = uint(15169)
	)

	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return([]string{"google"}, nil)

	customRuleHashes := []string{"hash_allow"}
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return(customRuleHashes, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_allow").Return(map[string]string{
		"action": ACTION_ALLOW,
		"value":  ipStr,
		"syntax": "ip4_addr",
	}, nil)

	cat := &servicescatalog.Catalog{Services: []servicescatalog.Service{{
		ID:   "google",
		Name: "Google",
		ASNs: []uint{asn},
	}}}

	dnsProxy := &proxy.Proxy{}
	ipFilter := NewIPFilter(dnsProxy, mockCache, staticCatalog{cat: cat}, staticASNLookup{asn: asn})

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ipStr)},
	}

	dnsCtx := &proxy.DNSContext{Req: req, Res: res}
	loggerFactory := logging.NewFactory(zerolog.DebugLevel)
	testLogger := loggerFactory.ForProfile(profileID, true)
	reqCtx := &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}

	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
}
