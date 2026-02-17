package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/cache/memory"
	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/filter"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	gocache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
)

const (
	ProfileIdAdditionalSectionCode = 0xfeed
)

type RequestManager interface {
	HandleBefore(p *proxy.Proxy, dctx *proxy.DNSContext) (err error)
	RequestHandler() func(p *proxy.Proxy, dctx *proxy.DNSContext) (err error)
	ResponseHandler() func(dctx *proxy.DNSContext, err error)
}

type Server struct {
	Config                *config.Config
	Proxy                 *proxy.Proxy // service.Interface
	Upstreams             map[string]*proxy.CustomUpstreamConfig
	DomainFilter          filter.Filter
	IPFilter              filter.Filter
	Cache                 cache.Cache
	InMemoryCache         memory.MemoryCache
	ProfileSettingsCache  *gocache.Cache
	CollectorChannels     map[string]channel.CollectorChannel
	LoggerFactory         logging.FactoryInterface
}

var _ RequestManager = (*Server)(nil)

var (
	errProfileIdNotProvided = errors.New("profile_id not provided")
	errProfileIdNotFound    = errors.New("profile_id not found")
)

func NewServer(serverConfig *config.Config, collectorChannels map[string]channel.CollectorChannel) (*Server, error) {
	cache, err := cache.NewCache(serverConfig.Cache, cache.CacheTypeRedis)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create cache")
	}

	memoryCache, err := memory.NewCache(serverConfig.Cache, memory.CacheTypeBigCache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create in memory cache")
	}

	// Initialize logging factory
	loggerFactory := logging.NewDefaultFactory()

	// In-memory profile settings cache to avoid Redis round-trips for warm profiles.
	profileSettingsCache := gocache.New(serverConfig.Server.ProfileSettingsCacheTTL, 2*serverConfig.Server.ProfileSettingsCacheTTL)

	server := &Server{
		Config:               serverConfig,
		Cache:                cache,
		InMemoryCache:        memoryCache,
		ProfileSettingsCache: profileSettingsCache,
		CollectorChannels:    collectorChannels,
		Upstreams:            make(map[string]*proxy.CustomUpstreamConfig, 0),
		LoggerFactory:        loggerFactory,
	}

	dnsProxy, err := server.newProxy(ProxyTypeAdguard, serverConfig)
	if err != nil {
		return nil, err
	}
	DomainFltr, err := filter.NewFilter(dnsProxy, cache, filter.FilterTypeDomain)
	if err != nil {
		return nil, err
	}

	IPFltr, err := filter.NewFilter(dnsProxy, cache, filter.FilterTypeIP)
	if err != nil {
		return nil, err
	}

	server.DomainFilter = DomainFltr
	server.IPFilter = IPFltr
	server.Proxy = dnsProxy

	profileIDMinLength = serverConfig.ProfileIDMinLength
	return server, nil
}

func (s *Server) HandleBefore(p *proxy.Proxy, dctx *proxy.DNSContext) (err error) {
	defer sentry.Recover()

	profileId, deviceId, err := s.clientIDFromDNSContext(dctx)
	if err != nil {
		return fmt.Errorf("getting profile_id: %w", err)
	}

	// Create a system logger for initial operations (before we know profile settings)
	systemLogger := s.LoggerFactory.ForSystem()
	systemLogger.Trace().Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("device_id", deviceId).Msg("Profile ID extracted from DNS context")

	if profileId == "" {
		// drop DNS request if profile_id is not provided
		systemLogger.Err(errProfileIdNotProvided).Msg(errProfileIdNotProvided.Error())
		return errProfileIdNotProvided
	} else {
		// Try in-memory profile settings cache first.
		var settings *model.ProfileSettings
		if cached, ok := s.ProfileSettingsCache.Get(profileId); ok {
			settings = cached.(*model.ProfileSettings)
		} else {
			// Cache miss — fetch from Redis pipeline.
			var fetchErr error
			settings, fetchErr = s.Cache.GetProfileSettingsBatch(context.Background(), profileId)
			if fetchErr != nil {
				systemLogger.Err(fetchErr).Msg("Failed to fetch profile settings batch")
				return errProfileIdNotFound
			}
			// Cache only successful fetches (profile exists).
			if settings.PrivacyErr == nil {
				s.ProfileSettingsCache.Set(profileId, settings, gocache.DefaultExpiration)
			}
		}

		// Privacy settings are required — missing means profile doesn't exist.
		if settings.PrivacyErr != nil {
			systemLogger.Err(settings.PrivacyErr).Msg(errProfileIdNotFound.Error())
			return errProfileIdNotFound
		}
		prvSettings := settings.Privacy

		// Logs settings: default to enabled if unavailable.
		logsSettings := settings.Logs
		var loggingEnabled bool
		if settings.LogsErr != nil {
			systemLogger.Err(settings.LogsErr).Msg("Error getting profile logs settings, defaulting to enabled")
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
				return errProfileIdNotFound
			}
			sendDoBit, err = strconv.ParseBool(dnssecSettings["send_do_bit"])
			if err != nil {
				reqLogger.Err(err).Msg(errProfileIdNotFound.Error())
				return errProfileIdNotFound
			}
		}

		// Advanced settings: default upstream if unavailable.
		advancedSettings := settings.Advanced
		upstreamName := s.Config.Upstream.Default
		if settings.AdvancedErr != nil {
			reqLogger.Info().Str("upstream", s.Config.Upstream.Default).Msg("Advanced settings not found, using default values")
		} else {
			var recursorFound bool
			upstreamName, recursorFound = advancedSettings["recursor"]
			if !recursorFound {
				reqLogger.Trace().Msg("Recursor not set, using default")
			}
		}

		dctx.CustomUpstreamConfig = s.Upstreams[upstreamName]
		reqLogger.Trace().Str("upstream", upstreamName).Msg("Upstream set")
		reqCtx := requestcontext.NewRequestContext(context.Background(), p, profileId, deviceId, prvSettings, logsSettings, dnssecSettings, advancedSettings, reqLogger)
		// TODO: set TTL for this request context - it's unnecessary to keep it in cache for long time since it's read right away in RequestHandler
		// TODO: investigate other in-memory cache types
		if err = s.InMemoryCache.SetRequestCtx(strconv.FormatUint(dctx.RequestID, 10), reqCtx); err != nil {
			reqLogger.Err(err).Msg("Failed to set request context")
			return err
		}

		dctx.Req.Extra = make([]dns.RR, 0)
		if !dnssecEnabled {
			dctx.Req.CheckingDisabled = true
		}

		if sendDoBit {
			// Enable EDNS0 with a reasonable UDP buffer size and DO=1
			// This sets a proper OPT RR instead of constructing one manually.
			dctx.Req.SetEdns0(2048, true)
		}
	}

	return nil
}

