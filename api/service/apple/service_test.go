package apple

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ivpn/dns/libs/deviceid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/urlshort"
)

type fakeCacheEntry struct {
	value     string
	expiresAt time.Time
	hasExpiry bool
}

type fakeCache struct {
	store map[string]fakeCacheEntry
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: make(map[string]fakeCacheEntry)}
}

func (f *fakeCache) Set(_ context.Context, key string, value any, expiration time.Duration) error {
	val := ""
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		val = fmt.Sprint(v)
	}
	entry := fakeCacheEntry{value: val}
	if expiration > 0 {
		entry.expiresAt = time.Now().Add(expiration)
		entry.hasExpiry = true
	}
	f.store[key] = entry
	return nil
}

func (f *fakeCache) Get(_ context.Context, key string) (string, error) {
	entry, ok := f.store[key]
	if !ok {
		return "", errors.New("not found")
	}
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		delete(f.store, key)
		return "", errors.New("not found")
	}
	return entry.value, nil
}

func (f *fakeCache) Del(_ context.Context, key string) error {
	delete(f.store, key)
	return nil
}

func (f *fakeCache) Incr(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}

// The following methods satisfy the cache.Cache interface but are unused in these tests.
func (f *fakeCache) AddBlocklist(_ context.Context, _ string, _ []byte) error { return nil }
func (f *fakeCache) CreateOrUpdateProfileSettings(_ context.Context, _ *model.ProfileSettings, _ bool) error {
	return nil
}
func (f *fakeCache) AddCustomRule(_ context.Context, _ string, _ *model.CustomRule) error { return nil }
func (f *fakeCache) RemoveCustomRule(_ context.Context, _ string, _ string) error         { return nil }
func (f *fakeCache) DeleteProfileSettings(_ context.Context, _ string) error              { return nil }
func (f *fakeCache) SetTOTPSecret(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (f *fakeCache) GetTOTPSecret(_ context.Context, _ string) (string, error) { return "", nil }
func (f *fakeCache) AppendBlocklistsToProfileSettings(_ context.Context, _ string, _ ...string) error {
	return nil
}
func (f *fakeCache) RemoveBlocklistsFromProfileSettings(_ context.Context, _ string, _ ...string) error {
	return nil
}
func (f *fakeCache) AddSubscription(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (f *fakeCache) GetSubscription(_ context.Context, _ string) (string, error) { return "", nil }
func (f *fakeCache) RemoveSubscription(_ context.Context, _ string) error        { return nil }

func TestAppleService_validate(t *testing.T) {
	tests := []struct {
		name    string
		req     requests.MobileConfigReq
		wantErr bool
		errMsg  string
		want    requests.MobileConfigReq
	}{
		{
			name: "Valid basic request",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
			},
			wantErr: false,
			want: requests.MobileConfigReq{
				ProfileId:          "test-profile",
				AdvancedOptionsReq: nil,
			},
		},
		{
			name: "Invalid encryption type",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					EncryptionType: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid encryption type: invalid",
		},
		{
			name: "Empty profile_id",
			req: requests.MobileConfigReq{
				ProfileId: "",
			},
			wantErr: true,
			errMsg:  "profile_id is required",
		},
		{
			name: "Profile ID too long",
			req: requests.MobileConfigReq{
				ProfileId: strings.Repeat("a", maxProfileIdLength+1),
			},
			wantErr: true,
			errMsg:  "profile_id exceeds maximum length",
		},
		{
			name:    "Profile not provided",
			req:     requests.MobileConfigReq{},
			wantErr: true,
			errMsg:  "profile_id is required",
		},
		// {
		// 	name: "Domain too long - should be skipped",
		// 	req: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: strings.Repeat("a", maxDomainLength+1),
		// 		},
		// 	},
		// 	wantErr: false,
		// 	want: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: "",
		// 		},
		// 	},
		// },
		// {
		// 	name: "Valid excluded domains",
		// 	req: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: "example.com,test.com",
		// 		},
		// 	},
		// 	wantErr: false,
		// 	want: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: "example.com,test.com",
		// 		},
		// 	},
		// },
		// {
		// 	name: "Too many excluded domains",
		// 	req: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: strings.Repeat("domain.com,", maxDomainsCount+1),
		// 		},
		// 	},
		// 	wantErr: true,
		// 	errMsg:  "too many excluded domains",
		// },
		// {
		// 	name: "Invalid domain format",
		// 	req: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: "invalid..domain.com,example.com",
		// 		},
		// 	},
		// 	wantErr: false,
		// 	want: requests.MobileConfigReq{
		// 		ProfileId: "test-profile",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: "example.com",
		// 		},
		// 	},
		// },
		{
			name: "Valid WiFi networks",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					ExcludedWifiNetworks: "Public WiFi,Cafe Network",
				},
			},
			wantErr: false,
			want: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					ExcludedWifiNetworks: "Public WiFi,Cafe Network",
				},
			},
		},
		{
			name: "Too many excluded WiFi networks",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					ExcludedWifiNetworks: strings.Repeat("WiFi,", maxNetworksCount+1),
				},
			},
			wantErr: true,
			errMsg:  "too many excluded WiFi networks",
		},
		{
			name: "WiFi network name too long",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					ExcludedWifiNetworks: strings.Repeat("a", maxNetworkLength+1),
				},
			},
			wantErr: false,
			want: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					ExcludedWifiNetworks: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			cfg := &config.Config{
				Server: &config.ServerConfig{
					DnsDomain:      "dns.com",
					FrontendDomain: "frontend.com",
				},
				Service: &config.ServiceConfig{},
			}
			shortener := urlshort.NewURLShortener()
			service := NewAppleService(cfg, newFakeCache(), shortener)

			got, err := service.validate(tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, *got)
		})
	}

}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No special characters",
			input: "Normal text",
			want:  "Normal text",
		},
		{
			name:  "Ampersand",
			input: "WiFi & Network",
			want:  "WiFi &amp; Network",
		},
		{
			name:  "Less than and greater than",
			input: "<Network>",
			want:  "&lt;Network&gt;",
		},
		{
			name:  "Quotes",
			input: "\"Network's Name\"",
			want:  "&quot;Network&apos;s Name&quot;",
		},
		{
			name:  "Multiple special characters",
			input: "WiFi & <Network's> \"Test\"",
			want:  "WiFi &amp; &lt;Network&apos;s&gt; &quot;Test&quot;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeXML(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateMobileConfig_Security(t *testing.T) {
	tests := []struct {
		name         string
		req          requests.MobileConfigReq
		genLink      bool
		wantErr      bool
		errMsg       string
		expectedLink string
	}{
		{
			name: "XML Injection Attempt",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					EncryptionType:       "https",
					ExcludedWifiNetworks: `<script>alert("xss")</script>`,
				},
			},
			genLink: false,
			wantErr: false, // Should not error but escape the XML
		},
		{
			name: "Profile ID too long",
			req: requests.MobileConfigReq{
				ProfileId: strings.Repeat("a", maxProfileIdLength+1),
			},
			genLink: false,
			wantErr: true,
			errMsg:  "profile_id exceeds maximum length",
		},
		// {
		// 	name: "Large Number of Domains",
		// 	req: requests.MobileConfigReq{
		// 		ProfileId: "test",
		// 		AdvancedOptionsReq: &requests.AdvancedOptionsReq{
		// 			ExcludedDomains: strings.Repeat("domain.com,", maxDomainsCount+1),
		// 		},
		// 	},
		// 	genLink: false,
		// 	wantErr: true,
		// 	errMsg:  "too many excluded domains",
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Server: &config.ServerConfig{
					DnsDomain:      "dns.com",
					FrontendDomain: "frontend.com",
				},
				Service: &config.ServiceConfig{
					MobileConfigCertPath:       "../../../certs/certificate.pem",
					MobileConfigPrivateKeyPath: "../../../certs/private_key.pem",
				},
			}
			shortener := urlshort.NewURLShortener()
			service := NewAppleService(cfg, newFakeCache(), shortener)

			ctx := context.Background()

			// Execute the function being tested
			data, link, err := service.GenerateMobileConfig(ctx, tt.req, "account_id", tt.genLink)

			// Validate results
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Empty(t, data)
				assert.Empty(t, link)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, data)

				if tt.genLink {
					assert.Equal(t, tt.expectedLink, link)
				} else {
					assert.Empty(t, link)
				}

				// For the XML injection case, verify the XML doesn't contain unescaped script tags
				if tt.name == "XML Injection Attempt" {
					assert.NotContains(t, string(data), "<script>")
					assert.NotContains(t, string(data), "alert(\"xss\")")
				}
			}
		})
	}
}

