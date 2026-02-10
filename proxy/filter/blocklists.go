package filter

import (
	"context"
	"fmt"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

const (
	SUBDOMAINS_RULE   = "blocklists_subdomains_rule"
	REASON_BLOCKLISTS = "blocklists"
)

func (f *DomainFilter) filterBlocklists(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()
	blocklists, err := f.Cache.GetProfileBlocklists(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}

	question := dctx.Req.Question[0].Name // answer only first question - google dns does the same
	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierBlocklists}
	for _, blocklistId := range blocklists {

		fqdn, _ := strings.CutSuffix(question, ".")
		// check exact match first
		blocklisted, err := f.Cache.GetBlocklistEntry(context.Background(), blocklistId, fqdn)
		if err != nil {
			return nil, err
		}

		if blocklisted {
			e := reqCtx.Logger.Debug().
				Str("reasons", "blocklists").
				Str("protocol", string(dctx.Proto)).
				Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype])
			reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
			reqCtx.AddDomain(e, question).Msg("Domain blocked")
			result.Decision = model.DecisionBlock
			result.Reasons = append(result.Reasons, "blocklist: "+blocklistId)
			return result, nil
		}

		if reqCtx.PrivacySettings[SUBDOMAINS_RULE] == RULE_BLOCK {
			// iterate over all subdomains
			parts := strings.Split(fqdn, ".")
			for i := range len(parts) - 1 {
				candidate := strings.Join(parts[i:], ".")
				// now, check if candidate domain is part of any blocklist entry
				blocklisted, err = f.Cache.GetBlocklistEntry(context.Background(), blocklistId, candidate)
				if err != nil {
					return nil, err
				}
				e := reqCtx.Logger.Trace().Bool("blocklisted", blocklisted).Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype]).Str("blocklist", blocklistId)
				reqCtx.MaybeDomain(e, "candidate", candidate).Msg("Candidate domain")

				if blocklisted {
					e := reqCtx.Logger.Debug().
						Str("reasons", fmt.Sprintf("%s,%s", REASON_BLOCKLISTS, SUBDOMAINS_RULE)).
						Str("protocol", string(dctx.Proto)).
						Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype])
					reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
					reqCtx.AddDomain(e, question).Msg("Subdomain blocked")
					result.Decision = model.DecisionBlock
					result.Reasons = append(result.Reasons, "blocklist: "+blocklistId)
					result.Reasons = append(result.Reasons, SUBDOMAINS_RULE)
					return result, nil
				}
			}
		}
	}
	return result, nil
}
