package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/libs/servicescatalogcache"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/filter"
	"github.com/ivpn/dns/proxy/internal/asnlookup"
	"github.com/ivpn/dns/proxy/internal/dnssec"
	"github.com/ivpn/dns/proxy/internal/metrics"
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	gocache "github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

const (
	ProfileIdAdditionalSectionCode = 0xfeed
)

type Server struct {
	Config    *config.Config
	Proxy     *proxy.Proxy // service.Interface
	Upstreams map[string]*proxy.CustomUpstreamConfig
	// edeStore holds DNSSEC-failure Extended DNS Error codes captured from upstream
	// responses (by dnssec.CapturingUpstream), drained per-request by EmitQueryLog.
	edeStore             *dnssec.EDEStore
	DomainFilter         filter.Filter
	IPFilter             filter.Filter
	Cache                cache.Cache
	ProfileSettingsCache *gocache.Cache
	CollectorChannels    map[string]channel.CollectorChannel
	LoggerFactory        logging.FactoryInterface
	RateLimiter          *ratelimit.RateLimiter
	Metrics              Metrics
}

var _ proxy.Handler = (*Server)(nil)

var (
	errProfileIdNotProvided = errors.New("profile_id not provided")
	errProfileIdNotFound    = errors.New("profile_id not found")
	errRateLimitedIP        = errors.New("rate limited by IP")
	errRateLimitedProfile   = errors.New("rate limited by profile")
)

func NewServer(serverConfig *config.Config, collectorChannels map[string]channel.CollectorChannel) (*Server, error) {
	cache, err := cache.NewCache(serverConfig.Cache, cache.CacheTypeRedis)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create cache")
	}

	// Initialize logging factory
	loggerFactory := logging.NewDefaultFactory()

	// In-memory profile settings cache to avoid Redis round-trips for warm profiles.
	profileSettingsCache := gocache.New(serverConfig.Server.ProfileSettingsCacheTTL, 2*serverConfig.Server.ProfileSettingsCacheTTL)

	rl := ratelimit.New(ratelimit.Config{
		PerIPEnabled:      serverConfig.RateLimit.PerIPEnabled,
		PerIPRate:         serverConfig.RateLimit.PerIPRate,
		PerIPBurst:        serverConfig.RateLimit.PerIPBurst,
		PerProfileEnabled: serverConfig.RateLimit.PerProfileEnabled,
		PerProfileRate:    serverConfig.RateLimit.PerProfileRate,
		PerProfileBurst:   serverConfig.RateLimit.PerProfileBurst,
		MaxBuckets:        serverConfig.RateLimit.MaxBuckets,
		IPv6PrefixLen:     serverConfig.RateLimit.IPv6PrefixLen,
	}, metrics.NewRateLimitMetrics(prometheus.DefaultRegisterer))

	server := &Server{
		Config:               serverConfig,
		Cache:                cache,
		ProfileSettingsCache: profileSettingsCache,
		CollectorChannels:    collectorChannels,
		Upstreams:            make(map[string]*proxy.CustomUpstreamConfig, 0),
		edeStore:             &dnssec.EDEStore{},
		LoggerFactory:        loggerFactory,
		RateLimiter:          rl,
		Metrics:              metrics.NewServerMetrics(prometheus.DefaultRegisterer),
	}

	dnsProxy, err := server.newProxy(ProxyTypeAdguard, serverConfig)
	if err != nil {
		return nil, err
	}

	// Services ASN blocking dependencies — both catalog and GeoDB are required.
	servicesCatalog, err := servicescatalogcache.New(serverConfig.Services.CatalogPath, serverConfig.Services.CatalogReloadEvery)
	if err != nil {
		log.Error().Err(err).Str("path", serverConfig.Services.CatalogPath).Msg("Failed to initialize services catalog")
		return nil, fmt.Errorf("services catalog: %w", err)
	}
	go servicesCatalog.Start(context.Background())

	lookup, err := asnlookup.New(serverConfig.Services.GeoIPASNDBPath)
	if err != nil {
		log.Error().Err(err).Str("path", serverConfig.Services.GeoIPASNDBPath).Msg("Failed to open ASN MMDB")
		return nil, fmt.Errorf("ASN lookup: %w", err)
	}
	log.Info().Str("catalog", serverConfig.Services.CatalogPath).Str("geodb", serverConfig.Services.GeoIPASNDBPath).Msg("Services blocking enabled")

	server.DomainFilter = filter.NewDomainFilter(dnsProxy, cache, servicesCatalog)
	server.IPFilter = filter.NewIPFilter(dnsProxy, cache, servicesCatalog, lookup, serverConfig.Rebinding, serverConfig.Filtering)
	server.Proxy = dnsProxy

	profileIDMinLength = serverConfig.ProfileIDMinLength
	return server, nil
}

