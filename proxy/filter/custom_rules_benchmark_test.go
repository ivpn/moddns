package filter

import (
	"fmt"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestData(size int) ([]string, map[string]map[string]string) {
	hashes := make([]string, size)
	rulesMap := make(map[string]map[string]string)

	// Create test data with different patterns
	for i := 0; i < size; i++ {
		hash := fmt.Sprintf("hash%d", i)
		hashes[i] = hash
		rulesMap[hash] = map[string]string{
			"value":  fmt.Sprintf("domain%d.com", i),
			"action": ACTION_BLOCK,
			"syntax": "domain",
		}
	}

	// Add some wildcard patterns
	if size > 0 {
		rulesMap["hash0"]["value"] = "*.example.com"
	}
	return hashes, rulesMap
}

func BenchmarkFilterCustomRules(b *testing.B) {
	testCases := []struct {
		name      string
		rulesSize int
		domain    string
	}{
		{"Small_Rules_Set", 10, "example.com"},
		{"Medium_Rules_Set", 100, "example.com"},
		{"Large_Rules_Set", 1000, "example.com"},
		{"Wildcard_Match_Start", 100, "sub.example.com"},
		{"Wildcard_Match_End", 100, "test.com"},
		{"No_Match", 100, "nomatch.com"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mockCache := &mocks.Cache{}
			filterManager := &DomainFilter{Cache: mockCache}

			hashes, rulesMap := setupTestData(tc.rulesSize)

			// Setup mock expectations
			mockCache.On("GetCustomRulesHashes", mock.Anything, mock.Anything).Return(hashes, nil)
			for hash, rule := range rulesMap {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).Return(rule, nil)
			}

			// Create DNS request context
			msg := new(dns.Msg)
			msg.SetQuestion(tc.domain+".", dns.TypeA)
			dnsCtx := &proxy.DNSContext{
				Req: msg,
			}
			loggerFactory := logging.NewFactory(zerolog.Disabled)
			reqCtx := &requestcontext.RequestContext{
				ProfileId: "test-profile",
				Logger:    loggerFactory.ForProfile("test-profile", true),
			}

			// Reset timer and run benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := filterManager.filterCustomRules(reqCtx, dnsCtx)
				require.NoError(b, err)
				require.NotNil(b, result)
			}
		})
	}
}

func BenchmarkDomainMatching(b *testing.B) {
	testCases := []struct {
		name    string
		domain  string
		pattern string
	}{
		{"Exact_Match", "example.com", "example.com"},
		{"Wildcard_Prefix", "sub.example.com", "*.example.com"},
		{"Wildcard_Suffix", "test.com", "test.*"},
		{"Multiple_Wildcards", "sub.example.com", "*.example.*"},
		{"No_Match", "nomatch.com", "example.com"},
	}

	mockCache := new(mocks.Cache)
	proxy := &proxy.Proxy{}
	fm := NewDomainFilter(proxy, mockCache)

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fm.matchDomain(tc.domain, tc.pattern)
			}
		})
	}
}
