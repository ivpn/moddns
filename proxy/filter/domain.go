package filter

import (
	"context"
	"strings"
	"sync"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

type DomainFilter struct {
	Proxy           *proxy.Proxy
	Cache           cache.Cache
	ServicesCatalog ServicesCatalogGetter
	// Metrics receives per-stage failures; nil disables the metric.
	Metrics      StageErrorRecorder
	patternCache sync.Map
	stages       []stage
}

// NewDomainFilter creates a new DomainFilter instance.
// servicesCatalog may be nil if service domain blocking is not available.
func NewDomainFilter(dnsProxy *proxy.Proxy, cache cache.Cache, servicesCatalog ServicesCatalogGetter) *DomainFilter {
	fltrManager := &DomainFilter{
		Cache:           cache,
		Proxy:           dnsProxy,
		ServicesCatalog: servicesCatalog,
	}
	fltrManager.stages = []stage{
		{StageBlocklists, fltrManager.filterBlocklists},
		{StageCustomRules, fltrManager.filterCustomRules},
		{StageServiceDomains, fltrManager.filterServiceDomains},
		{StageDefaultRule, fltrManager.applyDefaultRule},
	}
	return fltrManager
}

// Execute performs all stages of filtering DNS requests. Any stage failure
// yields StatusUnavailable: the partial results of the other stages are kept
// for logging but never aggregated into a decision.
func (f *DomainFilter) Execute(ctx context.Context, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (err error) {
	err = runStages(ctx, FilterTypeDomain, f.stages, f.Metrics, reqCtx, dctx)

	var finalFltrRes model.FilterResult
	if err != nil {
		finalFltrRes = model.FilterResult{Status: model.StatusUnavailable}
	} else {
		finalFltrRes = getFinalFilteringResult(reqCtx.PartialFilteringResults)
	}
	e := reqCtx.Logger.Debug().Str("Query status", string(finalFltrRes.Status)).Strs("reasons", finalFltrRes.Reasons).Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("filter_type", FilterTypeDomain)
	reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
	reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("Final filtering result")
	reqCtx.FilterResult = finalFltrRes

	return err
}

// filterServiceDomains blocks queries for domains listed in the services
// catalog. This runs in the domain phase (pre-resolve) to catch traffic
// that ASN-based blocking misses when services use third-party CDNs.
// Subdomain matching is always on: listing "microsoft.com" also blocks
// "www.microsoft.com", "login.microsoft.com", etc.
func (f *DomainFilter) filterServiceDomains(ctx context.Context, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierServices}
	if f.ServicesCatalog == nil {
		return result, nil
	}

	blockedServices := reqCtx.BlockedServices
	if len(blockedServices) == 0 {
		return result, nil
	}

	// The catalog is a local file, not the settings store: failing to load it
	// leaves the stage inert instead of failing the query.
	cat, err := f.ServicesCatalog.Get()
	if err != nil || cat == nil {
		return result, nil
	}

	domainMap := cat.DomainMapForServiceIDs(blockedServices)
	if len(domainMap) == 0 {
		return result, nil
	}

	fqdn, _ := strings.CutSuffix(dctx.Req.Question[0].Name, ".")
	fqdn = strings.ToLower(fqdn)

	// Check exact match, then parent domains (subdomain matching).
	parts := strings.Split(fqdn, ".")
	for i := range parts {
		candidate := strings.Join(parts[i:], ".")
		if svcID, ok := domainMap[candidate]; ok {
			result.Decision = model.DecisionBlock
			result.Reasons = append(result.Reasons, REASON_SERVICES, "service: "+svcID)
			return result, nil
		}
	}

	return result, nil
}