// ServeDNS implements [proxy.Handler]; it is the single entry point for every
// DNS request. The vendor proxy has already validated the question section
// (exactly one question) and answered ANY queries before calling it.
func (s *Server) ServeDNS(ctx context.Context, p *proxy.Proxy, dctx *proxy.DNSContext) (err error) {
	defer sentry.Recover()

	reqCtx, errResp, err := s.prepareRequest(ctx, p, dctx)
	if err != nil {
		// No response is sent for dropped requests.
		return fmt.Errorf("%w: %w", proxy.ErrDrop, err)
	}
	if errResp != nil {
		dctx.Res = errResp
		return nil
	}

	s.handleRequest(ctx, dctx, reqCtx)
	return nil
}

// postResolve runs IP filtering, emits query logs/statistics, and responds.
func (s *Server) postResolve(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	if reqCtx.FilterResult.Status != model.StatusBlocked {
		ipStart := time.Now()
		if err := s.IPFilter.Execute(reqCtx, dctx); err != nil {
			reqCtx.Logger.Err(err).Msg("IP Filtering error")
		}
		s.Metrics.RecordIPFilterDuration(string(dctx.Proto), time.Since(ipStart))
		if reqCtx.FilterResult.Status == model.StatusBlocked {
			s.Metrics.RecordBlocked("ip")
		}
	}
	s.respond(reqCtx, dctx)
	if !reqCtx.StartTime.IsZero() {
		s.Metrics.RecordQueryDuration(string(dctx.Proto), time.Since(reqCtx.StartTime))
	}
	go s.EmitQueryLog(reqCtx, dctx)
	go s.EmitStatistics(reqCtx, dctx)
}

