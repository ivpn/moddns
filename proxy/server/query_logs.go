package server

import (
	"strconv"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/internal/dnssec"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

// appendReason returns a new slice with r appended, without mutating existing
// (which is shared with the request context's FilterResult).
func appendReason(existing []string, r string) []string {
	out := make([]string, len(existing), len(existing)+1)
	copy(out, existing)
	return append(out, r)
}

func (s *Server) EmitQueryLog(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	defer sentry.Recover()

	// Drain any captured DNSSEC-failure EDE for this request unconditionally (even
	// if logging is disabled) so the edeStore never leaks entries.
	_, dnssecFailed := s.edeStore.Take(dctx.Req)

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
			clientIP = dctx.Addr.Addr().String()
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

		if dnssecFailed {
			queryLog.Reasons = appendReason(queryLog.Reasons, dnssec.ReasonFailed)
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
