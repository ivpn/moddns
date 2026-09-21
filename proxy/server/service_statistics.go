package server

import (
	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
)

// EmitServiceStatistics hands one query's counters to the statistics collector. The
// event carries no profile, device or time: it is summed into a service-wide
// document stamped at flush, so nothing per-profile is ever stored.
func (s *Server) EmitServiceStatistics(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	defer sentry.Recover()

	event := model.EventStatistics{Queries: model.Queries{Total: 1}}
	if reqCtx.FilterResult.Status == model.StatusBlocked {
		event.Queries.Blocked = 1
	}
	if dctx.Res != nil && dctx.Res.AuthenticatedData {
		event.Queries.DNSSEC = 1
	}

	if err := s.CollectorChannels[model.TYPE_STATISTICS].Send(event); err != nil {
		reqCtx.Logger.Err(err).Msg("Failed to send statistics event to channel")
	}
}