func TestGenerateMobileConfig_StoresPayloadInCache(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:      "dns.com",
			FrontendDomain: "frontend.com",
		},
		Service: &config.ServiceConfig{
			MobileConfigCertPath:       "../../../certs/certificate.pem",
			MobileConfigPrivateKeyPath: "../../../certs/private_key.pem",
		},
	}
	cache := newFakeCache()
	shortener := urlshort.NewURLShortener(urlshort.WithDefaultTTL(2 * time.Minute))
	service := NewAppleService(cfg, cache, shortener)

	req := requests.MobileConfigReq{ProfileId: "profile1", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}
	ctx := context.Background()

	_, link, err := service.GenerateMobileConfig(ctx, req, "acc", true)
	require.NoError(t, err)
	require.NotEmpty(t, link)

	token := strings.TrimPrefix(link, cfg.Server.FrontendDomain+"/short/")
	cacheKey := MobileConfigCacheKey(token)
	cached, err := cache.Get(ctx, cacheKey)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(cached, req.ProfileId+"|"))
	entry := cache.store[cacheKey]
	assert.True(t, entry.hasExpiry)
	assert.True(t, entry.expiresAt.After(time.Now()))
}

func TestGenerateMobileConfig_WifiNetworkSlicesNilBehavior(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:      "dns.com",
			FrontendDomain: "frontend.com",
		},
		Service: &config.ServiceConfig{
			MobileConfigCertPath:       "../../../certs/certificate.pem",
			MobileConfigPrivateKeyPath: "../../../certs/private_key.pem",
		},
	}
	shortener := urlshort.NewURLShortener()
	service := NewAppleService(cfg, newFakeCache(), shortener)

	ctx := context.Background()

	// Case 1: No Excluded networks provided -> slices should be nil
	req1 := requests.MobileConfigReq{ProfileId: "p1", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}
	data, _, err := service.GenerateMobileConfig(ctx, req1, "acc", false)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// We need to access internal struct; regenerate mobile config through validate/newMobileConfig path directly
	validated, err := service.validate(req1)
	require.NoError(t, err)
	mc1, err := service.newMobileConfig(ctx, *validated)
	require.NoError(t, err)
	assert.Nil(t, mc1.AdvancedOptions.ExcludedWifiNetworks, "ExcludedWifiNetworks should be nil when not provided")

	// Case 2: Excluded networks provided -> slice should be non-nil with entries
	exc := "Cafe"
	req2 := requests.MobileConfigReq{ProfileId: "p2", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https", ExcludedWifiNetworks: exc}}
	validated2, err := service.validate(req2)
	require.NoError(t, err)
	mc2, err := service.newMobileConfig(ctx, *validated2)
	require.NoError(t, err)
	assert.NotNil(t, mc2.AdvancedOptions.ExcludedWifiNetworks)
	assert.Equal(t, []string{"Cafe"}, mc2.AdvancedOptions.ExcludedWifiNetworks)
}

