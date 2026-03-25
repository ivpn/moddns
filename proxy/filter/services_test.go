package filter

import (
	"errors"
	"net"
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

type staticCatalog struct{ cat *servicescatalog.Catalog }

func (s staticCatalog) Get() (*servicescatalog.Catalog, error) { return s.cat, nil }

type staticCatalogErr struct{ err error }

func (s staticCatalogErr) Get() (*servicescatalog.Catalog, error) { return nil, s.err }

type staticASNLookup struct{ asn uint }

func (s staticASNLookup) ASN(_ net.IP) (uint, error) { return s.asn, nil }

type mapASNLookup struct {
	asnByIP map[string]uint
	err     error
}

func (m mapASNLookup) ASN(ip net.IP) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	if ip == nil {
		return 0, nil
	}
	if v, ok := m.asnByIP[ip.String()]; ok {
		return v, nil
	}
	return 0, nil
}

func newTestReqCtx(t *testing.T, profileID string) *requestcontext.RequestContext {
	t.Helper()
	loggerFactory := logging.NewFactory(zerolog.DebugLevel)
	testLogger := loggerFactory.ForProfile(profileID, true)
	return &requestcontext.RequestContext{ProfileId: profileID, Logger: testLogger}
}

func dnsCtxWithAAnswer(t *testing.T, ipStr string) *proxy.DNSContext {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ipStr)},
	}
	return &proxy.DNSContext{Req: req, Res: res}
}

func dnsCtxWithAnswers(t *testing.T, answers []dns.RR) *proxy.DNSContext {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = answers
	return &proxy.DNSContext{Req: req, Res: res}
}

func dnsCtxWithHTTPSAnswer(t *testing.T, name string, ipv4hints []net.IP, ipv6hints []net.IP) *proxy.DNSContext {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion(name, dns.TypeHTTPS)
	res := new(dns.Msg)
	res.SetReply(req)

	https := &dns.HTTPS{SVCB: dns.SVCB{
		Hdr:      dns.RR_Header{Name: name, Rrtype: dns.TypeHTTPS, Class: dns.ClassINET, Ttl: 60},
		Priority: 1,
		Target:   ".",
	}}
	if len(ipv4hints) > 0 {
		https.Value = append(https.Value, &dns.SVCBIPv4Hint{Hint: ipv4hints})
	}
	if len(ipv6hints) > 0 {
		https.Value = append(https.Value, &dns.SVCBIPv6Hint{Hint: ipv6hints})
	}
	res.Answer = []dns.RR{https}
	return &proxy.DNSContext{Req: req, Res: res}
}

func googleCatalogWithASN(asn uint) *servicescatalog.Catalog {
	return &servicescatalog.Catalog{Services: []servicescatalog.Service{{
		ID:   "google",
		Name: "Google",
		ASNs: []uint{asn},
	}}}
}

func multiServiceCatalog(asn uint) *servicescatalog.Catalog {
	return &servicescatalog.Catalog{Services: []servicescatalog.Service{
		{ID: "google", Name: "Google", ASNs: []uint{asn}},
		{ID: "cloudflare", Name: "Cloudflare", ASNs: []uint{asn}},
	}}
}

