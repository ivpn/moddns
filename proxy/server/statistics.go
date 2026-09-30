package server

import (
	"strconv"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
)

// EmitStatistics hands one query's counters to the statistics collector. The
// event carries no time. Profile, device, reason class and protocol travel with it only
// when the profile has statistics enabled.
func (s *Server) EmitStatistics(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) {
	defer sentry.Recover()

	event := model.EventStatistics{Queries: model.Queries{Total: 1}}
	if reqCtx.FilterResult.Status == model.StatusBlocked {
		event.Queries.Blocked = 1
	}
	if dctx.Res != nil && dctx.Res.AuthenticatedData {
		event.Queries.DNSSEC = 1
	}

	if statisticsConsented(reqCtx.StatisticsSettings) {
		event.Consented = &model.ConsentedStatistics{
			ProfileID: reqCtx.ProfileId,
			DeviceID:  reqCtx.DeviceId,
			Retention: model.Retention(reqCtx.StatisticsSettings["retention"]),
			Protocol:  statisticsProtocol(dctx.Proto),
		}
		if event.Queries.Blocked == 1 {
			event.Consented.ReasonClass = model.ClassifyReasons(reqCtx.FilterResult.Reasons)
		}
	}

	if err := s.CollectorChannels[model.TYPE_STATISTICS].Send(event); err != nil {
		reqCtx.Logger.Err(err).Msg("Failed to send statistics event to channel")
	}
}

// statisticsProtocol counts the encrypted transports only.
func statisticsProtocol(p proxy.Proto) model.StatisticsProtocol {
	switch p {
	case proxy.ProtoHTTPS:
		return model.ProtocolDoH
	case proxy.ProtoTLS:
		return model.ProtocolDoT
	case proxy.ProtoQUIC:
		return model.ProtocolDoQ
	default:
		return ""
	}
}

// statisticsConsented parses the flag the API stores; the empty check keeps the
// common no-settings case free of ParseBool's error allocation.
func statisticsConsented(settings map[string]string) bool {
	v := settings["enabled"]
	if v == "" {
		return false
	}
	on, _ := strconv.ParseBool(v)
	return on
}
