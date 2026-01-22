package apple

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/apple/pkcs7"
	"github.com/ivpn/dns/libs/deviceid"
	"github.com/ivpn/dns/libs/urlshort"
)

const mobileConfigCacheKeyPrefix = "mobileconfig:"

var mobileTemplate = template.Must(template.New("mobileconfig").Funcs(template.FuncMap{
	// urlquery escapes a string for inclusion in a URL path segment using %20 for spaces.
	"urlquery": deviceid.EncodeURL,
}).Parse(mobileconfigTemplate))

// device id helpers now sourced from shared libs/deviceid package

// TODO: verify default domains
// var DefaultExcludedDomains = []string{
// 	"*.local",
// 	"*.lan",
// 	"epdg.epc.aptg.com.tw",
// 	"epdg.epc.att.net",
// 	"epdg.mobileone.net.sg",
// 	"primgw.vowifina.spcsdns.net",
// 	"swu-loopback-epdg.qualcomm.com",
// 	"vowifi.jio.com",
// 	"wlan.three.com.hk",
// 	"wo.vzwwo.com",
// 	"epdg.epc.*.pub.3gppnetwork.org",
// 	"ss.epdg.epc.*.pub.3gppnetwork.org",
// 	"dengon.docomo.ne.jp",
// 	"dlinkap",
// 	"dlinkrouter",
// 	"edimax.setup",
// 	"fritz.box",

// 	// voicemail mobile domains
// 	"dav.orange.fr",
// 	"vvm.mobistar.be",
// 	"msg.t-mobile.com",
// 	"tma.vvm.mone.pan-net.eu",
// 	"vvm.ee.co.uk",
// }

// Constants for validation
const (
	// maxDomainLength    = 255
	maxNetworkLength = 64
	// maxDomainsCount    = 100
	maxNetworksCount   = 50
	maxProfileIdLength = 64
)

type AppleService struct {
	DnsServerDomain string
	ServerAddresses []string
	FrontendDomain  string
	PrivateKeyPath  string
	CertPath        string
	Shortener       *urlshort.URLShortener
	Cache           cache.Cache
}

func NewAppleService(cfg *config.Config, cache cache.Cache, shortener *urlshort.URLShortener) *AppleService {
	return &AppleService{
		DnsServerDomain: cfg.Server.DnsDomain,
		ServerAddresses: cfg.Server.ServerAddresses,
		FrontendDomain:  cfg.Server.FrontendDomain,
		PrivateKeyPath:  cfg.Service.MobileConfigPrivateKeyPath,
		CertPath:        cfg.Service.MobileConfigCertPath,
		Shortener:       shortener,
		Cache:           cache,
	}
}

func MobileConfigCacheKey(token string) string {
	return mobileConfigCacheKeyPrefix + token
}

