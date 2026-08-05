package filter

import (
	"bytes"
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
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFilterCustomRules(t *testing.T) {
	tests := []struct {
		name               string
		profileID          string
		domain             string
		customRuleHashes   []string
		customRules        map[string]map[string]string
		expectedFltrResult *model.StageResult
		wantErr            bool
	}{
		{
			name:             "Block domain",
			profileID:        "test-profile",
			domain:           "blocked.example.com",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {
					"action": ACTION_BLOCK,
					"value":  "blocked.example.com",
				},
			},
			expectedFltrResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
			wantErr: false,
		},
		{
			name:             "Allow domain",
			profileID:        "test-profile",
			domain:           "allowed.example.com",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {
					"action": ACTION_ALLOW,
					"value":  "allowed.example.com",
				},
			},
			expectedFltrResult: &model.StageResult{
				Decision: model.DecisionAllow,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
			wantErr: false,
		},
		{
			name:             "No matching rules",
			profileID:        "test-profile",
			domain:           "normal.example.com",
			customRuleHashes: []string{"hash1"},
			customRules: map[string]map[string]string{
				"hash1": {
					"action": ACTION_BLOCK,
					"value":  "blocked.example.com",
				},
			},
			expectedFltrResult: &model.StageResult{
				Decision: model.DecisionNone,
				Tier:     TierCustomRules,
				Reasons:  nil,
			},
			wantErr: false,
		},
		{
			name:             "Multiple rules - first match wins",
			profileID:        "test-profile",
			domain:           "multi.example.com",
			customRuleHashes: []string{"hash1", "hash2"},
			customRules: map[string]map[string]string{
				"hash1": {
					"action": ACTION_BLOCK,
					"value":  "multi.example.com",
				},
				"hash2": {
					"action": ACTION_ALLOW,
					"value":  "multi.example.com",
				},
			},
			expectedFltrResult: &model.StageResult{
				Decision: model.DecisionBlock,
				Tier:     TierCustomRules,
				Reasons:  []string{REASON_CUSTOM_RULES},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock cache
			mockCache := new(mocks.Cache)

			// Setup mock expectations
			mockCache.On("GetCustomRulesHashes", mock.Anything, tt.profileID).
				Return(tt.customRuleHashes, nil)

			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).
					Return(rule, nil).Maybe()
			}

			// Create filter manager with mock cache
			dnsProxy := &proxy.Proxy{}
			fm := NewDomainFilter(dnsProxy, mockCache, nil)

			// Create DNS message
			msg := new(dns.Msg)
			msg.SetQuestion(tt.domain+".", dns.TypeA)

			// Create a test logger to avoid nil pointer dereference
			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			testLogger := loggerFactory.ForProfile(tt.profileID, true)

			// Create request context
			reqCtx := &requestcontext.RequestContext{
				ProfileId: tt.profileID,
				Logger:    testLogger,
			}
			dnsCtx := &proxy.DNSContext{
				Req: msg,
			}

			// Call the function
			got, err := fm.filterCustomRules(reqCtx, dnsCtx)
			// Assert results
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tt.expectedFltrResult, got)

			// Verify all mock expectations were met
			mockCache.AssertExpectations(t)
		})
	}
}

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		pattern string
		want    bool
	}{
		{
			name:    "Exact match",
			domain:  "example.com",
			pattern: "example.com",
			want:    true,
		},
		{
			name:    "Subdomain wildcard match",
			domain:  "sub1.example.com",
			pattern: "*.example.com",
			want:    true,
		},
		{
			name:    "Subdomain wildcard match 2",
			domain:  "ads.facebook.com",
			pattern: "*ads.facebook.com",
			want:    true,
		},
		{
			name:    "Subdomain wildcard match 3",
			domain:  "euads.facebook.com",
			pattern: "*ads.facebook.com",
			want:    true,
		},
		{
			name:    "Subdomain wildcard match 4",
			domain:  "ads.facebook.com",
			pattern: "*.facebook.com",
			want:    true,
		},
		{
			name:    "Wildcard with root domain match",
			domain:  "facebook.com",
			pattern: "*.facebook.com",
			want:    true,
		},
		{
			name:    "Subdomain wildcard no match",
			domain:  "sub1.different.com",
			pattern: "*.example.com",
			want:    false,
		},
		{
			name:    "Multiple level subdomain match",
			domain:  "a.b.example.com",
			pattern: "*.example.com",
			want:    true,
		},
		{
			name:    "Prefix wildcard match",
			domain:  "mysubdomain.example.com",
			pattern: "my*.example.com",
			want:    true,
		},
		{
			name:    "Prefix wildcard no match",
			domain:  "other.example.com",
			pattern: "my*.example.com",
			want:    false,
		},
		{
			name:    "Middle wildcard match",
			domain:  "sub-test-domain.example.com",
			pattern: "sub-*-domain.example.com",
			want:    true,
		},
		{
			name:    "Middle wildcard no match",
			domain:  "sub.example.com",
			pattern: "sub-*-domain.example.com",
			want:    false,
		},
		{
			name:    "Invalid regex pattern",
			domain:  "example.com",
			pattern: "[invalid.regex*",
			want:    false,
		},
		{
			name:    "Case sensitive match",
			domain:  "Example.Com",
			pattern: "example.com",
			want:    true,
		},
		{
			name:    "Exact domain does not match subdomain",
			domain:  "sub.ads.com",
			pattern: "ads.com",
			want:    false,
		},
		{
			name:    "Star prefix wildcard matches root and subdomain",
			domain:  "ads.com",
			pattern: "*.ads.com",
			want:    true,
		},
		{
			name:    "Dot prefix behaves like star prefix for root",
			domain:  "ads.com",
			pattern: ".ads.com",
			want:    true,
		},
		{
			name:    "Dot prefix behaves like star prefix for subdomain",
			domain:  "sub.ads.com",
			pattern: ".ads.com",
			want:    true,
		},
		{
			name:    "Suffix wildcard matches different TLD",
			domain:  "ads.de",
			pattern: "ads.*",
			want:    true,
		},
		{
			name:    "Suffix wildcard does not match bare base",
			domain:  "ads",
			pattern: "ads.*",
			want:    false,
		},
		{
			name:    "Suffix wildcard does not match subdomain",
			domain:  "sub.ads.com",
			pattern: "ads.*",
			want:    false,
		},
		{
			name:    "Contains wildcard matches substring",
			domain:  "shopads.io",
			pattern: "*ads*",
			want:    true,
		},
		{
			name:    "Contains wildcard matches exact word",
			domain:  "ads",
			pattern: "*ads*",
			want:    true,
		},
		{
			name:    "Contains wildcard matches subdomain",
			domain:  "sub.ads.com",
			pattern: "*ads*",
			want:    true,
		},
	}

	for _, tt := range tests {
		mockCache := new(mocks.Cache)
		proxy := &proxy.Proxy{}
		fm := NewDomainFilter(proxy, mockCache, nil)
		t.Run(tt.name, func(t *testing.T) {
			got := fm.matchDomain(tt.domain, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIPFilter_FilterCustomRules_ASN_Table(t *testing.T) {
	const profileID = "test-profile-ip-asn-table"
	const asn = uint(15169)

	makeDNSCtx := func(t *testing.T, answers []dns.RR) *proxy.DNSContext {
		t.Helper()
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		res := new(dns.Msg)
		res.SetReply(req)
		res.Answer = answers
		return &proxy.DNSContext{Req: req, Res: res}
	}

	tests := []struct {
		name             string
		customRuleHashes []string
		customRules      map[string]map[string]string
		setupASNLookup   func(t *testing.T) ASNLookup
		dnsCtx           *proxy.DNSContext
		wantDecision     model.Decision
		wantHasReason    bool
	}{
		{
			name:             "allow by asn (A answer)",
			customRuleHashes: []string{"hash_allow_asn"},
			customRules: map[string]map[string]string{
				"hash_allow_asn": {"action": ACTION_ALLOW, "value": "AS15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("1.1.1.1")).Return(asn, nil)
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionAllow,
			wantHasReason: true,
		},
		{
			name:             "block by asn (A answer)",
			customRuleHashes: []string{"hash_block_asn"},
			customRules: map[string]map[string]string{
				"hash_block_asn": {"action": ACTION_BLOCK, "value": "15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("1.1.1.1")).Return(asn, nil)
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionBlock,
			wantHasReason: true,
		},
		{
			name:             "allow by asn (AAAA answer)",
			customRuleHashes: []string{"hash_allow_asn"},
			customRules: map[string]map[string]string{
				"hash_allow_asn": {"action": ACTION_ALLOW, "value": "as15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("2001:db8::1")).Return(asn, nil)
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.AAAA{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}, AAAA: net.ParseIP("2001:db8::1")},
			}),
			wantDecision:  model.DecisionAllow,
			wantHasReason: true,
		},
		{
			name:             "invalid asn value is ignored",
			customRuleHashes: []string{"hash_invalid_asn"},
			customRules: map[string]map[string]string{
				"hash_invalid_asn": {"action": ACTION_BLOCK, "value": "AS", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				return mocks.NewASNLookup(t)
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionNone,
			wantHasReason: false,
		},
		{
			name:             "missing syntax is ignored (old rule)",
			customRuleHashes: []string{"hash_old"},
			customRules: map[string]map[string]string{
				"hash_old": {"action": ACTION_BLOCK, "value": "AS15169"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				return mocks.NewASNLookup(t)
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionNone,
			wantHasReason: false,
		},
		{
			name:             "asn lookup returns error => no match",
			customRuleHashes: []string{"hash_block_asn"},
			customRules: map[string]map[string]string{
				"hash_block_asn": {"action": ACTION_BLOCK, "value": "15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("1.1.1.1")).Return(uint(0), errors.New("lookup failed"))
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionNone,
			wantHasReason: false,
		},
		{
			name:             "asn lookup returns 0 => no match",
			customRuleHashes: []string{"hash_block_asn"},
			customRules: map[string]map[string]string{
				"hash_block_asn": {"action": ACTION_BLOCK, "value": "15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("1.1.1.1")).Return(uint(0), nil)
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionNone,
			wantHasReason: false,
		},
		{
			name:             "block wins when allow and block both match",
			customRuleHashes: []string{"hash_allow_asn", "hash_block_asn"},
			customRules: map[string]map[string]string{
				"hash_allow_asn": {"action": ACTION_ALLOW, "value": "AS15169", "syntax": "asn"},
				"hash_block_asn": {"action": ACTION_BLOCK, "value": "15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup {
				m := mocks.NewASNLookup(t)
				m.On("ASN", net.ParseIP("1.1.1.1")).Return(asn, nil)
				return m
			},
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionBlock,
			wantHasReason: true,
		},
		{
			name:             "asn rules are skipped when asn lookup is nil",
			customRuleHashes: []string{"hash_block_asn"},
			customRules: map[string]map[string]string{
				"hash_block_asn": {"action": ACTION_BLOCK, "value": "AS15169", "syntax": "asn"},
			},
			setupASNLookup: func(t *testing.T) ASNLookup { return nil },
			dnsCtx: makeDNSCtx(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
			}),
			wantDecision:  model.DecisionNone,
			wantHasReason: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return(tt.customRuleHashes, nil)
			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).Return(rule, nil).Maybe()
			}

			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			testLogger := loggerFactory.ForProfile(profileID, true)
			reqCtx := &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}

			var asnLookup ASNLookup
			if tt.setupASNLookup != nil {
				asnLookup = tt.setupASNLookup(t)
			}

			ipFilter := &IPFilter{Cache: mockCache, ASNLookup: asnLookup}
			got, err := ipFilter.filterCustomRules(reqCtx, tt.dnsCtx)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, TierCustomRules, got.Tier)
			assert.Equal(t, tt.wantDecision, got.Decision)
			if tt.wantHasReason {
				assert.Contains(t, got.Reasons, REASON_CUSTOM_RULES)
			} else {
				assert.NotContains(t, got.Reasons, REASON_CUSTOM_RULES)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

// specRef: logging-behaviour.md A3
func TestFilterCustomRulesDomainNotLoggedWhenGateOff(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Logger
	log.Logger = zerolog.New(&buf)
	factory := logging.NewFactory(zerolog.DebugLevel)
	logger := factory.ForRequest(logging.LoggingConfig{
		Enabled:    true,
		ProfileID:  "prof-1",
		LogDomains: false,
	})
	log.Logger = orig

	mockCache := new(mocks.Cache)
	mockCache.On("GetCustomRulesHashes", mock.Anything, "prof-1").
		Return([]string{"h1"}, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "h1").
		Return(map[string]string{"action": ACTION_BLOCK, "value": "blocked.example.com"}, nil)

	fm := NewDomainFilter(&proxy.Proxy{}, mockCache, nil)
	msg := new(dns.Msg)
	msg.SetQuestion("blocked.example.com.", dns.TypeA)
	reqCtx := &requestcontext.RequestContext{
		ProfileId:    "prof-1",
		Logger:       logger,
		LoggerConfig: logger.Config(),
	}

	got, err := fm.filterCustomRules(reqCtx, &proxy.DNSContext{Req: msg})

	assert.NoError(t, err)
	assert.Equal(t, model.DecisionBlock, got.Decision)
	assert.NotContains(t, buf.String(), "blocked.example.com")
}
