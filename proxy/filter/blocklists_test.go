package filter

import (
	"context"
	"errors"
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

func TestFilterBlocklists(t *testing.T) {
	const (
		blocklistID1 = "bl1"
		blocklistID2 = "bl2"
	)

	tests := []struct {
		name             string
		profileID        string
		questionDomain   string
		blocklists       []string
		blocklistEntries map[string]map[string]bool // blocklistID -> domain -> isBlocked
		privacySettings  map[string]string
		expectBlocked    bool
		expectReasons    []string
		expectErr        bool
		cacheErr         error
	}{
		{
			name:           "Exact match - blocked",
			profileID:      "profile1",
			questionDomain: "blocked.example.com",
			blocklists:     []string{blocklistID1},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {"blocked.example.com": true},
			},
			privacySettings: map[string]string{},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1"},
			expectErr:       false,
		},
		{
			name:           "No match - processed",
			profileID:      "profile2",
			questionDomain: "notblocked.example.com",
			blocklists:     []string{blocklistID1},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {"blocked.example.com": true},
			},
			privacySettings: map[string]string{},
			expectBlocked:   false,
			expectReasons:   nil,
			expectErr:       false,
		},
		{
			name:           "Subdomain match - blocked",
			profileID:      "profile3",
			questionDomain: "sub.blocked.com",
			blocklists:     []string{blocklistID1},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {
					"blocked.com": true,
				},
			},
			privacySettings: map[string]string{
				SUBDOMAINS_RULE: RULE_BLOCK,
			},
			expectBlocked: true,
			expectReasons: []string{"blocklist: bl1", SUBDOMAINS_RULE},
			expectErr:     false,
		},
		{
			name:           "Subdomain match - privacy setting off",
			profileID:      "profile4",
			questionDomain: "sub.blocked.com",
			blocklists:     []string{blocklistID1},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {
					"blocked.com": true,
				},
			},
			privacySettings: map[string]string{
				SUBDOMAINS_RULE: RULE_ALLOW,
			},
			expectBlocked: false,
			expectReasons: nil,
			expectErr:     false,
		},
		{
			name:           "Multiple blocklists - first blocks",
			profileID:      "profile5",
			questionDomain: "foo.com",
			blocklists:     []string{blocklistID1, blocklistID2},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {"foo.com": true},
				blocklistID2: {"foo.com": false},
			},
			privacySettings: map[string]string{},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1"},
			expectErr:       false,
		},
		{
			name:             "Cache error on GetProfileBlocklists",
			profileID:        "profile6",
			questionDomain:   "foo.com",
			blocklists:       nil,
			blocklistEntries: map[string]map[string]bool{},
			privacySettings:  map[string]string{},
			expectBlocked:    false,
			expectReasons:    nil,
			expectErr:        true,
			cacheErr:         errors.New("cache error"),
		},
		{
			name:           "Cache error on GetBlocklistEntry",
			profileID:      "profile7",
			questionDomain: "foo.com",
			blocklists:     []string{blocklistID1},
			blocklistEntries: map[string]map[string]bool{
				blocklistID1: {},
			},
			privacySettings: map[string]string{},
			expectBlocked:   false,
			expectReasons:   nil,
			expectErr:       true,
			cacheErr:        errors.New("blocklist entry error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)

			// Setup GetProfileBlocklists
			if tt.cacheErr != nil && (tt.name == "Cache error on GetProfileBlocklists") {
				mockCache.On("GetProfileBlocklists", mock.Anything, tt.profileID).
					Return(nil, tt.cacheErr)
			} else {
				mockCache.On("GetProfileBlocklists", mock.Anything, tt.profileID).
					Return(tt.blocklists, nil)
			}

			if tt.name == "Multiple blocklists - first blocks" {
				entries := tt.blocklistEntries[blocklistID1]
				var blocked bool
				if entries != nil {
					blocked = entries[tt.questionDomain]
				}
				mockCache.On("GetBlocklistEntry", mock.Anything, blocklistID1, mock.Anything).Return(blocked, nil).Once()
			} else {
				// Setup GetBlocklistEntry
				for _, blID := range tt.blocklists {
					entries := tt.blocklistEntries[blID]
					// For exact match
					if tt.cacheErr != nil && (tt.name == "Cache error on GetBlocklistEntry") {
						mockCache.On("GetBlocklistEntry", mock.Anything, blID, mock.Anything).
							Return(false, tt.cacheErr)
					} else {
						if tt.name == "Subdomain match - blocked" {
							mockCache.On("GetBlocklistEntry", mock.Anything, blID, tt.questionDomain).Return(false, nil)
							// For subdomain match, we need to check all subdomains
							for domain, blocked := range entries {
								mockCache.On("GetBlocklistEntry", mock.Anything, blID, domain).Return(blocked, nil)
							}
						} else {
							var blocked bool
							if entries != nil {
								blocked = entries[tt.questionDomain]
							}
							mockCache.On("GetBlocklistEntry", mock.Anything, blID, mock.Anything).Return(blocked, nil)
						}
					}
				}
			}

			// No case in this table publishes exceptions; the consult on the
			// would-block path must see an absent set (specRef: #X5).
			mockCache.On("GetBlocklistExceptionEntry", mock.Anything, mock.Anything, mock.Anything).
				Return(false, nil).Maybe()

			dnsProxy := &proxy.Proxy{}
			fm := NewDomainFilter(dnsProxy, mockCache, nil)

			msg := new(dns.Msg)
			msg.SetQuestion(tt.questionDomain+".", dns.TypeA)

			// Create a test logger to avoid nil pointer dereference
			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			testLogger := loggerFactory.ForProfile(tt.profileID, true)

			reqCtx := &requestcontext.RequestContext{
				ProfileId:       tt.profileID,
				PrivacySettings: tt.privacySettings,
				Logger:          testLogger,
			}
			dnsCtx := &proxy.DNSContext{
				Req: msg,
			}

			result, err := fm.filterBlocklists(reqCtx, dnsCtx)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			if tt.expectBlocked {
				assert.Equal(t, model.DecisionBlock, result.Decision)
				assert.Equal(t, TierBlocklists, result.Tier)
				assert.ElementsMatch(t, tt.expectReasons, result.Reasons)
			} else {
				assert.Equal(t, model.DecisionNone, result.Decision)
				assert.Equal(t, TierBlocklists, result.Tier)
				assert.Nil(t, result.Reasons)
			}
			mockCache.AssertExpectations(t)
		})
	}
}

