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

func TestIPFilter_AllowByIPShouldOverrideBlock_CustomRules(t *testing.T) {
	const profileID = "test-profile"

	allowIP := "1.1.1.1"
	blockIP := "2.2.2.2"

	// Create mock cache
	mockCache := new(mocks.Cache)

	customRuleHashes := []string{"hash_allow", "hash_block"}
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
		Return(customRuleHashes, nil)

	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_allow").
		Return(map[string]string{
			"action": ACTION_ALLOW,
			"value":  allowIP,
			"syntax": "ip4_addr",
		}, nil)

	mockCache.On("GetCustomRulesHash", mock.Anything, "hash_block").
		Return(map[string]string{
			"action": ACTION_BLOCK,
			"value":  blockIP,
			"syntax": "ip4_addr",
		}, nil)

	// Create filter manager with mock cache
	dnsProxy := &proxy.Proxy{}
	ipFilter := NewIPFilter(dnsProxy, mockCache)

	// Create DNS request/response with two A answers.
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(allowIP),
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(blockIP),
		},
	}

	dnsCtx := &proxy.DNSContext{Req: req, Res: res}

	loggerFactory := logging.NewFactory(zerolog.DebugLevel)
	testLogger := loggerFactory.ForProfile(profileID, true)
	reqCtx := &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}

	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)

	// Expected behavior: allow-by-IP should win and final status should be processed.
	// Current behavior: any matched block rule makes the single FilterResult blocked,
	// and the final status becomes blocked.
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
}
