package apple

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/libs/deviceid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/libs/urlshort"

	"github.com/ivpn/dns/api/service/apple/pkcs7"
)

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
		{
			name: "Device ID normalized",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				DeviceId:  "My@Device!",
			},
			wantErr: false,
			want: requests.MobileConfigReq{
				ProfileId:          "test-profile",
				DeviceId:           deviceid.Normalize("My@Device!"),
				AdvancedOptionsReq: nil,
			},
		},
		{
			name: "Trims and escapes WiFi names",
			req: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					EncryptionType:       "https",
					ExcludedWifiNetworks: " Cafe & WiFi , Another ",
				},
			},
			wantErr: false,
			want: requests.MobileConfigReq{
				ProfileId: "test-profile",
				AdvancedOptionsReq: &requests.AdvancedOptionsReq{
					EncryptionType:       "https",
					ExcludedWifiNetworks: "Cafe &amp; WiFi,Another",
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
			service := NewAppleService(cfg, mocks.NewCachecache(t), shortener)

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
			service := NewAppleService(cfg, mocks.NewCachecache(t), shortener)

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
	mockCache := mocks.NewCachecache(t)
	var capturedKey string
	var capturedVal any
	var capturedTTL time.Duration
	mockCache.
		On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).
		Run(func(args mock.Arguments) {
			capturedKey = args.Get(1).(string)
			capturedVal = args.Get(2)
			capturedTTL = args.Get(3).(time.Duration)
		}).
		Return(nil)

	shortener := urlshort.NewURLShortener(urlshort.WithDefaultTTL(2 * time.Minute))
	service := NewAppleService(cfg, mockCache, shortener)

	req := requests.MobileConfigReq{ProfileId: "profile1", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}
	ctx := context.Background()

	_, link, err := service.GenerateMobileConfig(ctx, req, "acc", true)
	require.NoError(t, err)
	require.NotEmpty(t, link)

	token := strings.TrimPrefix(link, cfg.Server.FrontendDomain+"/short/")
	require.Equal(t, MobileConfigCacheKey(token), capturedKey)

	valBytes, ok := capturedVal.([]byte)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(string(valBytes), req.ProfileId+"|"))
	assert.Equal(t, 2*time.Minute, capturedTTL)

	mockCache.AssertExpectations(t)
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
	service := NewAppleService(cfg, mocks.NewCachecache(t), shortener)

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
	service := NewAppleService(cfg, mocks.NewCachecache(t), shortener)

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

func TestNewMobileConfig_DefaultsAndIdentifiers(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			DnsDomain:       "dns.example.com",
			ServerAddresses: []string{"10.0.0.1", "10.0.0.2"},
			FrontendDomain:  "frontend.example.com",
		},
		Service: &config.ServiceConfig{},
	}
	shortener := urlshort.NewURLShortener()
	service := NewAppleService(cfg, mocks.NewCachecache(t), shortener)

	ctx := context.Background()
	rawDevice := "Device-One! 123"
	req := requests.MobileConfigReq{ProfileId: "profile-1", DeviceId: rawDevice, AdvancedOptionsReq: &requests.AdvancedOptionsReq{}}

	validated, err := service.validate(req)
	require.NoError(t, err)

	mobileCfg, err := service.newMobileConfig(ctx, *validated)
	require.NoError(t, err)

	expectedDevice := deviceid.Normalize(rawDevice)

	assert.Equal(t, "profile-1", mobileCfg.ProfileId)
	assert.Equal(t, expectedDevice, mobileCfg.DeviceId)
	assert.Equal(t, deviceid.EncodeLabel(expectedDevice), mobileCfg.DeviceLabelEncoded)
	assert.Equal(t, "https", mobileCfg.EncryptionType)
	assert.True(t, mobileCfg.SignConfigurationProfile)
	assert.Equal(t, cfg.Server.ServerAddresses, mobileCfg.ServerAddresses)
	assert.Equal(t, cfg.Server.DnsDomain, mobileCfg.ServerDomain)
	assert.Equal(t, "com.apple.dnsSettings.managed", mobileCfg.DNSSettingsPayloadType)
	assert.True(t, strings.HasPrefix(mobileCfg.DNSSettingsPayloadIdentifier, "com.example.dns."))
	assert.Nil(t, mobileCfg.ExcludedWifiNetworks)
	assert.NotEqual(t, uuid.Nil, mobileCfg.PayloadIdentifier)
	assert.NotEqual(t, uuid.Nil, mobileCfg.PayloadUUID)
	assert.NotEqual(t, uuid.Nil, mobileCfg.DNSSettingsPayloadUUID)
}

func TestGenerateMobileConfig_NoLinkSkipsCache(t *testing.T) {
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
	mockCache := mocks.NewCachecache(t)
	shortener := urlshort.NewURLShortener()
	service := NewAppleService(cfg, mockCache, shortener)

	ctx := context.Background()
	req := requests.MobileConfigReq{ProfileId: "profile1", AdvancedOptionsReq: &requests.AdvancedOptionsReq{EncryptionType: "https"}}

	data, link, err := service.GenerateMobileConfig(ctx, req, "acc", false)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Empty(t, link)

	mockCache.AssertNotCalled(t, "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSign_ReturnsErrorOnMissingFiles(t *testing.T) {
	service := &AppleService{
		CertPath:       "/tmp/does-not-exist.cert",
		PrivateKeyPath: "/tmp/does-not-exist.key",
	}

	var buf bytes.Buffer
	buf.WriteString("payload")

	_, err := service.sign(buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file")
}

func TestSign_ECDSAKey_Works(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	notBefore := time.Now().Add(-time.Minute)
	notAfter := time.Now().Add(time.Hour)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-mobileconfig-signer"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NotEmpty(t, certPEM)

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	require.NotEmpty(t, keyPEM)

	certFile, err := os.CreateTemp("", "mobileconfig-cert-*.pem")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM)
	require.NoError(t, err)
	require.NoError(t, certFile.Close())

	keyFile, err := os.CreateTemp("", "mobileconfig-key-*.pem")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM)
	require.NoError(t, err)
	require.NoError(t, keyFile.Close())

	service := &AppleService{
		CertPath:       certFile.Name(),
		PrivateKeyPath: keyFile.Name(),
	}

	var buf bytes.Buffer
	buf.WriteString("payload")

	signed, err := service.sign(buf)
	require.NoError(t, err)
	require.NotEmpty(t, signed)

	p7, err := pkcs7.Parse(signed)
	require.NoError(t, err)
	require.NoError(t, p7.Verify())
}