// TestFilterBlocklists_Exceptions covers the list-level exception consult:
// a match from list L is withdrawn when L's own exception set covers the
// query name, without creating an Allow and without weakening other lists.
func TestFilterBlocklists_Exceptions(t *testing.T) {
	const (
		listL = "adguard_dns_filter"
		listM = "hagezi_pro"
	)

	tests := []struct {
		name            string
		questionDomain  string
		blocklists      []string
		blockEntries    map[string]map[string]bool // list -> domain -> blocked
		exceptions      map[string]map[string]bool // list -> domain -> excepted
		privacySettings map[string]string
		exceptionErr    error
		expectBlocked   bool
		expectReasons   []string
		expectErr       bool
	}{
		{
			// specRef: #X1 — the list's own unblock withdraws the exact match.
			name:           "exact match suppressed by same-list exception",
			questionDomain: "data.orders.costco.com",
			blocklists:     []string{listL},
			blockEntries:   map[string]map[string]bool{listL: {"data.orders.costco.com": true}},
			exceptions:     map[string]map[string]bool{listL: {"data.orders.costco.com": true}},
			expectBlocked:  false,
		},
		{
			// specRef: #X2 — exception on the query name suppresses a
			// parent-walk hit under blocklists_subdomains_rule = block.
			name:            "parent-walk match suppressed by exception on query name",
			questionDomain:  "sbs.demdex.net",
			blocklists:      []string{listL},
			blockEntries:    map[string]map[string]bool{listL: {"demdex.net": true}},
			exceptions:      map[string]map[string]bool{listL: {"sbs.demdex.net": true}},
			privacySettings: map[string]string{SUBDOMAINS_RULE: RULE_BLOCK},
			expectBlocked:   false,
		},
		{
			// specRef: #X3 — the exception walk covers subdomains of the
			// excepted name regardless of the subdomains rule.
			name:           "exact match suppressed by exception on parent",
			questionDomain: "x.sbs.demdex.net",
			blocklists:     []string{listL},
			blockEntries:   map[string]map[string]bool{listL: {"x.sbs.demdex.net": true}},
			exceptions:     map[string]map[string]bool{listL: {"sbs.demdex.net": true}},
			expectBlocked:  false,
		},
		{
			// specRef: #X4 — per-source scoping: L's exception cannot weaken
			// M; scanning continues and M's block stands.
			name:           "exception scoped to its list, other list still blocks",
			questionDomain: "tracker.example.com",
			blocklists:     []string{listL, listM},
			blockEntries: map[string]map[string]bool{
				listL: {"tracker.example.com": true},
				listM: {"tracker.example.com": true},
			},
			exceptions:    map[string]map[string]bool{listL: {"tracker.example.com": true}},
			expectBlocked: true,
			expectReasons: []string{"blocklist: " + listM},
		},
		{
			// specRef: #X5 — no exception set published: unchanged blocking.
			name:           "no exceptions published, block stands",
			questionDomain: "blocked.example.com",
			blocklists:     []string{listL},
			blockEntries:   map[string]map[string]bool{listL: {"blocked.example.com": true}},
			expectBlocked:  true,
			expectReasons:  []string{"blocklist: " + listL},
		},
		{
			// specRef: #X6 — exception lookup errors propagate like block
			// lookup errors.
			name:           "exception lookup error propagates",
			questionDomain: "blocked.example.com",
			blocklists:     []string{listL},
			blockEntries:   map[string]map[string]bool{listL: {"blocked.example.com": true}},
			exceptionErr:   errors.New("exception lookup error"),
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileBlocklists", mock.Anything, "profileX").
				Return(tt.blocklists, nil)
			for _, blID := range tt.blocklists {
				entries := tt.blockEntries[blID]
				mockCache.On("GetBlocklistEntry", mock.Anything, blID, mock.MatchedBy(func(string) bool { return true })).
					Return(func(_ context.Context, blocklistId, domain string) (bool, error) {
						return entries[domain], nil
					})
				excepted := tt.exceptions[blID]
				if tt.exceptionErr != nil {
					mockCache.On("GetBlocklistExceptionEntry", mock.Anything, blID, mock.Anything).
						Return(false, tt.exceptionErr)
				} else {
					mockCache.On("GetBlocklistExceptionEntry", mock.Anything, blID, mock.Anything).
						Return(func(_ context.Context, blocklistId, domain string) (bool, error) {
							return excepted[domain], nil
						}).Maybe()
				}
			}

			fm := NewDomainFilter(&proxy.Proxy{}, mockCache, nil)

			msg := new(dns.Msg)
			msg.SetQuestion(tt.questionDomain+".", dns.TypeA)
			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			reqCtx := &requestcontext.RequestContext{
				ProfileId:       "profileX",
				PrivacySettings: tt.privacySettings,
				Logger:          loggerFactory.ForProfile("profileX", true),
			}
			dnsCtx := &proxy.DNSContext{Req: msg}

			result, err := fm.filterBlocklists(reqCtx, dnsCtx)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			if tt.expectBlocked {
				assert.Equal(t, model.DecisionBlock, result.Decision)
				assert.Equal(t, tt.expectReasons, result.Reasons)
			} else {
				// A suppressed match must leave the decision at None — an
				// exception never produces an Allow.
				assert.Equal(t, model.DecisionNone, result.Decision)
				assert.Empty(t, result.Reasons)
			}
		})
	}
}
