package server

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/internal/dnssec"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

// Resolution-outcome tokens stored in QueryLog.Outcome.
// Decision table: docs/specs/query-log-outcomes-behaviour.md (rows O1-O10).
const (
	OutcomeResolved       = "resolved"          // O1: NOERROR with answer records
	OutcomeNoData         = "nodata"            // O2: NOERROR, empty answer
	OutcomeNXDomain       = "nxdomain"          // O3
	OutcomeBlocked        = "blocked"           // O4
	OutcomeServfailDNSSEC = "servfail_dnssec"   // O5
	OutcomeServfailUpstrm = "servfail_upstream" // O6
	OutcomeTimeout        = "timeout"           // O7
	OutcomeNetworkError   = "network_error"     // O8
	OutcomeRefused        = "refused"           // O9
)

// classifyOutcome maps a completed request to a resolution-outcome token.
// Precedence (spec rows O1-O10): blocked first, then transport errors captured
// from the vendor resolve call, then rcode-based outcomes, then answer content.
// Returns "" (unknown) only for the defensive nil-response-without-error case.
func classifyOutcome(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext, dnssecFailed bool) string {
	// O4 — a filter block always wins: the synthesized response is deliberate.
	if reqCtx.FilterResult.Status == model.StatusBlocked {
		return OutcomeBlocked
	}

	// O7 / O8 — the vendor resolve call failed; the client-visible SERVFAIL was
	// synthesized locally, so the transport error is the truthful outcome.
	if err := reqCtx.UpstreamErr; err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return OutcomeTimeout
		}
		return OutcomeNetworkError
	}

	// O10 — defensive: nothing to classify.
	if dctx.Res == nil {
		return ""
	}

	switch dctx.Res.Rcode {
	case dns.RcodeNameError:
		return OutcomeNXDomain // O3
	case dns.RcodeServerFailure:
		if dnssecFailed {
			return OutcomeServfailDNSSEC // O5
		}
		return OutcomeServfailUpstrm // O6
	case dns.RcodeRefused:
		return OutcomeRefused // O9
	case dns.RcodeSuccess:
		if len(dctx.Res.Answer) > 0 {
			return OutcomeResolved // O1
		}
		return OutcomeNoData // O2
	}
	return ""
}

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
			Timestamp: time.Now().UTC(),
			ProfileID: reqCtx.ProfileId,
			DeviceId:  reqCtx.DeviceId,
			Status:    string(reqCtx.FilterResult.Status),
			Reasons:   reqCtx.FilterResult.Reasons,
			Outcome:   classifyOutcome(reqCtx, dctx, dnssecFailed),
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
