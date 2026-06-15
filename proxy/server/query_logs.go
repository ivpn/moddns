package server

import (
	"strconv"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

func (s *Server) EmitQueryLog(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	defer sentry.Recover()

	// Use the contextual logger from the request context
	logger := reqCtx.Logger

	logsSettings := reqCtx.LogsSettings
	// Defensive: if not present, nothing to emit
	if logsSettings == nil {
		return
	}

	// Parse booleans ignoring errors (defaults to false on failure).
	// Rationale: EmitQueryLog is a best-effort, non-critical path. We deliberately
	// avoid extra branches / error noise for malformed or missing log settings.
	// Any parsing failure simply disables the specific logging facet (domains or client IP),
	// preserving privacy by default. If future troubleshooting requires visibility,
	// reintroduce explicit error logging or metrics here.
	loggingEnabled, _ := strconv.ParseBool(logsSettings["enabled"])
	clientIPsLoggingEnabled, _ := strconv.ParseBool(logsSettings["log_clients_ips"])
	domainLoggingEnabled, _ := strconv.ParseBool(logsSettings["log_domains"])

	var clientIP, domain string
	if loggingEnabled {
		if clientIPsLoggingEnabled {
			// Unmap IPv4-mapped IPv6 (::ffff:a.b.c.d) so v4 clients on the dual-stack DoT/DoQ
			// listeners are logged as native IPv4, consistent with IPv4-listener records.
			clientIP = dctx.Addr.Addr().Unmap().String()
		}
		if domainLoggingEnabled {
			domain = dctx.Req.Question[0].Name
		}

		queryLog := model.QueryLog{
			Timestamp: time.Now(),
			ProfileID: reqCtx.ProfileId,
			DeviceId:  reqCtx.DeviceId,
			Status:    string(reqCtx.FilterResult.Status),
			Reasons:   reqCtx.FilterResult.Reasons,
			DNSRequest: model.DNSRequest{
				Domain:    domain,
				QueryType: dns.TypeToString[dctx.Req.Question[0].Qtype],
			},
			ClientIP: clientIP,
			Protocol: string(dctx.Proto),
		}
		if dctx.Res != nil {
			queryLog.DNSRequest.ResponseCode = dns.RcodeToString[dctx.Res.Rcode]
			queryLog.DNSRequest.DNSSEC = dctx.Res.AuthenticatedData
		}
		retention := model.Retention(logsSettings["retention"])
		// send event to channel
		if sendErr := s.CollectorChannels[model.TYPE_QUERY_LOGS].Send(
			model.EventQueryLog{
				QueryLog: queryLog,
				Metadata: model.Metadata{
					Retention: retention,
				},
			},
		); sendErr != nil {
			logger.Err(sendErr).Msg("Error sending query log event")
		}
	}
}