func (s *Server) RequestHandler() func(p *proxy.Proxy, dctx *proxy.DNSContext) (err error) {
	return func(p *proxy.Proxy, dctx *proxy.DNSContext) (err error) {
		defer sentry.Recover()
		reqCtx, err := s.InMemoryCache.GetRequestCtx(strconv.FormatUint(dctx.RequestID, 10))
		if err != nil {
			// Use system logger if we can't get the request context
			systemLogger := s.LoggerFactory.ForSystem()
			systemLogger.Err(err).Msg("Failed to get request context")
		}

		// Use the contextual logger from the request context
		reqLogger := reqCtx.Logger

		if s.dnsCheckHandler(dctx, reqCtx.ProfileId, reqLogger) {
			reqLogger.Debug().Msg("DNS check handler executed")
			return nil
		}

		// perform filtering actions
		if err = s.DomainFilter.Execute(reqCtx, dctx); err != nil {
			reqLogger.Err(err).Msg("Filtering error")
		}

		if err = s.InMemoryCache.SetRequestCtx(strconv.FormatUint(dctx.RequestID, 10)+"_response", reqCtx); err != nil {
			reqLogger.Err(err).Msg("Failed to set request context")
			return err
		}

		if reqCtx.FilterResult.Status == model.StatusProcessed {
			reqLogger.Trace().Msg("Triggering default resolver")
			if err := s.Proxy.Resolve(dctx); err != nil {
				reqLogger.Err(err).Msg("DNS resolving error")
			}
		} else {
			if s.Proxy.ResponseHandler != nil {
				reqLogger.Trace().Msg("Going to response handler")
				s.Proxy.ResponseHandler(dctx, err)
			}
		}

		return nil
	}
}

func (s *Server) respond(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	var emptyResourceRecord string
	switch dctx.Req.Question[0].Qtype {
	case dns.TypeA:
		emptyResourceRecord = "%s	30	IN	A	0.0.0.0"
	case dns.TypeAAAA:
		emptyResourceRecord = "%s	30	IN	AAAA	::"
	default:
		emptyResourceRecord = "%s	30	IN	A	0.0.0.0"
	}

	if reqCtx.FilterResult.Status == model.StatusBlocked {
		dctx.Res = dctx.Req
		dctx.Res.MsgHdr.Response = true // Set QR flag to indicate this is a response
		q := dctx.Req.Question[0].Name
		fakeRR, err := dns.NewRR(fmt.Sprintf(emptyResourceRecord, q))
		if err != nil {
			reqCtx.Logger.Err(err).Msg("Error creating fake RR")
		}
		dctx.Res.Answer = []dns.RR{fakeRR}
	}
}

func (s *Server) ResponseHandler() func(dctx *proxy.DNSContext, err error) {
	return func(dctx *proxy.DNSContext, err error) {
		defer sentry.Recover()

		// get DNS request context from cache containing filtering results
		reqCtx, ctxErr := s.InMemoryCache.GetRequestCtx(strconv.FormatUint(dctx.RequestID, 10) + "_response")

		// Use system logger if we can't get request context, otherwise use contextual logger
		// TODO: check if necessary
		var logger logging.LoggerInterface
		if ctxErr != nil {
			logger = s.LoggerFactory.ForSystem()
			logger.Err(ctxErr).Msg("Failed to get request context")
		} else {
			logger = reqCtx.Logger
		}

		if err != nil {
			logger.Err(err).Msg("DNS resolving error")
		}

		// Only continue if we have a valid request context
		if ctxErr == nil {
			// perform filtering actions
			if err = s.IPFilter.Execute(reqCtx, dctx); err != nil {
				logger.Err(err).Msg("IP Filtering error")
			}

			go s.EmitQueryLog(reqCtx, dctx)
			go s.EmitStatistics(reqCtx, dctx)

			s.respond(reqCtx, dctx)
		}
	}
}

func (s *Server) dnsCheckHandler(dctx *proxy.DNSContext, profileId string, logger logging.LoggerInterface) (executed bool) {
	logger.Trace().Str("dctx.question", dctx.Req.Question[0].Name).Str("cfg", s.Config.Server.DnsCheckDomain).Msg("Checking if DNS check handler should be executed")
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
