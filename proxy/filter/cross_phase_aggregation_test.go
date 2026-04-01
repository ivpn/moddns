package filter

import (
	"net"
	"net/netip"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// domainAllowResult returns a domain-phase Allow StageResult at TierCustomRules.
func domainAllowResult() model.StageResult {
	return model.StageResult{
		Decision: model.DecisionAllow,
		Tier:     TierCustomRules,
		Reasons:  []string{REASON_CUSTOM_RULES},
	}
}

// domainBlocklistResult returns a domain-phase Block StageResult at TierBlocklists.
func domainBlocklistResult() model.StageResult {
	return model.StageResult{
		Decision: model.DecisionBlock,
		Tier:     TierBlocklists,
		Reasons:  []string{"blocklists"},
	}
}

// domainCustomBlockResult returns a domain-phase Block StageResult at TierCustomRules.
func domainCustomBlockResult() model.StageResult {
	return model.StageResult{
		Decision: model.DecisionBlock,
		Tier:     TierCustomRules,
		Reasons:  []string{REASON_CUSTOM_RULES},
	}
}

// TestIPFilter_CrossPhaseAggregation verifies that IPFilter.Execute aggregates
// all results from both domain and IP phases. Domain-phase custom Allow (T200)
// overrides IP-phase service blocks (T100) and IP-phase custom blocks (T200),
// following the global aggregation rule: any Allow present wins.
//
// Table references correspond to the cross-phase behaviour table in
// docs/proxy-filtering-behaviour.md.
func TestIPFilter_CrossPhaseAggregation(t *testing.T) {
	const (
		profileID = "phase-independence"
		asn       = uint(15169)
		answerIP  = "1.1.1.1"
	)

	tests := []struct {
		name string
		// tableRef documents which row(s) in the behaviour table this covers.
		tableRef string
		// domainResults are pre-populated in reqCtx.PartialFilteringResults
		// to simulate domain-phase output that ran before the IP phase.
		domainResults []model.StageResult
		// services configuration
		blockedServiceIDs []string
		catalog           ServicesCatalogGetter
		asnLookup         ASNLookup
		// IP custom rules configuration
		customHashes []string
		customRules  map[string]map[string]string
		// DNS context
		dnsCtx *proxy.DNSContext
		// expectations
		wantStatus      model.Status
		wantContains    []string
		wantNotContains []string
	}{
		// ── Section A: Domain Processed (no domain Allow) + IP phase ──

		{
			name:              "#1 — no rules matched anywhere",
			tableRef:          "#1",
			domainResults:     nil,
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0}, // no ASN match
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusProcessed,
		},
		{
			name:              "#2 — SVC Block only",
			tableRef:          "#2",
			domainResults:     nil,
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusBlocked,
			wantContains:      []string{REASON_SERVICES},
		},
		{
			name:              "#3 — IP CR Block only",
			tableRef:          "#3",
			domainResults:     nil,
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusBlocked,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#4 — SVC Block + IP CR Block",
			tableRef:          "#4",
			domainResults:     nil,
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:     dnsCtxWithAAnswer(t, answerIP),
			wantStatus: model.StatusBlocked,
		},
		{
			name:              "#5 — IP CR Allow only",
			tableRef:          "#5",
			domainResults:     nil,
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusProcessed,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#6 — SVC Block + IP CR Allow → custom overrides services within IP phase",
			tableRef:          "#6",
			domainResults:     nil,
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:          dnsCtxWithAAnswer(t, answerIP),
			wantStatus:      model.StatusProcessed,
			wantContains:    []string{REASON_CUSTOM_RULES},
			wantNotContains: []string{REASON_SERVICES},
		},

		// ── Section A: Domain Allow + IP phase (behaviour-changing scenarios) ──

		{
			name:              "#7 — Domain Allow + IP no opinion → Processed",
			tableRef:          "#7",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusProcessed,
		},
		{
			name:              "#8 — Domain Allow + SVC Block → Processed (domain T200 overrides service T100)",
			tableRef:          "#8",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusProcessed,
			wantContains:      []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#9 — Domain Allow + IP CR Block → Processed (T200 allow overrides T200 block)",
			tableRef:          "#9",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusProcessed,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#10 — Domain Allow + SVC Block + IP CR Block → Processed (T200 allow wins)",
			tableRef:          "#10",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusProcessed,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#11 — Domain Allow + IP CR Allow → Processed",
			tableRef:          "#11",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:     dnsCtxWithAAnswer(t, answerIP),
			wantStatus: model.StatusProcessed,
		},
		{
			name:              "#12 — Domain Allow + SVC Block + IP CR Allow → Processed (IP custom > services)",
			tableRef:          "#12",
			domainResults:     []model.StageResult{domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:          dnsCtxWithAAnswer(t, answerIP),
			wantStatus:      model.StatusProcessed,
			wantContains:    []string{REASON_CUSTOM_RULES},
			wantNotContains: []string{REASON_SERVICES},
		},

		// ── Blocklist Block + Domain Allow + IP phase (same IP-phase behaviour) ──

		{
			name:              "#13 — BL Block + Domain Allow + IP no opinion → Processed",
			tableRef:          "#13",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusProcessed,
		},
		{
			name:              "#14 — BL Block + Domain Allow + SVC Block → Processed (domain T200 overrides)",
			tableRef:          "#14",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{},
			dnsCtx:            dnsCtxWithAAnswer(t, answerIP),
			wantStatus:        model.StatusProcessed,
			wantContains:      []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#15 — BL Block + Domain Allow + IP CR Block → Processed (T200 allow wins)",
			tableRef:          "#15",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusProcessed,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#16 — BL Block + Domain Allow + SVC Block + IP CR Block → Processed (T200 allow wins)",
			tableRef:          "#16",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_block_ip"},
			customRules: map[string]map[string]string{
				"h_block_ip": {"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:       dnsCtxWithAAnswer(t, answerIP),
			wantStatus:   model.StatusProcessed,
			wantContains: []string{REASON_CUSTOM_RULES},
		},
		{
			name:              "#17 — BL Block + Domain Allow + IP CR Allow → Processed",
			tableRef:          "#17",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: 0},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:     dnsCtxWithAAnswer(t, answerIP),
			wantStatus: model.StatusProcessed,
		},
		{
			name:              "#18 — BL Block + Domain Allow + SVC Block + IP CR Allow → Processed",
			tableRef:          "#18",
			domainResults:     []model.StageResult{domainBlocklistResult(), domainAllowResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": answerIP, "syntax": "ip4_addr"},
			},
			dnsCtx:          dnsCtxWithAAnswer(t, answerIP),
			wantStatus:      model.StatusProcessed,
			wantContains:    []string{REASON_CUSTOM_RULES},
			wantNotContains: []string{REASON_SERVICES},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)

			// Services cache setup
			if len(tt.blockedServiceIDs) > 0 {
				mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
					Return(tt.blockedServiceIDs, nil)
			} else {
				mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
					Return([]string{}, nil)
			}

			// Custom rules cache setup
			mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
				Return(tt.customHashes, nil)
			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).
					Return(rule, nil).Maybe()
			}

			ipFilter := NewIPFilter(&proxy.Proxy{}, mockCache, tt.catalog, tt.asnLookup)

			reqCtx := newTestReqCtx(t, profileID)
			// Pre-populate with domain-phase results to simulate the real pipeline.
			reqCtx.PartialFilteringResults = append(
				reqCtx.PartialFilteringResults, tt.domainResults...,
			)

			err := ipFilter.Execute(reqCtx, tt.dnsCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, reqCtx.FilterResult.Status,
				"table %s: expected status %s", tt.tableRef, tt.wantStatus)

			for _, r := range tt.wantContains {
				assert.Contains(t, reqCtx.FilterResult.Reasons, r,
					"table %s: expected reason %q", tt.tableRef, r)
			}
			for _, r := range tt.wantNotContains {
				assert.NotContains(t, reqCtx.FilterResult.Reasons, r,
					"table %s: unexpected reason %q", tt.tableRef, r)
			}

			// Verify domain results are preserved in PartialFilteringResults
			// for observability (query logs).
			for _, dr := range tt.domainResults {
				assert.Contains(t, reqCtx.PartialFilteringResults, dr,
					"table %s: domain result should remain in PartialFilteringResults", tt.tableRef)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

// TestIPFilter_NilResponse_PreservesDomainBlock verifies that when dctx.Res is
// nil (domain blocked, no upstream resolution), IPFilter.Execute preserves the
// domain block through unified aggregation. The server-level postResolve guard
// still prevents this call in practice, but even without it the result is
// correct (table #19-#21).
func TestIPFilter_NilResponse_PreservesDomainBlock(t *testing.T) {
	const profileID = "nil-response"

	tests := []struct {
		name          string
		tableRef      string
		domainResults []model.StageResult
		priorStatus   model.Status
	}{
		{
			name:          "#19 — BL Block domain, nil Res → IP preserves Blocked via unified aggregation",
			tableRef:      "#19",
			domainResults: []model.StageResult{domainBlocklistResult()},
			priorStatus:   model.StatusBlocked,
		},
		{
			name:          "#20 — Domain CR Block, nil Res → IP preserves Blocked via unified aggregation",
			tableRef:      "#20",
			domainResults: []model.StageResult{domainCustomBlockResult()},
			priorStatus:   model.StatusBlocked,
		},
		{
			name:     "#21 — BL + Domain CR Block, nil Res → IP preserves Blocked via unified aggregation",
			tableRef: "#21",
			domainResults: []model.StageResult{
				domainBlocklistResult(),
				domainCustomBlockResult(),
			},
			priorStatus: model.StatusBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
				Return([]string{}, nil).Maybe()
			mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
				Return([]string{}, nil).Maybe()

			ipFilter := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil)

			reqCtx := newTestReqCtx(t, profileID)
			reqCtx.PartialFilteringResults = append(
				reqCtx.PartialFilteringResults, tt.domainResults...,
			)
			// Simulate the domain-phase FilterResult that would be set before
			// postResolve is called.
			reqCtx.FilterResult = model.FilterResult{Status: tt.priorStatus}

			// dctx.Res is nil — no upstream resolution occurred.
			req := new(dns.Msg)
			req.SetQuestion("blocked.example.com.", dns.TypeA)
			dnsCtx := &proxy.DNSContext{Req: req, Res: nil}

			err := ipFilter.Execute(reqCtx, dnsCtx)
			assert.NoError(t, err)

			// With unified aggregation, domain-phase Block results propagate
			// into IP-phase aggregation. Even though IP sub-filters return
			// None (nil Res), the domain Block is preserved.
			assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status,
				"table %s: domain block preserved via unified aggregation", tt.tableRef)
		})
	}
}

