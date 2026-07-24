package filter

import (
	"context"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

const REASON_CNAME_UNCLOAKING = "cname_uncloaking"

// extractCNAMETargets returns the deduplicated, lowercased CNAME target names
// from an answer section, trailing dots stripped. The QNAME itself is excluded
// — the domain phase already judged it pre-resolve.
func extractCNAMETargets(answers []dns.RR, qname string) []string {
	var targets []string
	var seen map[string]struct{}
	q := strings.ToLower(strings.TrimSuffix(qname, "."))
	for _, rr := range answers {
		cname, ok := rr.(*dns.CNAME)
		if !ok {
			continue
		}
		t := strings.ToLower(strings.TrimSuffix(cname.Target, "."))
		if t == "" || t == q {
			continue
		}
		if seen == nil {
			seen = make(map[string]struct{}, 2)
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		targets = append(targets, t)
	}
	return targets
}

// filterCNAME uncloaks CNAME chains: every target name in the answer is
// re-checked against the profile's subscribed blocklists and custom domain
// rules, closing the gap where a first-party QNAME hides a third-party tracker
// behind a CNAME. Always-on; spec: proxy-filtering-behaviour.md Section F.
//
// The stage returns a single StageResult, so it applies the same precedence
// internally that the aggregator applies globally: custom Allow (T200) >
// custom Block (T200) > blocklist Block (T100). Custom rules are therefore
// always evaluated, while blocklist lookups are skipped once a custom rule
// has decided.
func (f *IPFilter) filterCNAME(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierBlocklists}
	if dctx == nil || dctx.Res == nil {
		return result, nil
	}
	targets := extractCNAMETargets(dctx.Res.Answer, dctx.Req.Question[0].Name)
	if len(targets) == 0 {
		return result, nil
	}

	customRuleHashes, err := f.Cache.GetCustomRulesHashes(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}
	allowMatched, blockMatched := false, false
	for _, customRuleHash := range customRuleHashes {
		hash, err := f.Cache.GetCustomRulesHash(context.Background(), customRuleHash)
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			if matchDomainPattern(&f.patternCache, target, hash["value"]) {
				switch hash["action"] {
				case ACTION_BLOCK:
					blockMatched = true
				case ACTION_ALLOW:
					allowMatched = true
				}
			}
		}
	}
	if allowMatched || blockMatched {
		result.Tier = TierCustomRules
		result.Reasons = []string{REASON_CUSTOM_RULES, REASON_CNAME_UNCLOAKING}
		if allowMatched {
			result.Decision = model.DecisionAllow
		} else {
			result.Decision = model.DecisionBlock
		}
		reqCtx.Logger.Debug().
			Str("reasons", REASON_CUSTOM_RULES+","+REASON_CNAME_UNCLOAKING).
			Str("decision", string(result.Decision)).
			Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype]).
			Msg("CNAME target matched custom rule")
		return result, nil
	}

	blocklists, err := f.Cache.GetProfileBlocklists(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		match, err := matchDomainAgainstBlocklists(context.Background(), f.Cache, reqCtx, blocklists, target)
		if err != nil {
			return nil, err
		}
		if match == nil {
			continue
		}
		e := reqCtx.Logger.Debug().
			Str("reasons", REASON_BLOCKLISTS+","+REASON_CNAME_UNCLOAKING).
			Str("blocklist", match.blocklistID).
			Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype])
		reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("CNAME target blocked")
		result.Decision = model.DecisionBlock
		result.Tier = TierBlocklists
		result.Reasons = append(result.Reasons, "blocklist: "+match.blocklistID)
		if match.viaParent {
			result.Reasons = append(result.Reasons, SUBDOMAINS_RULE)
		}
		result.Reasons = append(result.Reasons, REASON_CNAME_UNCLOAKING)
		return result, nil
	}
	return result, nil
}
