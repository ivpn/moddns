package filter

import (
	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
)

const (
	RULE_BLOCK   = "block"
	RULE_ALLOW   = "allow"
	DEFAULT_RULE = "default_rule"
)

func (f *DomainFilter) applyDefaultRule(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	// Privacy settings already travel on the request context; no store read.
	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierDefaultRule}
	if reqCtx.PrivacySettings[DEFAULT_RULE] == RULE_BLOCK {
		result.Decision = model.DecisionBlock
		result.Reasons = append(result.Reasons, DEFAULT_RULE)
		reqCtx.Logger.Debug().Msg("Applied default block rule")
	}
	return result, nil
}