// TestIPFilter_NilResponse_IPAllowInert verifies that when the domain phase
// blocks (dctx.Res is nil), configured IP allow rules are inert — they cannot
// match without response IPs. With unified aggregation the domain Block
// propagates, so the final result is Blocked (table #24-#27).
func TestIPFilter_NilResponse_IPAllowInert(t *testing.T) {
	const (
		profileID = "nil-response-ip-allow"
		asn       = uint(15169)
		allowIP   = "1.1.1.1"
	)

	tests := []struct {
		name              string
		tableRef          string
		domainResults     []model.StageResult
		blockedServiceIDs []string
		catalog           ServicesCatalogGetter
		asnLookup         ASNLookup
		customHashes      []string
		customRules       map[string]map[string]string
	}{
		{
			name:              "#24 — Domain CR Block + IP CR Allow → Blocked (IP allow inert, nil Res)",
			tableRef:          "#24",
			domainResults:     []model.StageResult{domainCustomBlockResult()},
			blockedServiceIDs: []string{},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": allowIP, "syntax": "ip4_addr"},
			},
		},
		{
			name:              "#25 — Domain CR Block + SVC Block + IP CR Allow → Blocked (IP allow inert, nil Res)",
			tableRef:          "#25",
			domainResults:     []model.StageResult{domainCustomBlockResult()},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": allowIP, "syntax": "ip4_addr"},
			},
		},
		{
			name:     "#26 — BL Block + Domain CR Block + IP CR Allow → Blocked (IP allow inert, nil Res)",
			tableRef: "#26",
			domainResults: []model.StageResult{
				domainBlocklistResult(),
				domainCustomBlockResult(),
			},
			blockedServiceIDs: []string{},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": allowIP, "syntax": "ip4_addr"},
			},
		},
		{
			name:     "#27 — BL Block + Domain CR Block + SVC Block + IP CR Allow → Blocked (IP allow inert, nil Res)",
			tableRef: "#27",
			domainResults: []model.StageResult{
				domainBlocklistResult(),
				domainCustomBlockResult(),
			},
			blockedServiceIDs: []string{"google"},
			catalog:           staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:         staticASNLookup{asn: asn},
			customHashes:      []string{"h_allow_ip"},
			customRules: map[string]map[string]string{
				"h_allow_ip": {"action": ACTION_ALLOW, "value": allowIP, "syntax": "ip4_addr"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
				Return(tt.blockedServiceIDs, nil).Maybe()
			mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
				Return(tt.customHashes, nil)
			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).
					Return(rule, nil).Maybe()
			}

			ipFilter := NewIPFilter(&proxy.Proxy{}, mockCache, tt.catalog, tt.asnLookup)

			reqCtx := newTestReqCtx(t, profileID)
			reqCtx.PartialFilteringResults = append(
				reqCtx.PartialFilteringResults, tt.domainResults...,
			)
			reqCtx.FilterResult = model.FilterResult{Status: model.StatusBlocked}

			// dctx.Res is nil — domain blocked, no upstream resolution.
			req := new(dns.Msg)
			req.SetQuestion("blocked.example.com.", dns.TypeA)
			dnsCtx := &proxy.DNSContext{Req: req, Res: nil}

			err := ipFilter.Execute(reqCtx, dnsCtx)
			assert.NoError(t, err)

			// With unified aggregation, domain-phase Block propagates. IP allow
			// rules are inert (nil Res, can't match IPs), so domain Block wins.
			assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status,
				"table %s: domain block preserved — IP allow inert with nil Res", tt.tableRef)

			mockCache.AssertExpectations(t)
		})
	}
}

