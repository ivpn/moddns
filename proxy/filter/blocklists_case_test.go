package filter

import (
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

// Blocklist matching must be case-insensitive.
//
// tableRef: #N1, #N2, #N3, #N4, #N5, #N6
// (docs/specs/proxy-filtering-behaviour.md - Section G: Name Normalisation)
//
// DNS preserves query-name case on the wire (RFC 1035 2.3.3) while name
// comparison is case-insensitive (RFC 4343), and the ingest pipeline stores every
// blocklist member lowercased. The cache lookup is a byte-exact set membership
// test, so the queried name must be normalised first.
//
// The mock models that byte-exact behaviour deliberately: membership is a map
// lookup on the raw string, so any case difference is a miss. TestFilterBlocklists
// uses mock.Anything for the domain argument and so cannot cover this.
func TestFilterBlocklistsIsCaseInsensitive(t *testing.T) {
	const blocklistID = "bl1"

	// Stored members are lowercase, as guaranteed by the ingest pipeline.
	storedLowercase := []string{"googleadservices.com", "blocked.com"}

	tests := []struct {
		name            string
		questionDomain  string
		privacySettings map[string]string
		expectBlocked   bool
		expectReasons   []string
	}{
		// --- exact match, varying query case --------------------------------
		{
			name:            "exact match, lowercase query (control)",
			questionDomain:  "googleadservices.com",
			privacySettings: map[string]string{},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1"},
		},
		{
			name:            "exact match, mixed-case query",
			questionDomain:  "GoOgLeAdSeRvIcEs.cOm",
			privacySettings: map[string]string{},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1"},
		},
		{
			name:            "exact match, uppercase query",
			questionDomain:  "GOOGLEADSERVICES.COM",
			privacySettings: map[string]string{},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1"},
		},
		// --- subdomain rule, varying query case -----------------------------
		{
			name:            "subdomain match, mixed-case query",
			questionDomain:  "SuB.BlOcKeD.cOm",
			privacySettings: map[string]string{SUBDOMAINS_RULE: RULE_BLOCK},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1", SUBDOMAINS_RULE},
		},
		{
			name:            "subdomain match, uppercase query",
			questionDomain:  "SUB.BLOCKED.COM",
			privacySettings: map[string]string{SUBDOMAINS_RULE: RULE_BLOCK},
			expectBlocked:   true,
			expectReasons:   []string{"blocklist: bl1", SUBDOMAINS_RULE},
		},
		// --- negative controls: normalisation must not over-block -----------
		{
			name:            "unrelated domain, mixed case, not blocked",
			questionDomain:  "ExAmPlE.oRg",
			privacySettings: map[string]string{},
			expectBlocked:   false,
		},
		{
			name:            "domain merely containing a blocked name is not blocked",
			questionDomain:  "NotGoogleAdServices.com",
			privacySettings: map[string]string{},
			expectBlocked:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileBlocklists", mock.Anything, "profile1").
				Return([]string{blocklistID}, nil)

			// Model Redis SISMEMBER: exact byte match on the stored (lowercase)
			// members, everything else is a miss. Specific expectations are
			// registered before the catch-all so testify matches them first.
			for _, member := range storedLowercase {
				mockCache.On("GetBlocklistEntry", mock.Anything, blocklistID, member).
					Return(true, nil)
			}
			mockCache.On("GetBlocklistEntry", mock.Anything, blocklistID, mock.Anything).
				Return(false, nil)

			fm := NewDomainFilter(&proxy.Proxy{}, mockCache, nil)

			msg := new(dns.Msg)
			msg.SetQuestion(tt.questionDomain+".", dns.TypeA)

			loggerFactory := logging.NewFactory(zerolog.DebugLevel)
			reqCtx := &requestcontext.RequestContext{
				ProfileId:       "profile1",
				PrivacySettings: tt.privacySettings,
				Logger:          loggerFactory.ForProfile("profile1", true),
			}

			result, err := fm.filterBlocklists(reqCtx, &proxy.DNSContext{Req: msg})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, TierBlocklists, result.Tier)
			if tt.expectBlocked {
				assert.Equal(t, model.DecisionBlock, result.Decision,
					"query %q must be blocked: stored members are lowercase and DNS "+
						"name comparison is case-insensitive (RFC 4343)", tt.questionDomain)
				assert.ElementsMatch(t, tt.expectReasons, result.Reasons)
			} else {
				assert.Equal(t, model.DecisionNone, result.Decision,
					"query %q must NOT be blocked", tt.questionDomain)
				assert.Nil(t, result.Reasons)
			}
		})
	}
}

// Section G's invariant covers every domain-matching sub-filter, so the siblings
// are asserted too.

// tableRef: #N7 (custom rules match case-insensitively — both sides lowercased)
func TestMatchDomainIsCaseInsensitive(t *testing.T) {
	fm := NewDomainFilter(&proxy.Proxy{}, new(mocks.Cache), nil)

	tests := []struct {
		name    string
		domain  string
		pattern string
		want    bool
	}{
		{"lowercase both (control)", "example.com", "example.com", true},
		{"mixed-case domain, lowercase pattern", "ExAmPlE.cOm", "example.com", true},
		{"lowercase domain, mixed-case pattern", "example.com", "ExAmPlE.cOm", true},
		{"uppercase both", "EXAMPLE.COM", "EXAMPLE.COM", true},
		{"mixed-case wildcard pattern", "SuB.ExAmPlE.cOm", "*.example.com", true},
		{"uppercase wildcard pattern", "sub.example.com", "*.EXAMPLE.COM", true},
		{"non-match stays non-match regardless of case", "NotExample.com", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fm.matchDomain(tt.domain, tt.pattern),
				"matchDomain(%q, %q): DNS name comparison is case-insensitive (RFC 4343)",
				tt.domain, tt.pattern)
		})
	}
}

// tableRef: #N8 (service-domain matching is case-insensitive)
//
// Calls the production filterServiceDomains rather than re-implementing its
// exact-then-parent walk, so a regression in that walk fails the test.
func TestServiceDomainMatchingIsCaseInsensitive(t *testing.T) {
	catalog := &servicescatalog.Catalog{
		Services: []servicescatalog.Service{
			{ID: "microsoft", Name: "Microsoft", Domains: []string{"microsoft.com"}},
		},
	}

	tests := []struct {
		qname  string
		expect model.Decision
	}{
		{"microsoft.com.", model.DecisionBlock},     // control
		{"MiCrOsOfT.cOm.", model.DecisionBlock},     // mixed case
		{"MICROSOFT.COM.", model.DecisionBlock},     // uppercase
		{"SuB.MiCrOsOfT.cOm.", model.DecisionBlock}, // parent walk, mixed case
		{"NotMicrosoft.com.", model.DecisionNone},   // must not over-block
	}

	for _, tt := range tests {
		t.Run(tt.qname, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileServicesBlocked", mock.Anything, "test-profile").
				Return([]string{"microsoft"}, nil)

			fm := &DomainFilter{Cache: mockCache, ServicesCatalog: staticCatalog{cat: catalog}}

			msg := new(dns.Msg)
			msg.SetQuestion(tt.qname, dns.TypeA)

			result, err := fm.filterServiceDomains(newTestReqCtx(t, "test-profile"),
				&proxy.DNSContext{Req: msg})

			assert.NoError(t, err)
			assert.Equal(t, tt.expect, result.Decision,
				"query %q: service-domain matching is case-insensitive (RFC 4343)", tt.qname)
		})
	}
}