func TestGenerateMobileConfig_DeviceID(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:      "dns.com",
			FrontendDomain: "frontend.com",
		},
		Service: &config.ServiceConfig{
			MobileConfigCertPath:       "../../../certs/certificate.pem",
			MobileConfigPrivateKeyPath: "../../../certs/private_key.pem",
		},
	}
	shortener := urlshort.NewURLShortener()
	service := NewAppleService(cfg, newFakeCache(), shortener)

	ctx := context.Background()

	// Helper to get unsigned template output
	render := func(r requests.MobileConfigReq) string {
		validated, err := service.validate(r)
		require.NoError(t, err)
		mc, err := service.newMobileConfig(ctx, *validated)
		require.NoError(t, err)
		var buf bytes.Buffer
		err = mobileTemplate.Execute(&buf, mc)
		require.NoError(t, err)
		return buf.String()
	}

	// HTTPS variant with device id (should appear in path, URL encoded with %20)
	reqHTTPS := requests.MobileConfigReq{ProfileId: "prof123", DeviceId: "My Phone 01", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}
	outHTTPS := render(reqHTTPS)
	assert.Contains(t, outHTTPS, "/dns-query/prof123/My%20Phone%2001")

	// TLS variant with device id (should appear in ServerName label encoded with -- for spaces)
	reqTLS := requests.MobileConfigReq{ProfileId: "prof123", DeviceId: "My Phone 01", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "tls"}}
	outTLS := render(reqTLS)
	assert.Contains(t, outTLS, "My--Phone--01-prof123.dns.com")

	// Normalization: disallowed chars stripped and length truncated
	longRaw := "@@@VERY*LONG*DEVICE*NAME*WITH*CHARS 1234567890" // will strip symbols and truncate
	reqNorm := requests.MobileConfigReq{ProfileId: "prof123", DeviceId: longRaw, AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}
	outNorm := render(reqNorm)
	expectedLogical := deviceid.Normalize(longRaw)
	assert.Contains(t, outNorm, "/dns-query/prof123/"+deviceid.EncodeURL(expectedLogical))
}
