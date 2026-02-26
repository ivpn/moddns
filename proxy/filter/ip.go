package filter

import (
	"context"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

type IPFilter struct {
	Cache           cache.Cache
	Proxy           *proxy.Proxy
	ServicesCatalog ServicesCatalogGetter
	ASNLookup       ASNLookup
	// patternCache   sync.Map
	FilteringFuncs []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error)
}

// NewIPFilter creates a new IPFilter instance
func NewIPFilter(dnsProxy *proxy.Proxy, cache cache.Cache, servicesCatalog ServicesCatalogGetter, asnLookup ASNLookup) *IPFilter {
	fltrManager := &IPFilter{
		Cache:           cache,
		Proxy:           dnsProxy,
		ServicesCatalog: servicesCatalog,
		ASNLookup:       asnLookup,
	}
	fltrManager.FilteringFuncs = []func(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error){
		fltrManager.filterServices,
		fltrManager.filterCustomRules,
	}
	return fltrManager
}

// Execute performs all stages of filtering DNS requests
func (f *IPFilter) Execute(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (err error) {
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
		reqCtx.Logger.Err(err).Msg("Error filtering IP address in DNS response")
	}
	close(resultChan)

	var ipResults []model.StageResult
	for res := range resultChan {
		ipResults = append(ipResults, *res)
		reqCtx.PartialFilteringResults = append(reqCtx.PartialFilteringResults, *res)
	}

	finalFltrRes := getFinalFilteringResult(ipResults)
	e := reqCtx.Logger.Debug().Str("Query status", string(finalFltrRes.Status)).Strs("Reasons", finalFltrRes.Reasons).Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("filter_type", FilterTypeIP)
	reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
	reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("Final filtering result")
	// save the final filtering result to the request context once, only in IP filtering phase?
	reqCtx.FilterResult = finalFltrRes

	return nil
}