func (a *AppleService) GenerateMobileConfig(ctx context.Context, req requests.MobileConfigReq, accountId string, genLink bool) (data []byte, link string, err error) {
	// Validate and sanitize inputs first
	validatedReq, err := a.validate(req)
	if err != nil {
		log.Warn().Err(err).Msg("Validation failed for mobileconfig request")
		return nil, "", err
	}

	mobilecfg, err := a.newMobileConfig(ctx, *validatedReq)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	err = mobileTemplate.Execute(&buf, mobilecfg)
	if err != nil {
		return nil, "", err
	}

	// Remove empty lines
	var cleanBuf bytes.Buffer
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			cleanBuf.WriteString(line + "\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	buf = cleanBuf

	if mobilecfg.SignConfigurationProfile {
		data, err = a.sign(buf)
		if err != nil {
			return nil, "", err
		}
	} else {
		data = buf.Bytes()
	}
	// in this case it does not matter what's the link since we need only data later on
	origURL := a.FrontendDomain + "/" + uuid.NewString()
	if genLink {
		// Prepend profile_id to the data for retrieval later
		// Format: profile_id|mobileconfig_data
		dataWithMetadata := append([]byte(validatedReq.ProfileId+"|"), data...)

		urlToken, entry, err := a.Shortener.ShortenWithData(origURL, dataWithMetadata)
		if err != nil {
			return nil, "", err
		}

		if err := a.Cache.Set(ctx, MobileConfigCacheKey(urlToken), dataWithMetadata, entry.TTL); err != nil {
			return nil, "", err
		}

		link = fmt.Sprintf("%s/short/%s", a.FrontendDomain, urlToken)
		log.Info().Str("link", link).Msg("Generated short link for mobileconfig")
	}
	return data, link, nil
}

// newMobileConfig returns a new MobileConfig struct with default values.
func (a *AppleService) newMobileConfig(ctx context.Context, req requests.MobileConfigReq) (model.MobileConfig, error) {
	mobilecfg := model.MobileConfig{}
	if req.ProfileId == "" {
		return mobilecfg, fmt.Errorf("profile_id is required")
	}
	mobilecfg.ProfileId = req.ProfileId
	// DeviceId already normalized in validate(); just propagate
	mobilecfg.DeviceId = req.DeviceId
	if mobilecfg.DeviceId != "" {
		mobilecfg.DeviceLabelEncoded = deviceid.EncodeLabel(mobilecfg.DeviceId)
	}
	if req.AdvancedOptionsReq == nil {
		mobilecfg.AdvancedOptions = model.AdvancedOptions{
			// PayloadRemovalDisallowed: false,
			SignConfigurationProfile: true, // all config profiles are signed by default
		}
	} else {
		if req.EncryptionType != "" {
			mobilecfg.EncryptionType = req.EncryptionType
		} else {
			mobilecfg.EncryptionType = "https"
		}
		// if req.AdvancedOptionsReq.ExcludedDomains != "" {
		// 	mobilecfg.AdvancedOptions.ExcludedDomains = strings.Split(req.AdvancedOptionsReq.ExcludedDomains, ",")
		// }
		// TODO: verify all default domains
		// else {
		// 	mobilecfg.AdvancedOptions.ExcludedDomains = DefaultExcludedDomains
		// }
		if req.ExcludedWifiNetworks != "" {
			mobilecfg.ExcludedWifiNetworks = strings.Split(req.ExcludedWifiNetworks, ",")
		}
		// if req.AdvancedOptionsReq.PayloadRemovalDisallowed != nil {
		// 	mobilecfg.AdvancedOptions.PayloadRemovalDisallowed = *req.AdvancedOptionsReq.PayloadRemovalDisallowed
		// }
		// if req.AdvancedOptionsReq.SignConfigurationProfile != nil {
		// 	mobilecfg.AdvancedOptions.SignConfigurationProfile = *req.AdvancedOptionsReq.SignConfigurationProfile
		// }
		mobilecfg.SignConfigurationProfile = true // all config profiles are signed by default
	}

	mobilecfg.PayloadIdentifier = uuid.New()
	mobilecfg.ContentIdentifier = uuid.New()
	mobilecfg.PayloadUUID = uuid.New()

	mobilecfg.ServerAddresses = a.ServerAddresses

	mobilecfg.ServerDomain = a.DnsServerDomain
	parts := strings.Split(a.DnsServerDomain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	serverAddressReversed := strings.Join(parts, ".")
	mobilecfg.DNSSettingsPayloadType = "com.apple.dnsSettings.managed"
	dnsSettingsPayloadIdUUID := uuid.New()
	mobilecfg.DNSSettingsPayloadIdentifier = fmt.Sprintf("%s.%s", serverAddressReversed, dnsSettingsPayloadIdUUID)
	mobilecfg.DNSSettingsPayloadUUID = uuid.New()

	// Ensure slices are nil (not just zero-length) when unused so template conditionals
	// like `{{ if .ExcludedWifiNetworks }}` behave predictably and we don't emit
	// empty XML blocks. This is defensive if future changes assign empty slices.
	if len(mobilecfg.ExcludedWifiNetworks) == 0 {
		mobilecfg.ExcludedWifiNetworks = nil
	}
	return mobilecfg, nil
}

func (a *AppleService) sign(configFile bytes.Buffer) ([]byte, error) {
	certData, err := os.ReadFile(a.CertPath)
	if err != nil {
		return nil, fmt.Errorf("read signing certificate: %w", err)
	}
	certs, err := parsePEMCertificates(certData)
	if err != nil {
		return nil, fmt.Errorf("parse signing certificate(s): %w", err)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found in %s", a.CertPath)
	}
	signerCert := certs[0]

	keyFile, err := os.ReadFile(a.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read signing private key: %w", err)
	}
	privateKey, err := parsePEMRSAPrivateKey(keyFile)
	if err != nil {
		return nil, fmt.Errorf("parse signing private key: %w", err)
	}

	// Create a new PKCS#7 signer
	p7, err := pkcs7.NewSignedData(configFile.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error creating signed data: %w", err)
	}

	// Add the signer
	if err := p7.AddSigner(signerCert, privateKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("error adding signer: %w", err)
	}

	// Embed the parent certificate(s) in the SignedData so clients (notably iOS)
	// can validate the chain without fetching intermediates.
	for _, cert := range certs[1:] {
		p7.AddCertificate(cert)
	}

	// Finish the signing process
	signedData, err := p7.Finish()
	if err != nil {
		return nil, fmt.Errorf("error finishing signing: %w", err)
	}
	return signedData, nil
}

func parsePEMCertificates(pemData []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate

	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("no PEM certificates found")
	}
	return certs, nil
}

func parsePEMRSAPrivateKey(pemData []byte) (crypto.PrivateKey, error) {
	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if !strings.Contains(block.Type, "PRIVATE KEY") {
			continue
		}
		if x509.IsEncryptedPEMBlock(block) { //nolint:staticcheck // legacy PEM encryption detection; we intentionally reject encrypted PEM keys
			return nil, fmt.Errorf("encrypted PEM private keys are not supported")
		}
		switch block.Type {
		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			switch k := key.(type) {
			case *rsa.PrivateKey:
				return k, nil
			case *ecdsa.PrivateKey:
				return k, nil
			default:
				return nil, fmt.Errorf("unsupported PKCS#8 private key type %T (RSA or ECDSA required)", key)
			}
		default:
			return nil, fmt.Errorf("unsupported private key PEM block type %q (RSA or ECDSA required)", block.Type)
		}
	}
	return nil, fmt.Errorf("no PEM private key found")
}

