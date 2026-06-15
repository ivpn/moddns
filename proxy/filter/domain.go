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
	"golang.org/x/sync/errgroup"
)

type DomainFilter struct {
	Proxy           *proxy.Proxy
	Cache           cache.Cache
	ServicesCatalog ServicesCatalogGetter
	patternCache    sync.Map
	FilteringFuncs  []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error)
}

// NewDomainFilter creates a new DomainFilter instance.
// servicesCatalog may be nil if service domain blocking is not available.
func NewDomainFilter(dnsProxy *proxy.Proxy, cache cache.Cache, servicesCatalog ServicesCatalogGetter) *DomainFilter {
	fltrManager := &DomainFilter{
		Cache:           cache,
		Proxy:           dnsProxy,
		ServicesCatalog: servicesCatalog,
	}
	fltrManager.FilteringFuncs = []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error){
		fltrManager.filterBlocklists,
		fltrManager.filterCustomRules,
		fltrManager.filterServiceDomains,
		fltrManager.applyDefaultRule,
	}
	return fltrManager
}

// Execute performs all stages of filtering DNS requests
func (f *DomainFilter) Execute(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (err error) {
	ctx := context.Background()
	eg, egCtx := errgroup.WithContext(ctx)
	resultChan := make(chan *model.StageResult, len(f.FilteringFuncs))
	for _, fltrFunc := range f.FilteringFuncs {
		func(ctx context.Context, reqCtx *requestcontext.RequestContext) {
			eg.Go(func() error {
				fltrRes, err := fltrFunc(reqCtx, dctx)
				if err != nil {
					return err
				}
				resultChan <- fltrRes
				return nil
			})
		}(egCtx, reqCtx)
	}
	if err := eg.Wait(); err != nil {
		reqCtx.Logger.Err(err).Msg("Error filtering DNS requests")
	}
	close(resultChan)

	for res := range resultChan {
		reqCtx.PartialFilteringResults = append(reqCtx.PartialFilteringResults, *res)
	}
	finalFltrRes := getFinalFilteringResult(reqCtx.PartialFilteringResults)
	e := reqCtx.Logger.Debug().Str("Query status", string(finalFltrRes.Status)).Strs("reasons", finalFltrRes.Reasons).Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("filter_type", FilterTypeDomain)
	reqCtx.AddClientIP(e, dctx.Addr.Addr().Unmap().String())
	reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("Final filtering result")
	reqCtx.FilterResult = finalFltrRes

	return nil
}

// filterServiceDomains blocks queries for domains listed in the services
// catalog. This runs in the domain phase (pre-resolve) to catch traffic
// that ASN-based blocking misses when services use third-party CDNs.
// Subdomain matching is always on: listing "microsoft.com" also blocks
// "www.microsoft.com", "login.microsoft.com", etc.
func (f *DomainFilter) filterServiceDomains(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierServices}
	if f.ServicesCatalog == nil {
		return result, nil
	}

	blockedServices, err := f.Cache.GetProfileServicesBlocked(context.Background(), reqCtx.ProfileId)
	if err != nil || len(blockedServices) == 0 {
		return result, nil
	}

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
