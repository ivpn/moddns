package filter

import (
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

			dnsProxy := &proxy.Proxy{}
			fm := NewDomainFilter(dnsProxy, mockCache)

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