// prepareRequest runs everything that must precede filtering: rate limits,
// request validation, profile extraction and settings lookup. It returns
// exactly one of: a request context to continue with, a DNS response to answer
// immediately (errResp), or an error meaning the request must be dropped
// without a response.
func (s *Server) prepareRequest(ctx context.Context, p *proxy.Proxy, dctx *proxy.DNSContext) (reqCtx *requestcontext.RequestContext, errResp *dns.Msg, err error) {
	s.Metrics.RecordQuery(string(dctx.Proto))

	// Layer 1: per-IP rate limit (before any IO or profile extraction).
	if !s.RateLimiter.CheckIP(dctx.Addr.Addr(), string(dctx.Proto)) {
		if s.Config.RateLimit.PerIPResponse == config.RateLimitResponseRefuse {
			return nil, s.refusedResponse(dctx.Req), nil
		}
		return nil, nil, errRateLimitedIP
	}

	// QDCOUNT (RFC 1035 §4.1.1) is a header field not validated against the body,
	// so a message can unpack cleanly with an empty question section. Exactly one
	// question is required for opcode QUERY (RFC 9619); QDCOUNT=0 is only valid
	// for DNS Cookies (RFC 7873 §5.4), which this proxy does not implement.
	// The vendor proxy already answers FORMERR before invoking the handler;
	// this guard keeps the invariant when the handler is driven directly.
	if len(dctx.Req.Question) != 1 {
		return nil, s.formErrResponse(dctx.Req), nil
	}

	profileId, deviceId, err := s.clientIDFromDNSContext(dctx)
	if err != nil {
		return nil, nil, fmt.Errorf("getting profile_id: %w", err)
	}

	// Create a system logger for initial operations (before we know profile settings)
	systemLogger := s.LoggerFactory.ForSystem()
	systemLogger.Trace().Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("device_id", deviceId).Msg("Profile ID extracted from DNS context")

	if profileId == "" {
		// drop DNS request if profile_id is not provided
		systemLogger.Warn().Err(errProfileIdNotProvided).Msg(errProfileIdNotProvided.Error())
		return nil, nil, errProfileIdNotProvided
	} else {
		// Try in-memory profile settings cache first.
		var settings *model.ProfileSettings
		if cached, ok := s.ProfileSettingsCache.Get(profileId); ok {
			s.Metrics.RecordProfileCacheLookup(true)
			settings = cached.(*model.ProfileSettings)
		} else {
			s.Metrics.RecordProfileCacheLookup(false)
			// Cache miss — fetch from Redis pipeline.
			var fetchErr error
			settings, fetchErr = s.Cache.GetProfileSettingsBatch(ctx, profileId)
			if fetchErr != nil {
				systemLogger.Err(fetchErr).Msg("Failed to fetch profile settings batch")
				return nil, nil, errProfileIdNotFound
			}
			// Cache only successful fetches (profile exists).
			if settings.PrivacyErr == nil {
				s.ProfileSettingsCache.Set(profileId, settings, gocache.DefaultExpiration)
			}
		}

		// Privacy settings are required — missing means profile doesn't exist.
		if settings.PrivacyErr != nil {
			systemLogger.Debug().Err(settings.PrivacyErr).Msg(errProfileIdNotFound.Error())
			return nil, nil, errProfileIdNotFound
		}

		// Layer 2: per-profile rate limit. Runs after the existence check so
		// buckets are only created for profiles that exist.
		if !s.RateLimiter.CheckProfile(profileId, string(dctx.Proto)) {
			if s.Config.RateLimit.PerProfileResponse == config.RateLimitResponseRefuse {
				return nil, s.refusedResponse(dctx.Req), nil
			}
			return nil, nil, errRateLimitedProfile
		}
		prvSettings := settings.Privacy

		// Logs settings: default to enabled if unavailable.
		logsSettings := settings.Logs
		var loggingEnabled bool
		if settings.LogsErr != nil {
			systemLogger.Warn().Err(settings.LogsErr).Msg("Error getting profile logs settings, defaulting to enabled")
			loggingEnabled = true
		} else {
			loggingEnabled, err = strconv.ParseBool(logsSettings["enabled"])
			if err != nil {
				systemLogger.Err(err).Msg("Error parsing profile logs settings, defaulting to enabled")
				loggingEnabled = true
			}
		}

		// Determine domain logging preference
		var logDomains, logClientIPs bool
		if logsSettings != nil {
			if v, ok := logsSettings["log_domains"]; ok && (v == "true" || v == "1") {
				logDomains = true
			}
			if v, ok := logsSettings["log_clients_ips"]; ok && (v == "true" || v == "1") {
				logClientIPs = true
			}
		}

		// Create contextual logger including domain logging flag (Level=0 triggers factory default)
		reqLogger := s.LoggerFactory.ForRequest(logging.LoggingConfig{
			Enabled:      loggingEnabled,
			Level:        0,
			ProfileID:    profileId,
			LogDomains:   logDomains,
			LogClientIPs: logClientIPs,
		})

		// DNSSEC settings: default to enabled if unavailable.
		dnssecSettings := settings.DNSSEC
		var dnssecEnabled, sendDoBit = true, true
		if settings.DNSSECErr != nil {
			reqLogger.Debug().Msg("DNSSEC settings not found, using default values")
		} else {
			dnssecEnabled, err = strconv.ParseBool(dnssecSettings["enabled"])
			if err != nil {
				reqLogger.Err(err).Msg(errProfileIdNotFound.Error())
				return nil, nil, errProfileIdNotFound
			}
			sendDoBit, err = strconv.ParseBool(dnssecSettings["send_do_bit"])
			if err != nil {
				reqLogger.Err(err).Msg(errProfileIdNotFound.Error())
				return nil, nil, errProfileIdNotFound
			}
		}

		// Rebinding protection (security): missing hash = empty map = opt-in OFF.
		// Raw map is threaded through; the IP-phase filter reads the "enabled" key.
		rebindingProtectionSettings := settings.RebindingProtection

		// Advanced settings: default upstream if unavailable.
		advancedSettings := settings.Advanced
		upstreamName := s.Config.Upstream.Default
		if settings.AdvancedErr != nil {
			reqLogger.Info().Str("upstream", s.Config.Upstream.Default).Msg("Advanced settings not found, using default values")
		} else if recursor, recursorFound := advancedSettings["recursor"]; recursorFound && recursor != "" {
			upstreamName = recursor
		} else {
			reqLogger.Trace().Msg("Recursor not set, using default")
		}

		// Fall back to the default recursor when the selected upstream is not
		// configured (e.g. a stale/removed recursor name persisted on a profile),
		// so a query is never routed to a nil upstream.
		upstreamConfig, ok := s.Upstreams[upstreamName]
		if !ok {
			reqLogger.Warn().Str("recursor", upstreamName).Str("upstream", s.Config.Upstream.Default).Msg("Unknown recursor, falling back to default")
			upstreamName = s.Config.Upstream.Default
			upstreamConfig = s.Upstreams[upstreamName]
		}

		dctx.CustomUpstreamConfig = upstreamConfig
		reqLogger.Trace().Str("upstream", upstreamName).Msg("Upstream set")
		reqCtx = requestcontext.NewRequestContext(ctx, p, profileId, deviceId, prvSettings, logsSettings, dnssecSettings, rebindingProtectionSettings, advancedSettings, reqLogger)
		reqCtx.StartTime = time.Now()
		reqCtx.UpstreamName = upstreamName

		dnssec.ApplyRequestFlags(dctx.Req, dnssecEnabled, sendDoBit)
	}

	return reqCtx, nil, nil
}

