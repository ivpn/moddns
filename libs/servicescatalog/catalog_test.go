package servicescatalog

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_DomainRules(t *testing.T) {
	tests := []struct {
		name    string
		cat     *Catalog
		wantErr string
	}{
		{
			name: "valid catalog with domains",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Domains: []string{"example.com", "foo.com"}},
				{ID: "b", Name: "B", Domains: []string{"bar.com"}},
			}},
		},
		{
			name: "uppercase domain rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Domains: []string{"Example.com"}},
			}},
			wantErr: "must be lowercase",
		},
		{
			name: "trailing dot rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Domains: []string{"example.com."}},
			}},
			wantErr: "trailing dot",
		},
		{
			name: "duplicate domain across services rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Domains: []string{"example.com"}},
				{ID: "b", Name: "B", Domains: []string{"example.com"}},
			}},
			wantErr: "already used by",
		},
		{
			name: "no domains is valid",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", ASNs: []uint{1}},
			}},
		},
		{
			name: "alias is valid and does not need its own domains",
			cat: &Catalog{Services: []Service{
				{ID: "tiktok", Name: "TikTok", Domains: []string{"tiktok.com"}, Aliases: []string{"tiktok2"}},
			}},
		},
		{
			name: "alias duplicating another service id rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A"},
				{ID: "b", Name: "B", Aliases: []string{"a"}},
			}},
			wantErr: "duplicates an existing service id or alias",
		},
		{
			name: "service id duplicating an earlier alias rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Aliases: []string{"legacy"}},
				{ID: "legacy", Name: "Legacy"},
			}},
			wantErr: "duplicate service id",
		},
		{
			name: "empty alias rejected",
			cat: &Catalog{Services: []Service{
				{ID: "a", Name: "A", Aliases: []string{""}},
			}},
			wantErr: "alias must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cat)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDomainMapForServiceIDs(t *testing.T) {
	cat := &Catalog{Services: []Service{
		{ID: "ms", Name: "Microsoft", Domains: []string{"microsoft.com", "office.com"}},
		{ID: "apple", Name: "Apple", Domains: []string{"apple.com"}},
		{ID: "google", Name: "Google"},
	}}

	m := cat.DomainMapForServiceIDs([]string{"ms", "apple"})
	assert.Equal(t, "ms", m["microsoft.com"])
	assert.Equal(t, "ms", m["office.com"])
	assert.Equal(t, "apple", m["apple.com"])
	assert.Len(t, m, 3)

	m = cat.DomainMapForServiceIDs([]string{"google"})
	assert.Empty(t, m)

	m = cat.DomainMapForServiceIDs([]string{"unknown"})
	assert.Empty(t, m)
}

func TestFindByID_Aliases(t *testing.T) {
	cat := &Catalog{Services: []Service{
		{ID: "tiktok", Name: "TikTok", Domains: []string{"tiktok.com", "tiktokcdn.com"}, Aliases: []string{"tiktok2"}},
	}}

	// Canonical ID resolves.
	svc, ok := cat.FindByID("tiktok")
	require.True(t, ok)
	assert.Equal(t, "tiktok", svc.ID)

	// Alias resolves to the same service (the migration window: profiles still
	// referencing the old ID keep blocking).
	svc, ok = cat.FindByID("tiktok2")
	require.True(t, ok)
	assert.Equal(t, "tiktok", svc.ID)

	// Both the canonical ID and the alias yield the service's domains, so
	// domain-phase blocking never fails open during the rename.
	assert.Equal(t, "tiktok", cat.DomainMapForServiceIDs([]string{"tiktok2"})["tiktok.com"])
	assert.Equal(t, "tiktok", cat.DomainMapForServiceIDs([]string{"tiktok"})["tiktok.com"])

	_, ok = cat.FindByID("nope")
	assert.False(t, ok)
}

// TestFindByID_ConcurrentFirstLookup exercises the lazy sync.Once index build
// under concurrent first-lookups, mirroring how the proxy reads a freshly
// loaded catalog from many goroutines. Run with -race to catch a data race in
// the index construction.
func TestFindByID_ConcurrentFirstLookup(t *testing.T) {
	cat := &Catalog{Services: []Service{
		{ID: "tiktok", Name: "TikTok", Domains: []string{"tiktok.com"}, Aliases: []string{"tiktok2"}},
		{ID: "netflix", Name: "Netflix", Domains: []string{"netflix.com"}},
	}}

	var wg sync.WaitGroup
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc, ok := cat.FindByID("tiktok2")
			assert.True(t, ok)
			assert.Equal(t, "tiktok", svc.ID)
			_, ok = cat.FindByID("missing")
			assert.False(t, ok)
		}()
	}
	wg.Wait()
}

// buildBenchCatalog builds a catalog roughly the size of the production one.
func buildBenchCatalog() *Catalog {
	svcs := make([]Service, 0, 16)
	for i := 0; i < 16; i++ {
		svcs = append(svcs, Service{
			ID:      fmt.Sprintf("svc%d", i),
			Name:    fmt.Sprintf("Service %d", i),
			Domains: []string{fmt.Sprintf("svc%d.com", i)},
		})
	}
	// Alias on the last service, matching the transitional rename shape.
	svcs[len(svcs)-1].Aliases = []string{"svc-legacy"}
	return &Catalog{Services: svcs}
}

func BenchmarkFindByID(b *testing.B) {
	cat := buildBenchCatalog()
	// Warm the index so we measure steady-state lookups, not the one-time build.
	cat.FindByID("svc0")

	b.Run("hit_first", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cat.FindByID("svc0")
		}
	})
	b.Run("hit_last", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cat.FindByID("svc15")
		}
	})
	b.Run("hit_alias", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cat.FindByID("svc-legacy")
		}
	})
	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cat.FindByID("nope")
		}
	})
}