func checkDomain(name string) error {
	switch {
	case len(name) == 0:
		return nil // an empty domain name will result in a cookie without a domain restriction
	case len(name) > 255:
		return fmt.Errorf("domain name length is %d, can't exceed 255", len(name))
	}
	var l int
	for i := 0; i < len(name); i++ {
		b := name[i]
		if b == '.' {
			// check domain labels validity
			switch {
			case i == l:
				return fmt.Errorf("domain has invalid character '.' at offset %d, label can't begin with a period", i)
			case i-l > 63:
				return fmt.Errorf("domain byte length of label '%s' is %d, can't exceed 63", name[l:i], i-l)
			case name[l] == '-':
				return fmt.Errorf("domain label '%s' at offset %d begins with a hyphen", name[l:i], l)
			case name[i-1] == '-':
				return fmt.Errorf("domain label '%s' at offset %d ends with a hyphen", name[l:i], l)
			}
			l = i + 1
			continue
		}
		// test label character validity, note: tests are ordered by decreasing validity frequency
		if (b < 'a' || b > 'z') && (b < '0' || b > '9') && b != '-' && (b < 'A' || b > 'Z') {
			// show the printable unicode character starting at byte offset i
			c, _ := utf8.DecodeRuneInString(name[i:])
			if c == utf8.RuneError {
				return fmt.Errorf("domain has invalid rune at offset %d", i)
			}
			return fmt.Errorf("domain has invalid character '%c' at offset %d", c, i)
		}
	}
	// check top level domain validity
	switch {
	case l == len(name):
		return fmt.Errorf("domain has missing top level domain, domain can't end with a period")
	case len(name)-l > 63:
		return fmt.Errorf("domain's top level domain '%s' has byte length %d, can't exceed 63", name[l:], len(name)-l)
	case name[l] == '-':
		return fmt.Errorf("domain's top level domain '%s' at offset %d begin with a hyphen", name[l:], l)
	case name[len(name)-1] == '-':
		return fmt.Errorf("domain's top level domain '%s' at offset %d ends with a hyphen", name[l:], l)
	case name[l] >= '0' && name[l] <= '9':
		return fmt.Errorf("domain's top level domain '%s' at offset %d begins with a digit", name[l:], l)
	}
	return nil
}