// handleRequest runs domain filtering, resolves via the profile's upstream
// when the query is not blocked, and finishes with postResolve for both cache
// hits and misses.
func (s *Server) handleRequest(ctx context.Context, dctx *proxy.DNSContext, reqCtx *requestcontext.RequestContext) {
	// Use the contextual logger from the request context
	reqLogger := reqCtx.Logger

	if s.dnsCheckHandler(dctx, reqCtx.ProfileId, reqLogger) {
		reqLogger.Debug().Msg("DNS check handler executed")
		return
	}

	// perform filtering actions
	domainStart := time.Now()
	if err := s.DomainFilter.Execute(reqCtx, dctx); err != nil {
		reqLogger.Err(err).Msg("Filtering error")
	}
	s.Metrics.RecordDomainFilterDuration(string(dctx.Proto), time.Since(domainStart))
	if reqCtx.FilterResult.Status == model.StatusBlocked {
		s.Metrics.RecordBlocked("domain")
	}

	if reqCtx.FilterResult.Status == model.StatusProcessed {
		reqLogger.Trace().Msg("Triggering default resolver")
		upstreamStart := time.Now()
		if err := s.Proxy.Resolve(ctx, dctx); err != nil {
			reqCtx.UpstreamErr = err
			reqLogger.Err(err).Msg("DNS resolving error")
		}
		s.Metrics.RecordUpstreamDuration(reqCtx.UpstreamName, time.Since(upstreamStart))
	}

	s.postResolve(reqCtx, dctx)
}

func (s *Server) respond(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	if reqCtx.FilterResult.Status != model.StatusBlocked {
		return
	}

	resp := new(dns.Msg)
	resp.SetReply(dctx.Req)

	switch dctx.Req.Question[0].Qtype {
	case dns.TypeA:
		q := dctx.Req.Question[0].Name
		resp.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{Name: q, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 30},
			A:   net.IPv4zero,
		}}
	case dns.TypeAAAA:
		q := dctx.Req.Question[0].Name
		resp.Answer = []dns.RR{&dns.AAAA{
			Hdr:  dns.RR_Header{Name: q, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 30},
			AAAA: net.IPv6zero,
		}}
	default:
		// For HTTPS, SVCB, and other record types: return empty answer (NODATA).
		// An empty answer with NOERROR signals the domain exists but has no records
		// of the requested type, which correctly blocks without type mismatch.
	}

	dctx.Res = resp
}

