package filter

import (
	"net"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIPFilter_BlockWinsOnConflict_CustomRules_ASN(t *testing.T) {
	const profileID = "test-profile-asn"
	const allowASN = uint(15169)

	allowIP := "1.1.1.1"
	blockIP := "2.2.2.2"

	mockCache := new(mocks.Cache)
	customRuleHashes := []string{"hash_allow_asn", "hash_block_asn"}
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return(customRuleHashes, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_allow_asn").Return(map[string]string{
		"action": ACTION_ALLOW,
		"value":  "AS15169",
		"syntax": "asn",
	}, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_block_asn").Return(map[string]string{
		"action": ACTION_BLOCK,
		"value":  "15169",
		"syntax": "asn",
	}, nil)

	mockASN := mocks.NewASNLookup(t)
	mockASN.On("ASN", net.ParseIP(allowIP)).Return(allowASN, nil)
	mockASN.On("ASN", net.ParseIP(blockIP)).Return(allowASN, nil)

	dnsProxy := &proxy.Proxy{}
	ipFilter := NewIPFilter(dnsProxy, mockCache, nil, mockASN, nil)

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(allowIP)},
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(blockIP)},
	}

	dnsCtx := &proxy.DNSContext{Req: req, Res: res}
	loggerFactory := logging.NewFactory(zerolog.DebugLevel)
	testLogger := loggerFactory.ForProfile(profileID, true)
	reqCtx := &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}

	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status)
	assert.Contains(t, reqCtx.FilterResult.Reasons, REASON_CUSTOM_RULES)
}

func TestIPFilter_BlockByASN_CustomRules(t *testing.T) {
	const profileID = "test-profile-asn-block"
	const asn = uint(15169)

	ipStr := "1.1.1.1"

	mockCache := new(mocks.Cache)
	customRuleHashes := []string{"hash_block_asn"}
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return(customRuleHashes, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_block_asn").Return(map[string]string{
		"action": ACTION_BLOCK,
		"value":  "AS15169",
		"syntax": "asn",
	}, nil)

	mockASN := mocks.NewASNLookup(t)
	mockASN.On("ASN", net.ParseIP(ipStr)).Return(asn, nil)

	dnsProxy := &proxy.Proxy{}
	ipFilter := NewIPFilter(dnsProxy, mockCache, nil, mockASN, nil)

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
	assert.Contains(t, reqCtx.FilterResult.Reasons, REASON_CUSTOM_RULES)
}