// TestIPFilter_CrossPhaseAggregation_PartialResultsGrow verifies that IP-phase
// results are appended to PartialFilteringResults and aggregated together with
// domain-phase results for the final decision.
func TestIPFilter_CrossPhaseAggregation_PartialResultsGrow(t *testing.T) {
	const (
		profileID = "partial-results-grow"
		asn       = uint(15169)
		answerIP  = "1.1.1.1"
	)

	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
		Return([]string{"google"}, nil)
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
		Return([]string{"h_block_ip"}, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "h_block_ip").
		Return(map[string]string{
			"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr",
		}, nil)

	ipFilter := NewIPFilter(
		&proxy.Proxy{}, mockCache,
		staticCatalog{cat: googleCatalogWithASN(asn)},
		staticASNLookup{asn: asn},
	)

	reqCtx := newTestReqCtx(t, profileID)
	// Start with one domain-phase result.
	reqCtx.PartialFilteringResults = []model.StageResult{domainAllowResult()}

	dnsCtx := dnsCtxWithAAnswer(t, answerIP)
	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)

	// Domain (1) + services (1) + custom rules (1) = 3 partial results.
	assert.Equal(t, 3, len(reqCtx.PartialFilteringResults),
		"PartialFilteringResults should contain domain + all IP-phase results")

	// Final decision based on unified aggregation: domain Allow (T200) wins
	// over services block (T100) and IP custom block (T200).
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
}