func (s *Server) dnsCheckHandler(dctx *proxy.DNSContext, profileId string, logger logging.LoggerInterface) (executed bool) {
	e := logger.Trace().Str("cfg", s.Config.Server.DnsCheckDomain)
	if logger.Config().LogDomains {
		e = e.Str("dctx.question", dctx.Req.Question[0].Name)
	}
	e.Msg("Checking if DNS check handler should be executed")
	if strings.Contains(dctx.Req.Question[0].Name, s.Config.Server.DnsCheckDomain) {
		logger.Trace().Msg("DNS check request received")
		// Build a proper DNS response based on upstream authoritative reply.
		// We don't assign dctx.Res here; will set after upstream exchange.
		executed = true
		c := new(dns.Client)
		m := new(dns.Msg)
		var qtype uint16
		switch dctx.Req.Question[0].Qtype {
		case dns.TypeA:
			qtype = dns.TypeA
		case dns.TypeAAAA:
			qtype = dns.TypeAAAA
		}

		// Add profileId to the additional section
		opt := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
			Option: []dns.EDNS0{
				&dns.EDNS0_LOCAL{
					Code: ProfileIdAdditionalSectionCode, // Custom option code
					Data: []byte(profileId),
				},
			},
		}
		m.Extra = append(m.Extra, opt)

		m.SetQuestion(dns.Fqdn(dctx.Req.Question[0].Name), qtype)
		// send the request
		dnsCheckServerAddress := s.Config.Server.DnsCheckDomain + ":" + s.Config.Server.DnsCheckPort
		logger.Trace().Str("dns_server", dnsCheckServerAddress).Msg("Sending DNS check request")
		r, _, err := c.Exchange(m, dnsCheckServerAddress) // "dnscheck:53"
		if err != nil {
			logger.Error().Err(err).Msg("error sending test query")
			return
		}
		if r == nil {
			logger.Error().Err(err).Msg("r is nil")
			return
		}

		if r.Rcode != dns.RcodeSuccess {
			logger.Error().Err(err).Msg("invalid answer name  after MX query for ")
		}
		// Build a well-formed response. We intentionally DO NOT preserve any EDNS(OPT)
		// records from the upstream response to avoid leaking upstream/local EDNS0 options
		// or padding. Per RFC 6891, absence of OPT simply signals no EDNS capabilities
		// in this specific message; clients will handle it gracefully.
		dctx.Res = s.buildDNSCheckResponse(dctx.Req, r)
		return
	}
	return
}

// buildDNSCheckResponse constructs a proper DNS response for the dns-check flow.
// It sets QR, copies the ID/opcode via SetReply, propagates Rcode and copies
// Answer/Ns/Extra sections EXCEPT any OPT (EDNS) pseudo-records which are
// intentionally stripped (see comment in caller). Authoritative flag is set
// since we act as an authoritative-style responder for this synthetic domain.
func (s *Server) buildDNSCheckResponse(origReq *dns.Msg, upstream *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(origReq) // sets Response flag and copies ID/opcode
	resp.Authoritative = true
	resp.Rcode = upstream.Rcode

	// Copy Answer records
	if len(upstream.Answer) > 0 {
		resp.Answer = make([]dns.RR, len(upstream.Answer))
		copy(resp.Answer, upstream.Answer)
	}

	// Helper to copy a section excluding OPT records.
	filterSection := func(src []dns.RR) (dst []dns.RR) {
		for _, rr := range src {
			if _, isOpt := rr.(*dns.OPT); isOpt {
				continue // drop EDNS OPT pseudo-RR deliberately
			}
			dst = append(dst, rr)
		}
		return
	}

	if len(upstream.Ns) > 0 {
		resp.Ns = filterSection(upstream.Ns)
	}
	if len(upstream.Extra) > 0 {
		resp.Extra = filterSection(upstream.Extra)
	}

	return resp
}

// refusedResponse builds a minimal DNS REFUSED response for the given request.
func (s *Server) refusedResponse(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeRefused)
	return resp
}

// formErrResponse builds a minimal DNS FORMERR response for the given request.
func (s *Server) formErrResponse(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeFormatError)
	return resp
}
