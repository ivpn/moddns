package filter

import (
	"context"
	"fmt"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

const (
	SUBDOMAINS_RULE   = "blocklists_subdomains_rule"
	REASON_BLOCKLISTS = "blocklists"
)

// blocklistMatch describes which blocklist matched a domain and whether the
// match came from the parent-domain walk rather than an exact entry.
type blocklistMatch struct {
	blocklistID string
	viaParent   bool
}

// matchDomainAgainstBlocklists checks fqdn (lowercase, no trailing dot; RFC
// 4343 comparison is case-insensitive and blocklist members are stored
// lowercased) against the given subscribed blocklists: exact membership first,
// then the parent-domain walk when the profile's subdomains rule is set to
// block. Lists are checked in subscription order and the first hit wins.
// Returns nil when nothing matches. Shared between the domain phase (QNAME)
// and the IP phase (CNAME targets).
func matchDomainAgainstBlocklists(ctx context.Context, c cache.Cache, reqCtx *requestcontext.RequestContext, blocklists []string, fqdn string) (*blocklistMatch, error) {
	for _, blocklistId := range blocklists {
		match, err := matchDomainAgainstBlocklist(ctx, c, reqCtx, blocklistId, fqdn)
		if err != nil {
			return nil, err
		}
		if match == nil {
			continue
		}

		// The list's own exception set (its @@ rules) withdraws the match;
		// scoped per source, so the remaining subscribed lists still get
		// checked. This consult lives inside the stage on purpose: the
		// aggregator resolves any Allow over every Block, so a list-level
		// allow stage would override user custom block rules.
		excepted, err := matchDomainAgainstExceptions(ctx, c, blocklistId, fqdn)
		if err != nil {
			return nil, err
		}
		if excepted {
			e := reqCtx.Logger.Debug().Str("blocklist", blocklistId)
			reqCtx.MaybeDomain(e, "domain", fqdn).Msg("Blocklist match withdrawn by the list's exception")
			continue
		}
		return match, nil
	}
	return nil, nil
}

// matchDomainAgainstBlocklist checks fqdn against a single list: exact
// membership first, then the parent-domain walk when the profile's subdomains
// rule is set to block.
func matchDomainAgainstBlocklist(ctx context.Context, c cache.Cache, reqCtx *requestcontext.RequestContext, blocklistId string, fqdn string) (*blocklistMatch, error) {
	// check exact match first
	blocklisted, err := c.GetBlocklistEntry(ctx, blocklistId, fqdn)
	if err != nil {
		return nil, err
	}
	if blocklisted {
		return &blocklistMatch{blocklistID: blocklistId}, nil
	}

	if reqCtx.PrivacySettings[SUBDOMAINS_RULE] == RULE_BLOCK {
		// iterate over all parent domains, excluding the TLD and the full
		// FQDN (already covered by the exact-match check above)
		parts := strings.Split(fqdn, ".")
		var candidate string
		for i := len(parts) - 2; i >= 1; i-- {
			// Build candidate incrementally by prepending current part
			if i == len(parts)-2 {
				candidate = parts[i] + "." + parts[i+1]
			} else {
				candidate = parts[i] + "." + candidate
			}

			// now, check if candidate domain is part of any blocklist entry
			blocklisted, err = c.GetBlocklistEntry(ctx, blocklistId, candidate)
			if err != nil {
				return nil, err
			}
			e := reqCtx.Logger.Trace().Bool("blocklisted", blocklisted).Str("blocklist", blocklistId)
			reqCtx.MaybeDomain(e, "candidate", candidate).Msg("Candidate domain")

			if blocklisted {
				return &blocklistMatch{blocklistID: blocklistId, viaParent: true}, nil
			}
		}
	}
	return nil, nil
}

// matchDomainAgainstExceptions reports whether fqdn or any of its parent
// domains (down to two labels) is in the list's exception set. The walk is
// unconditional: an adblock exception (`@@||d^`) covers d and its subdomains
// regardless of the profile's blocklists_subdomains_rule. Runs only on the
// would-block path, so its lookups never touch the common allow path.
func matchDomainAgainstExceptions(ctx context.Context, c cache.Cache, blocklistId string, fqdn string) (bool, error) {
	parts := strings.Split(fqdn, ".")
	for i := 0; i <= len(parts)-2; i++ {
		excepted, err := c.GetBlocklistExceptionEntry(ctx, blocklistId, strings.Join(parts[i:], "."))
		if err != nil {
			return false, err
		}
		if excepted {
			return true, nil
		}
	}
	return false, nil
}

func (f *DomainFilter) filterBlocklists(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()
	blocklists, err := f.Cache.GetProfileBlocklists(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}

	question := dctx.Req.Question[0].Name // answer only first question - google dns does the same

	// DNS preserves query-name case on the wire (RFC 1035 §2.3.3) while name
	// comparison is case-insensitive (RFC 4343), and blocklist members are stored
	// lowercased — so normalise before the byte-exact cache lookup. `question`
	// keeps its original case for logging.
	fqdn, _ := strings.CutSuffix(question, ".")
	fqdn = strings.ToLower(fqdn)

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierBlocklists}

	match, err := matchDomainAgainstBlocklists(context.Background(), f.Cache, reqCtx, blocklists, fqdn)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return result, nil
	}

	reasons := "blocklists"
	msg := "Domain blocked"
	if match.viaParent {
		reasons = fmt.Sprintf("%s,%s", REASON_BLOCKLISTS, SUBDOMAINS_RULE)
		msg = "Subdomain blocked"
	}
	e := reqCtx.Logger.Debug().
		Str("reasons", reasons).
		Str("protocol", string(dctx.Proto)).
		Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype])
	reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
	reqCtx.AddDomain(e, question).Msg(msg)

	result.Decision = model.DecisionBlock
	result.Reasons = append(result.Reasons, "blocklist: "+match.blocklistID)
	if match.viaParent {
		result.Reasons = append(result.Reasons, SUBDOMAINS_RULE)
	}
	return result, nil
}