// validate sanitizes and validates user input for mobileconfig generation
func (a *AppleService) validate(req requests.MobileConfigReq) (*requests.MobileConfigReq, error) {
	// Validate profile ID
	if req.ProfileId == "" {
		return nil, fmt.Errorf("profile_id is required")
	}

	if len(req.ProfileId) > maxProfileIdLength {
		return nil, fmt.Errorf("profile_id exceeds maximum length of %d", maxProfileIdLength)
	}

	// Skip further validation if no advanced options
	if req.AdvancedOptionsReq == nil {
		// Still sanitize device id if provided
		req.DeviceId = deviceid.Normalize(req.DeviceId)
		if req.DeviceId == "" { // ensure empty string not just spaces removed
			req.DeviceId = ""
		}
		return &req, nil
	}

	// Validate encryption type
	if req.EncryptionType != "" {
		switch req.EncryptionType {
		case "https", "tls":
			break
		default:
			return nil, fmt.Errorf("invalid encryption type: %s", req.EncryptionType)
		}
	}

	// Validate excluded domains if provided
	// Note: this option is currently not used
	// if req.AdvancedOptionsReq.ExcludedDomains != "" {
	// 	domains := strings.Split(req.AdvancedOptionsReq.ExcludedDomains, ",")
	// 	if len(domains) > maxDomainsCount {
	// 		log.Warn().Int("count", len(domains)).Int("max", maxDomainsCount).
	// 			Msg("Excluded domains count exceeded maximum")
	// 		return nil, fmt.Errorf("too many excluded domains (max %d)", maxDomainsCount)
	// 	}

	// 	validatedDomains := []string{}

	// 	for _, domain := range domains {
	// 		domain = strings.TrimSpace(domain)
	// 		if domain == "" {
	// 			continue
	// 		}

	// 		if len(domain) > maxDomainLength {
	// 			log.Warn().Str("domain", domain).Int("max_length", maxDomainLength).
	// 				Msg("Domain exceeds maximum length, skipping")
	// 			continue
	// 		}

	// 		if err := checkDomain(domain); err != nil {
	// 			log.Warn().Str("domain", domain).
	// 				Msg("Invalid domain format, skipping")
	// 			continue
	// 		}

	// 		validatedDomains = append(validatedDomains, domain)
	// 	}

	// 	// Reconstruct the validated domains string
	// 	req.AdvancedOptionsReq.ExcludedDomains = strings.Join(validatedDomains, ",")
	// }

	// Validate WiFi network names (excluded)
	if req.ExcludedWifiNetworks != "" {
		networks := strings.Split(req.ExcludedWifiNetworks, ",")
		if len(networks) > maxNetworksCount {
			log.Warn().Int("count", len(networks)).Int("max", maxNetworksCount).
				Msg("Excluded WiFi networks count exceeded maximum")
			return nil, fmt.Errorf("too many excluded WiFi networks (max %d)", maxNetworksCount)
		}

		validNetworks := []string{}
		for _, network := range networks {
			network = strings.TrimSpace(network)
			if network == "" {
				continue
			}

			if len(network) > maxNetworkLength {
				log.Warn().Str("network", network).Int("max_length", maxNetworkLength).
					Msg("WiFi network name exceeds maximum length, skipping")
				continue
			}

			// Escape any XML special characters
			network = escapeXML(network)
			validNetworks = append(validNetworks, network)
		}

		req.ExcludedWifiNetworks = strings.Join(validNetworks, ",")
	}

	// Sanitize optional device id (allow only [A-Za-z0-9 -] and truncate to runtime max
	req.DeviceId = deviceid.Normalize(req.DeviceId)
	if req.DeviceId == "" { // unify empty
		req.DeviceId = ""
	}

	return &req, nil
}

// escapeXML escapes XML special characters from a string
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
