package filter

import (
	"context"
	"sync"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

type DomainFilter struct {
	Proxy          *proxy.Proxy
	Cache          cache.Cache
	patternCache   sync.Map
	FilteringFuncs []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error)
}

// NewDNSFilterManager creates a new DNSFilterManager instance
func NewDomainFilter(dnsProxy *proxy.Proxy, cache cache.Cache) *DomainFilter {
	fltrManager := &DomainFilter{
		Cache: cache,
		Proxy: dnsProxy,
	}
	fltrManager.FilteringFuncs = []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error){
		fltrManager.filterBlocklists,
		fltrManager.filterCustomRules,
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
	reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
	reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("Final filtering result")
	reqCtx.FilterResult = finalFltrRes

	return nil
}