// TestIPFilter_NilResponse_SubFiltersReturnNone confirms both sub-filters
// individually return DecisionNone when dctx.Res is nil (guard behaviour
// relied upon by postResolve).
func TestIPFilter_NilResponse_SubFiltersReturnNone(t *testing.T) {
	const profileID = "nil-res-subfilters"

	mockCache := new(mocks.Cache)
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
		Return([]string{"h1"}, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "h1").
		Return(map[string]string{
			"action": ACTION_BLOCK, "value": "1.1.1.1", "syntax": "ip4_addr",
		}, nil)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
		Return([]string{"google"}, nil)

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	dnsCtx := &proxy.DNSContext{Req: req, Res: nil}
	reqCtx := newTestReqCtx(t, profileID)

	// filterServices with nil Res
	svcFilter := &IPFilter{
		Cache:           mockCache,
		ServicesCatalog: staticCatalog{cat: googleCatalogWithASN(15169)},
		ASNLookup:       staticASNLookup{asn: 15169},
	}
	svcResult, err := svcFilter.filterServices(reqCtx, dnsCtx)
	assert.NoError(t, err)
	assert.Equal(t, model.DecisionNone, svcResult.Decision, "filterServices must return None for nil Res")

	// filterCustomRules with nil Res
	crFilter := &IPFilter{
		Cache:     mockCache,
		ASNLookup: staticASNLookup{asn: 15169},
	}
	crResult, err := crFilter.filterCustomRules(reqCtx, dnsCtx)
	assert.NoError(t, err)
	assert.Equal(t, model.DecisionNone, crResult.Decision, "filterCustomRules must return None for nil Res")
}

// TestIPFilter_DnsCtxWithAddr verifies that Execute works correctly when
// dctx.Addr is set (as it would be in real requests for logging).
func TestIPFilter_DnsCtxWithAddr(t *testing.T) {
	const (
		profileID = "with-addr"
		answerIP  = "1.1.1.1"
	)

	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).
		Return([]string{}, nil)
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).
		Return([]string{"h_block_ip"}, nil)
	mockCache.On("GetCustomRulesHash", mock.Anything, "h_block_ip").
		Return(map[string]string{
			"action": ACTION_BLOCK, "value": answerIP, "syntax": "ip4_addr",
		}, nil)

	ipFilter := NewIPFilter(&proxy.Proxy{}, mockCache, nil, nil)

	reqCtx := newTestReqCtx(t, profileID)
	reqCtx.PartialFilteringResults = []model.StageResult{domainAllowResult()}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(answerIP),
		},
	}

	addr := netip.MustParseAddrPort("10.0.0.1:12345")
	dnsCtx := &proxy.DNSContext{Req: req, Res: res, Addr: addr}

	err := ipFilter.Execute(reqCtx, dnsCtx)
	assert.NoError(t, err)

	// Domain allow (T200) overrides IP block (T200) — allow always wins.
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
}
