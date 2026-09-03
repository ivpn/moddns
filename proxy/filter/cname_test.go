package filter

import (
	"net"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// buildCNAMEChainResponse creates a response whose answer section contains a
// CNAME chain qname -> chain[0] -> ... -> chain[n-1] followed by an A record
// on the last chain element (when finalIPv4 is non-empty).
func buildCNAMEChainResponse(qname string, qtype uint16, chain []string, finalIPv4 string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), qtype)
	msg.Response = true
	owner := dns.Fqdn(qname)
	for _, target := range chain {
		msg.Answer = append(msg.Answer, &dns.CNAME{
			Hdr:    dns.RR_Header{Name: owner, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: dns.Fqdn(target),
		})
		owner = dns.Fqdn(target)
	}
	if finalIPv4 != "" {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: owner, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(finalIPv4),
		})
	}
	return msg
}

// TestExtractCNAMETargets covers the answer-walk helper: dedupe, trailing-dot
// stripping, lowercasing, and QNAME exclusion.
func TestExtractCNAMETargets(t *testing.T) {
	tests := []struct {
		name     string
		response *dns.Msg
		qname    string
		want     []string
	}{
		{
			name:     "no CNAME records",
			response: buildCNAMEChainResponse("example.com", dns.TypeA, nil, "1.2.3.4"),
			qname:    "example.com.",
			want:     nil,
		},
		{
			name:     "single CNAME",
			response: buildCNAMEChainResponse("www.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			qname:    "www.shop.example.",
			want:     []string{"tracker.evil.net"},
		},
		{
			name:     "multi-hop chain",
			response: buildCNAMEChainResponse("www.shop.example", dns.TypeA, []string{"edge.cdn.example", "tracker.evil.net"}, "1.2.3.4"),
			qname:    "www.shop.example.",
			want:     []string{"edge.cdn.example", "tracker.evil.net"},
		},
		{
			name:     "mixed case target is lowercased",
			response: buildCNAMEChainResponse("www.shop.example", dns.TypeA, []string{"Tracker.Evil.NET"}, "1.2.3.4"),
			qname:    "www.shop.example.",
			want:     []string{"tracker.evil.net"},
		},
		{
			name: "duplicate targets deduped",
			response: func() *dns.Msg {
				m := buildCNAMEChainResponse("a.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4")
				m.Answer = append(m.Answer, &dns.CNAME{
					Hdr:    dns.RR_Header{Name: "a.example.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
					Target: "tracker.evil.net.",
				})
				return m
			}(),
			qname: "a.example.",
			want:  []string{"tracker.evil.net"},
		},
		{
			name:     "self-referential target equal to qname excluded",
			response: buildCNAMEChainResponse("loop.example", dns.TypeA, []string{"loop.example"}, ""),
			qname:    "loop.example.",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCNAMETargets(tt.response.Answer, tt.qname)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFilterCNAME covers Section F of docs/specs/proxy-filtering-behaviour.md
// (CNAME uncloaking) at the sub-filter level.
func TestFilterCNAME(t *testing.T) {
	const (
		profileID   = "cname-profile"
		blocklistID = "bl1"
	)

	tests := []struct {
		name     string
		tableRef string
		response *dns.Msg
		// nil response slice means: pass dctx with nil Res
		nilRes           bool
		blocklists       []string
		blocklistEntries map[string]map[string]bool // blocklistID -> domain -> blocked
		privacySettings  map[string]string
		customHashes     []string
		customRules      map[string]map[string]string
		// uncloakingDisabled simulates the CNAME_UNCLOAKING_ENABLED=false master switch.
		uncloakingDisabled bool
		// expectNoCacheCalls asserts the early exit fires before any Redis access.
		expectNoCacheCalls bool
		wantDecision       model.Decision
		wantTier           int
		wantReasons        []string
	}{
		{
			name:               "U1 — no CNAMEs in answer: early exit, no cache calls",
			tableRef:           "F/U1",
			response:           buildCNAMEChainResponse("plain.example.com", dns.TypeA, nil, "1.2.3.4"),
			expectNoCacheCalls: true,
			wantDecision:       model.DecisionNone,
		},
		{
			name:     "U2 — target on subscribed blocklist: Block T100",
			tableRef: "F/U2",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker.evil.net": true},
			},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionBlock,
			wantTier:        TierBlocklists,
			wantReasons:     []string{"blocklist: " + blocklistID, REASON_CNAME_UNCLOAKING},
		},
		{
			name:     "U3 — intermediate chain name on blocklist: Block T100",
			tableRef: "F/U3",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net", "edge.clean-cdn.example"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker.evil.net": true},
			},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionBlock,
			wantTier:        TierBlocklists,
			wantReasons:     []string{"blocklist: " + blocklistID, REASON_CNAME_UNCLOAKING},
		},
		{
			name:     "U4 — parent of target on blocklist, subdomains rule on: Block T100 + subdomains reason",
			tableRef: "F/U4",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"sub.tracker-park.net"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker-park.net": true},
			},
			privacySettings: map[string]string{SUBDOMAINS_RULE: RULE_BLOCK},
			customHashes:    []string{},
			wantDecision:    model.DecisionBlock,
			wantTier:        TierBlocklists,
			wantReasons:     []string{"blocklist: " + blocklistID, SUBDOMAINS_RULE, REASON_CNAME_UNCLOAKING},
		},
		{
			name:     "U5 — parent of target on blocklist, subdomains rule off: None",
			tableRef: "F/U5",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"sub.tracker-park.net"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker-park.net": true},
			},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionNone,
		},
		{
			name:     "U6 — target only on an unsubscribed list: None",
			tableRef: "F/U6",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {}, // subscribed list does not contain the target
			},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionNone,
		},
		{
			name:     "U7 — target matches custom Block rule (wildcard): Block T200",
			tableRef: "F/U7",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"sub.tracker.net"}, "1.2.3.4"),
			blocklists:      []string{},
			privacySettings: map[string]string{},
			customHashes:    []string{"h1"},
			customRules: map[string]map[string]string{
				"h1": {"action": ACTION_BLOCK, "value": "*.tracker.net", "syntax": "domain"},
			},
			wantDecision: model.DecisionBlock,
			wantTier:     TierCustomRules,
			wantReasons:  []string{REASON_CUSTOM_RULES, REASON_CNAME_UNCLOAKING},
		},
		{
			name:     "U8 — target matches custom Allow rule and a blocklist: Allow T200 wins",
			tableRef: "F/U8",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker.evil.net": true},
			},
			privacySettings: map[string]string{},
			customHashes:    []string{"h1"},
			customRules: map[string]map[string]string{
				"h1": {"action": ACTION_ALLOW, "value": "tracker.evil.net", "syntax": "domain"},
			},
			wantDecision: model.DecisionAllow,
			wantTier:     TierCustomRules,
			wantReasons:  []string{REASON_CUSTOM_RULES, REASON_CNAME_UNCLOAKING},
		},
		{
			// A nil Res is exactly the domain-blocked state (#U10): no upstream
			// resolution happened, so the stage must stay inert. The server-level
			// postResolve guard additionally skips the whole IP phase.
			name:               "U10/U11 — nil Res (domain blocked, no resolution): defensive None, no cache calls",
			tableRef:           "F/U10 F/U11",
			nilRes:             true,
			expectNoCacheCalls: true,
			wantDecision:       model.DecisionNone,
		},
		{
			name:               "U11 — empty answer: None, no cache calls",
			tableRef:           "F/U11",
			response:           buildCNAMEChainResponse("nxdomain.example", dns.TypeA, nil, ""),
			expectNoCacheCalls: true,
			wantDecision:       model.DecisionNone,
		},
		{
			name:     "U12 — HTTPS qtype answer carrying a CNAME: Block T100",
			tableRef: "F/U12",
			response: buildCNAMEChainResponse("metrics.shop.example", dns.TypeHTTPS, []string{"tracker.evil.net"}, ""),
			blocklists: []string{blocklistID},
			blocklistEntries: map[string]map[string]bool{
				blocklistID: {"tracker.evil.net": true},
			},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionBlock,
			wantTier:        TierBlocklists,
			wantReasons:     []string{"blocklist: " + blocklistID, REASON_CNAME_UNCLOAKING},
		},
		{
			name:            "U13 — CNAMEs present but no blocklists and no custom rules: None",
			tableRef:        "F/U13",
			response:        buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			blocklists:      []string{},
			privacySettings: map[string]string{},
			customHashes:    []string{},
			wantDecision:    model.DecisionNone,
		},
		{
			name:               "U14 — master switch off: None despite blocklisted target, no cache calls",
			tableRef:           "F/U14",
			response:           buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4"),
			uncloakingDisabled: true,
			expectNoCacheCalls: true,
			wantDecision:       model.DecisionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)

			if !tt.expectNoCacheCalls {
				mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
					Return(tt.customHashes, nil).Maybe()
				for hash, rule := range tt.customRules {
					mockCache.On("GetCustomRulesHash", mock.Anything, hash).
						Return(rule, nil).Maybe()
				}
				mockCache.On("GetProfileBlocklists", mock.Anything, profileID).
					Return(tt.blocklists, nil).Maybe()
				for blID, entries := range tt.blocklistEntries {
					for domain, blocked := range entries {
						mockCache.On("GetBlocklistEntry", mock.Anything, blID, domain).
							Return(blocked, nil).Maybe()
					}
				}
				mockCache.On("GetBlocklistEntry", mock.Anything, mock.Anything, mock.Anything).
					Return(false, nil).Maybe()
				mockCache.On("GetBlocklistExceptionEntry", mock.Anything, mock.Anything, mock.Anything).
					Return(false, nil).Maybe()
			}

			f := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil, nil,
				&config.FilteringConfig{CNAMEUncloakingEnabled: !tt.uncloakingDisabled})

			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			reqCtx := &requestcontext.RequestContext{
				ProfileId:       profileID,
				PrivacySettings: tt.privacySettings,
				Logger:          loggerFactory.ForProfile(profileID, true),
			}

			req := new(dns.Msg)
			if tt.response != nil {
				req.SetQuestion(tt.response.Question[0].Name, tt.response.Question[0].Qtype)
			} else {
				req.SetQuestion("metrics.shop.example.", dns.TypeA)
			}
			dnsCtx := &proxy.DNSContext{Req: req}
			if !tt.nilRes {
				dnsCtx.Res = tt.response
			}

			result, err := f.filterCNAME(reqCtx, dnsCtx)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.wantDecision, result.Decision, "tableRef %s", tt.tableRef)
			if tt.wantDecision != model.DecisionNone {
				assert.Equal(t, tt.wantTier, result.Tier, "tableRef %s", tt.tableRef)
				assert.ElementsMatch(t, tt.wantReasons, result.Reasons, "tableRef %s", tt.tableRef)
			}
			// expectNoCacheCalls rows registered no expectations, so any cache
			// access would have panicked inside the mock.
			mockCache.AssertExpectations(t)
		})
	}
}

