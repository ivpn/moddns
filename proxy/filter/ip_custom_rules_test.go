package filter

import (
	"errors"
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

// buildDNSResponse creates a dns.Msg response with the given A and AAAA answer records.
func buildDNSResponse(domain string, ipv4s []string, ipv6s []string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.Response = true
	for _, ip := range ipv4s {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP(ip),
		})
	}
	for _, ip := range ipv6s {
		msg.Answer = append(msg.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP(ip),
		})
	}
	return msg
}

func TestIPFilterCustomRules(t *testing.T) {
	tests := []struct {
		name             string
		profileID        string
		customRuleHashes []string
		customRules      map[string]map[string]string
		response         *dns.Msg // nil means no response
		expectedResult   *model.StageResult
		wantErr          bool
	}{
		{
			name:             "Block matching IPv4",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Allow matching IPv4",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_ALLOW, "value": "1.2.3.4", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{
				Decision: model.DecisionAllow,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Block matching IPv6",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "2001:db8::1", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", nil, []string{"2001:db8::1"}),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Allow matching IPv6",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_ALLOW, "value": "2001:db8::1", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", nil, []string{"2001:db8::1"}),
			expectedResult: &model.StageResult{
				Decision: model.DecisionAllow,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "No match - different IPv4",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response:       buildDNSResponse("example.com", []string{"5.6.7.8"}, nil),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "No match - different IPv6",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "2001:db8::1", "syntax": "ip"},
			},
			response:       buildDNSResponse("example.com", nil, []string{"2001:db8::2"}),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Nil response - no answers to filter",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response:       nil,
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Syntax missing - old rule skipped",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4"},
			},
			response:       buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Syntax empty string - old rule skipped",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": ""},
			},
			response:       buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Syntax is domain - IP rule skipped",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "domain"},
			},
			response:       buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Syntax contains ip among others",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "domain,ip"},
			},
			response: buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Empty hashes list - no rules",
			profileID:        "test-profile",
			customRuleHashes: []string{},
			customRules:      map[string]map[string]string{},
			response:         buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult:   &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
		{
			name:             "Multiple rules - block wins",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1", "hash2"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_ALLOW, "value": "9.9.9.9", "syntax": "ip"},
				"hash2": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Multiple answer records - one matches",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", []string{"5.6.7.8", "1.2.3.4"}, nil),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Mixed A and AAAA - IPv6 rule matches AAAA",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "2001:db8::1", "syntax": "ip"},
			},
			response: buildDNSResponse("example.com", []string{"1.2.3.4"}, []string{"2001:db8::1"}),
			expectedResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
		},
		{
			name:             "Response with no answer records",
			profileID:        "test-profile",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {"action": ACTION_BLOCK, "value": "1.2.3.4", "syntax": "ip"},
			},
			response:       buildDNSResponse("example.com", nil, nil),
			expectedResult: &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules},
		},
	}

	loggerFactory := logging.NewFactory(zerolog.Disabled)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)

			mockCache.On("GetCustomRulesHashes", mock.Anything, tt.profileID).
				Return(tt.customRuleHashes, nil)
			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).
					Return(rule, nil).Maybe()
			}

			fm := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil)

			reqCtx := &requestcontext.RequestContext{
				ProfileId: tt.profileID,
				Logger:    loggerFactory.ForProfile(tt.profileID, true),
			}

			msg := new(dns.Msg)
			msg.SetQuestion("example.com.", dns.TypeA)
			dctx := &proxy.DNSContext{
				Req: msg,
				Res: tt.response,
			}

			got, err := fm.filterCustomRules(reqCtx, dctx)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tt.expectedResult, got)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestIPFilterCustomRules_CacheErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.Cache)
	}{
		{
			name: "GetCustomRulesHashes returns error",
			setupMock: func(m *mocks.Cache) {
				m.On("GetCustomRulesHashes", mock.Anything, "test-profile").
					Return([]string(nil), errors.New("redis connection refused"))
			},
		},
		{
			name: "GetCustomRulesHash returns error",
			setupMock: func(m *mocks.Cache) {
				m.On("GetCustomRulesHashes", mock.Anything, "test-profile").
					Return([]string{"hash1"}, nil)
				m.On("GetCustomRulesHash", mock.Anything, "hash1").
					Return(map[string]string(nil), errors.New("redis timeout"))
			},
		},
	}

	loggerFactory := logging.NewFactory(zerolog.Disabled)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			tt.setupMock(mockCache)

			fm := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil)

			reqCtx := &requestcontext.RequestContext{
				ProfileId: "test-profile",
				Logger:    loggerFactory.ForProfile("test-profile", true),
			}

			msg := new(dns.Msg)
			msg.SetQuestion("example.com.", dns.TypeA)
			dctx := &proxy.DNSContext{
				Req: msg,
				Res: buildDNSResponse("example.com", []string{"1.2.3.4"}, nil),
			}

			got, err := fm.filterCustomRules(reqCtx, dctx)

			assert.Error(t, err)
			assert.Nil(t, got)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestMatchIPRule(t *testing.T) {
	tests := []struct {
		name        string
		ip          net.IP
		hash        map[string]string
		expectAllow bool
		expectBlock bool
	}{
		{
			name:        "IPv4 - block match",
			ip:          net.ParseIP("93.184.216.34"),
			hash:        map[string]string{"action": ACTION_BLOCK, "value": "93.184.216.34"},
			expectAllow: false,
			expectBlock: true,
		},
		{
			name:        "IPv4 - allow match",
			ip:          net.ParseIP("93.184.216.34"),
			hash:        map[string]string{"action": ACTION_ALLOW, "value": "93.184.216.34"},
			expectAllow: true,
			expectBlock: false,
		},
		{
			name:        "IPv4 - no match",
			ip:          net.ParseIP("93.184.216.34"),
			hash:        map[string]string{"action": ACTION_BLOCK, "value": "10.0.0.1"},
			expectAllow: false,
			expectBlock: false,
		},
		{
			name:        "IPv6 - block match",
			ip:          net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"),
			hash:        map[string]string{"action": ACTION_BLOCK, "value": "2606:2800:220:1:248:1893:25c8:1946"},
			expectAllow: false,
			expectBlock: true,
		},
		{
			name:        "IPv6 - allow match",
			ip:          net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"),
			hash:        map[string]string{"action": ACTION_ALLOW, "value": "2606:2800:220:1:248:1893:25c8:1946"},
			expectAllow: true,
			expectBlock: false,
		},
		{
			name:        "IPv6 - no match",
			ip:          net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"),
			hash:        map[string]string{"action": ACTION_BLOCK, "value": "::1"},
			expectAllow: false,
			expectBlock: false,
		},
		{
			name:        "nil IP - ignored",
			ip:          nil,
			hash:        map[string]string{"action": ACTION_BLOCK, "value": "1.2.3.4"},
			expectAllow: false,
			expectBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := &IPFilter{}

			allow, block := fm.matchIPRule(tt.ip, tt.hash)

			assert.Equal(t, tt.expectAllow, allow, "allow mismatch")
			assert.Equal(t, tt.expectBlock, block, "block mismatch")
		})
	}
}
