package filter

import (
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/servicescatalog"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterServiceDomains(t *testing.T) {
	catalog := &servicescatalog.Catalog{
		Services: []servicescatalog.Service{
			{
				ID:      "microsoft",
				Name:    "Microsoft",
				ASNs:    []uint{8075},
				Domains: []string{"microsoft.com", "office.com", "live.com"},
			},
			{
				ID:      "apple",
				Name:    "Apple",
				ASNs:    []uint{714},
				Domains: []string{"apple.com", "icloud.com"},
			},
			{
				ID:   "google",
				Name: "Google",
				ASNs: []uint{15169},
				// No domains — ASN-only service.
			},
		},
	}

	tests := []struct {
		name             string
		domain           string
		blockedIDs       []string
		expectedDecision model.Decision
		expectedService  string // "" if no match expected
	}{
		{
			name:             "exact match blocks",
			domain:           "microsoft.com.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "microsoft",
		},
		{
			name:             "subdomain match blocks",
			domain:           "www.microsoft.com.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "microsoft",
		},
		{
			name:             "deep subdomain match blocks",
			domain:           "login.live.com.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "microsoft",
		},
		{
			name:             "unrelated domain not blocked",
			domain:           "example.com.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionNone,
		},
		{
			name:             "service not in blocked list not matched",
			domain:           "apple.com.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionNone,
		},
		{
			name:             "apple exact match",
			domain:           "icloud.com.",
			blockedIDs:       []string{"apple"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "apple",
		},
		{
			name:             "multiple services blocked",
			domain:           "office.com.",
			blockedIDs:       []string{"apple", "microsoft"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "microsoft",
		},
		{
			name:             "service with no domains does not match",
			domain:           "google.com.",
			blockedIDs:       []string{"google"},
			expectedDecision: model.DecisionNone,
		},
		{
			name:             "empty blocked services",
			domain:           "microsoft.com.",
			blockedIDs:       []string{},
			expectedDecision: model.DecisionNone,
		},
		{
			name:             "case insensitive match",
			domain:           "WWW.Microsoft.COM.",
			blockedIDs:       []string{"microsoft"},
			expectedDecision: model.DecisionBlock,
			expectedService:  "microsoft",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := &DomainFilter{
				Cache:           mocks.NewCache(t),
				ServicesCatalog: staticCatalog{cat: catalog},
			}

			reqCtx := newTestReqCtx(t, "test-profile")
			reqCtx.BlockedServices = tt.blockedIDs
			msg := new(dns.Msg)
			msg.SetQuestion(tt.domain, dns.TypeA)
			dctx := &proxy.DNSContext{Req: msg}

			result, err := fm.filterServiceDomains(reqCtx, dctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, result.Decision)
			assert.Equal(t, TierServices, result.Tier)

			if tt.expectedService != "" {
				assert.Contains(t, result.Reasons, REASON_SERVICES)
				assert.Contains(t, result.Reasons, "service: "+tt.expectedService)
			}
		})
	}
}

func TestFilterServiceDomains_NilCatalog(t *testing.T) {
	fm := &DomainFilter{
		ServicesCatalog: nil,
	}
	reqCtx := newTestReqCtx(t, "test-profile")
	msg := new(dns.Msg)
	msg.SetQuestion("microsoft.com.", dns.TypeA)
	dctx := &proxy.DNSContext{Req: msg}

	result, err := fm.filterServiceDomains(reqCtx, dctx)
	require.NoError(t, err)
	assert.Equal(t, model.DecisionNone, result.Decision)
}

func TestFilterServiceDomains_CatalogError(t *testing.T) {
	fm := &DomainFilter{
		Cache:           mocks.NewCache(t),
		ServicesCatalog: staticCatalogErr{err: assert.AnError},
	}
	reqCtx := newTestReqCtx(t, "test-profile")
	reqCtx.BlockedServices = []string{"microsoft"}
	msg := new(dns.Msg)
	msg.SetQuestion("microsoft.com.", dns.TypeA)
	dctx := &proxy.DNSContext{Req: msg}

	result, err := fm.filterServiceDomains(reqCtx, dctx)
	require.NoError(t, err)
	assert.Equal(t, model.DecisionNone, result.Decision)
}

func TestDomainMapForServiceIDs(t *testing.T) {
	catalog := &servicescatalog.Catalog{
		Services: []servicescatalog.Service{
			{ID: "microsoft", Domains: []string{"microsoft.com", "office.com"}},
			{ID: "apple", Domains: []string{"apple.com"}},
			{ID: "google"}, // no domains
		},
	}

	t.Run("returns domains for requested services", func(t *testing.T) {
		m := catalog.DomainMapForServiceIDs([]string{"microsoft", "apple"})
		assert.Equal(t, "microsoft", m["microsoft.com"])
		assert.Equal(t, "microsoft", m["office.com"])
		assert.Equal(t, "apple", m["apple.com"])
		assert.Len(t, m, 3)
	})

	t.Run("ignores unknown service IDs", func(t *testing.T) {
		m := catalog.DomainMapForServiceIDs([]string{"nonexistent"})
		assert.Empty(t, m)
	})

	t.Run("service with no domains returns empty map", func(t *testing.T) {
		m := catalog.DomainMapForServiceIDs([]string{"google"})
		assert.Empty(t, m)
	})

	t.Run("nil catalog returns empty map", func(t *testing.T) {
		var cat *servicescatalog.Catalog
		m := cat.DomainMapForServiceIDs([]string{"microsoft"})
		assert.Empty(t, m)
	})
}