// TestIPFilter_CrossPhase_CNAMEUncloaking verifies Section F rows that span
// phases, running the full IPFilter.Execute aggregation.
func TestIPFilter_CrossPhase_CNAMEUncloaking(t *testing.T) {
	const (
		profileID   = "cname-cross-phase"
		blocklistID = "bl1"
	)

	tests := []struct {
		name          string
		tableRef      string
		domainResults []model.StageResult
		wantStatus    model.Status
		wantContains  []string
	}{
		{
			name:         "U2 — CNAME target on blocklist, no domain opinion → Blocked",
			tableRef:     "F/U2",
			wantStatus:   model.StatusBlocked,
			wantContains: []string{"blocklist: " + blocklistID, REASON_CNAME_UNCLOAKING},
		},
		{
			name:          "U9 — CNAME target on blocklist + domain custom Allow → Processed",
			tableRef:      "F/U9",
			domainResults: []model.StageResult{domainAllowResult()},
			wantStatus:    model.StatusProcessed,
			wantContains:  []string{REASON_CUSTOM_RULES},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
				Return([]string{}, nil)
			mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
				Return([]string{}, nil)
			mockCache.On("GetProfileBlocklists", mock.Anything, profileID).
				Return([]string{blocklistID}, nil)
			mockCache.On("GetBlocklistEntry", mock.Anything, blocklistID, "tracker.evil.net").
				Return(true, nil)
			mockCache.On("GetBlocklistEntry", mock.Anything, mock.Anything, mock.Anything).
				Return(false, nil).Maybe()
			mockCache.On("GetBlocklistExceptionEntry", mock.Anything, mock.Anything, mock.Anything).
				Return(false, nil).Maybe()

			ipFilter := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil, nil, nil)

			reqCtx := newTestReqCtx(t, profileID)
			reqCtx.PartialFilteringResults = append(
				reqCtx.PartialFilteringResults, tt.domainResults...,
			)

			res := buildCNAMEChainResponse("metrics.shop.example", dns.TypeA, []string{"tracker.evil.net"}, "1.2.3.4")
			req := new(dns.Msg)
			req.SetQuestion("metrics.shop.example.", dns.TypeA)
			dnsCtx := &proxy.DNSContext{Req: req, Res: res}

			err := ipFilter.Execute(reqCtx, dnsCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, reqCtx.FilterResult.Status, "tableRef %s", tt.tableRef)
			for _, r := range tt.wantContains {
				assert.Contains(t, reqCtx.FilterResult.Reasons, r, "tableRef %s", tt.tableRef)
			}
		})
	}
}