func TestIPFilter_filterServices_Table(t *testing.T) {
	const (
		profileID = "profile-services-table"
		asn       = uint(15169)
	)

	tests := []struct {
		name           string
		servicesGetter ServicesCatalogGetter
		asnLookup      ASNLookup
		blockedIDs     []string
		cacheErr       error
		dnsCtx         *proxy.DNSContext
		wantDecision   model.Decision
		wantReasons    []string
	}{
		{
			name:           "no catalog getter",
			servicesGetter: nil,
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "no asn lookup",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      nil,
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "nil dns context",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         nil,
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "nil response",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         &proxy.DNSContext{Req: new(dns.Msg), Res: nil},
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "cache error treated as disabled",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     nil,
			cacheErr:       errors.New("cache error"),
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "no blocked services",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "catalog load error treated as disabled",
			servicesGetter: staticCatalogErr{err: errors.New("catalog load")},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "no match when asn differs",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn + 1},
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "blocks when asn matches",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionBlock,
			wantReasons:    []string{REASON_SERVICES, "service: google"},
		},
		{
			name:           "blocks and reports multiple matched services",
			servicesGetter: staticCatalog{cat: multiServiceCatalog(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google", "cloudflare"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionBlock,
			wantReasons:    []string{REASON_SERVICES, "service: google", "service: cloudflare"},
		},
		{
			name:           "ignores unknown blocked service ids",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"unknown"},
			dnsCtx:         dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "multiple answers can match",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup: mapASNLookup{asnByIP: map[string]uint{
				"1.1.1.1": asn,
				"2.2.2.2": 0,
			}},
			blockedIDs: []string{"google"},
			dnsCtx: dnsCtxWithAnswers(t, []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("1.1.1.1")},
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("2.2.2.2")},
			}),
			wantDecision: model.DecisionBlock,
			wantReasons:  []string{REASON_SERVICES, "service: google"},
		},
		{
			name:           "blocks on HTTPS answer with ipv4hint when asn matches",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup: mapASNLookup{asnByIP: map[string]uint{
				"142.250.74.46": asn,
			}},
			blockedIDs: []string{"google"},
			dnsCtx:     dnsCtxWithHTTPSAnswer(t, "example.com.", []net.IP{net.ParseIP("142.250.74.46").To4()}, nil),
			wantDecision: model.DecisionBlock,
			wantReasons:  []string{REASON_SERVICES, "service: google"},
		},
		{
			name:           "blocks on HTTPS answer with ipv6hint when asn matches",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup: mapASNLookup{asnByIP: map[string]uint{
				"2a00:1450:4010:c0a::5e": asn,
			}},
			blockedIDs: []string{"google"},
			dnsCtx:     dnsCtxWithHTTPSAnswer(t, "example.com.", nil, []net.IP{net.ParseIP("2a00:1450:4010:c0a::5e")}),
			wantDecision: model.DecisionBlock,
			wantReasons:  []string{REASON_SERVICES, "service: google"},
		},
		{
			name:           "HTTPS answer without IP hints is not blocked",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup:      staticASNLookup{asn: asn},
			blockedIDs:     []string{"google"},
			dnsCtx:         dnsCtxWithHTTPSAnswer(t, "example.com.", nil, nil),
			wantDecision:   model.DecisionNone,
		},
		{
			name:           "blocks on AAAA answer when asn matches",
			servicesGetter: staticCatalog{cat: googleCatalogWithASN(asn)},
			asnLookup: mapASNLookup{asnByIP: map[string]uint{
				"2001:db8::1": asn,
			}},
			blockedIDs: []string{"google"},
			dnsCtx: dnsCtxWithAnswers(t, []dns.RR{
				&dns.AAAA{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}, AAAA: net.ParseIP("2001:db8::1")},
			}),
			wantDecision: model.DecisionBlock,
			wantReasons:  []string{REASON_SERVICES, "service: google"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			shouldCallCache := tt.servicesGetter != nil && tt.asnLookup != nil && tt.dnsCtx != nil && tt.dnsCtx.Res != nil
			if shouldCallCache {
				if tt.cacheErr != nil {
					mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return(nil, tt.cacheErr)
				} else {
					mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return(tt.blockedIDs, nil)
				}
			}

			ipFilter := &IPFilter{
				Cache:           mockCache,
				Proxy:           &proxy.Proxy{},
				ServicesCatalog: tt.servicesGetter,
				ASNLookup:       tt.asnLookup,
			}

			reqCtx := newTestReqCtx(t, profileID)
			got, err := ipFilter.filterServices(reqCtx, tt.dnsCtx)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, TierServices, got.Tier)
			assert.Equal(t, tt.wantDecision, got.Decision)
			for _, r := range tt.wantReasons {
				assert.Contains(t, got.Reasons, r)
			}

			if shouldCallCache {
				mockCache.AssertExpectations(t)
			} else {
				mockCache.AssertNotCalled(t, "GetProfileServicesBlocked", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestIPFilter_ServicesBlocking_Integration_Table(t *testing.T) {
	const (
		asn = uint(15169)
	)

	tests := []struct {
		name            string
		profileID       string
		blockedIDs      []string
		customRules     map[string]map[string]string
		customHashes    []string
		catalog         *servicescatalog.Catalog
		asnLookup       ASNLookup
		dnsCtx          *proxy.DNSContext
		wantStatus      model.Status
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "blocks when services ASN matches",
			profileID:    "svc-int-block",
			blockedIDs:   []string{"google"},
			customHashes: []string{},
			catalog:      googleCatalogWithASN(asn),
			asnLookup:    staticASNLookup{asn: asn},
			dnsCtx:       dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantStatus:   model.StatusBlocked,
			wantContains: []string{REASON_SERVICES, "service: google"},
		},
		{
			name:         "allow-by-ip custom rule overrides services block (final aggregation)",
			profileID:    "svc-int-allow-ip",
			blockedIDs:   []string{"google"},
			customHashes: []string{"hash_allow"},
			customRules: map[string]map[string]string{
				"hash_allow": {"action": ACTION_ALLOW, "value": "1.1.1.1", "syntax": "ip4_addr"},
			},
			catalog:    googleCatalogWithASN(asn),
			asnLookup:  staticASNLookup{asn: asn},
			dnsCtx:     dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantStatus: model.StatusProcessed,
		},
		{
			name:         "allow-by-asn custom rule overrides services block (final aggregation)",
			profileID:    "svc-int-allow-asn",
			blockedIDs:   []string{"google"},
			customHashes: []string{"hash_allow_asn"},
			customRules: map[string]map[string]string{
				"hash_allow_asn": {"action": ACTION_ALLOW, "value": "AS15169", "syntax": "asn"},
			},
			catalog:         googleCatalogWithASN(asn),
			asnLookup:       staticASNLookup{asn: asn},
			dnsCtx:          dnsCtxWithAAnswer(t, "1.1.1.1"),
			wantStatus:      model.StatusProcessed,
			wantContains:    []string{REASON_CUSTOM_RULES},
			wantNotContains: []string{REASON_SERVICES},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := new(mocks.Cache)
			mockCache.On("GetProfileServicesBlocked", mock.Anything, tt.profileID).Return(tt.blockedIDs, nil)
			mockCache.On("GetCustomRulesHashes", mock.Anything, tt.profileID).Return(tt.customHashes, nil)
			for hash, rule := range tt.customRules {
				mockCache.On("GetCustomRulesHash", mock.Anything, hash).Return(rule, nil)
			}

			dnsProxy := &proxy.Proxy{}
			ipFilter := NewIPFilter(dnsProxy, mockCache, staticCatalog{cat: tt.catalog}, tt.asnLookup)
			reqCtx := newTestReqCtx(t, tt.profileID)

			err := ipFilter.Execute(reqCtx, tt.dnsCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, reqCtx.FilterResult.Status)
			for _, s := range tt.wantContains {
				assert.Contains(t, reqCtx.FilterResult.Reasons, s)
			}
			for _, s := range tt.wantNotContains {
				assert.NotContains(t, reqCtx.FilterResult.Reasons, s)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

func TestExtractIPsFromAnswer(t *testing.T) {
	tests := []struct {
		name    string
		answers []dns.RR
		wantIPs []string
	}{
		{
			name:    "nil answers",
			answers: nil,
			wantIPs: nil,
		},
		{
			name: "A record",
			answers: []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4")},
			},
			wantIPs: []string{"1.2.3.4"},
		},
		{
			name: "AAAA record",
			answers: []dns.RR{
				&dns.AAAA{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA}, AAAA: net.ParseIP("2001:db8::1")},
			},
			wantIPs: []string{"2001:db8::1"},
		},
		{
			name: "HTTPS with ipv4hint and ipv6hint",
			answers: []dns.RR{
				&dns.HTTPS{SVCB: dns.SVCB{
					Hdr:      dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeHTTPS},
					Priority: 1,
					Target:   ".",
					Value: []dns.SVCBKeyValue{
						&dns.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("142.250.74.46").To4()}},
						&dns.SVCBIPv6Hint{Hint: []net.IP{net.ParseIP("2a00:1450::1")}},
					},
				}},
			},
			wantIPs: []string{"142.250.74.46", "2a00:1450::1"},
		},
		{
			name: "HTTPS without IP hints",
			answers: []dns.RR{
				&dns.HTTPS{SVCB: dns.SVCB{
					Hdr:      dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeHTTPS},
					Priority: 1,
					Target:   ".",
					Value: []dns.SVCBKeyValue{
						&dns.SVCBAlpn{Alpn: []string{"h2", "h3"}},
					},
				}},
			},
			wantIPs: nil,
		},
		{
			name: "mixed A and HTTPS records",
			answers: []dns.RR{
				&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("1.1.1.1")},
				&dns.HTTPS{SVCB: dns.SVCB{
					Hdr:      dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeHTTPS},
					Priority: 1,
					Target:   ".",
					Value: []dns.SVCBKeyValue{
						&dns.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("2.2.2.2").To4()}},
					},
				}},
			},
			wantIPs: []string{"1.1.1.1", "2.2.2.2"},
		},
		{
			name: "SVCB record with ipv4hint",
			answers: []dns.RR{
				&dns.SVCB{
					Hdr:      dns.RR_Header{Name: "_svc.example.com.", Rrtype: dns.TypeSVCB},
					Priority: 1,
					Target:   ".",
					Value: []dns.SVCBKeyValue{
						&dns.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("3.3.3.3").To4()}},
					},
				},
			},
			wantIPs: []string{"3.3.3.3"},
		},
		{
			name: "unrelated record types are ignored",
			answers: []dns.RR{
				&dns.CNAME{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeCNAME}, Target: "other.example.com."},
				&dns.MX{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeMX}, Mx: "mail.example.com."},
			},
			wantIPs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIPsFromAnswer(tt.answers)
			if tt.wantIPs == nil {
				assert.Nil(t, got)
				return
			}
			var gotStrs []string
			for _, ip := range got {
				gotStrs = append(gotStrs, ip.String())
			}
			assert.Equal(t, tt.wantIPs, gotStrs)
		})
	}
}
