package filter

import (
	"sync"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

type IPFilter struct {
	Cache           cache.Cache
	Proxy           *proxy.Proxy
	ServicesCatalog ServicesCatalogGetter
	ASNLookup       ASNLookup
	RebindingConfig *config.RebindingConfig
	FilteringConfig *config.FilteringConfig
	// Metrics receives per-stage failures; nil disables the metric.
	Metrics      StageErrorRecorder
	patternCache sync.Map
	stages       []stage
}

// NewIPFilter creates a new IPFilter instance. A nil filteringConfig means all
// master switches take their defaults (CNAME uncloaking enabled).
func NewIPFilter(dnsProxy *proxy.Proxy, cache cache.Cache, servicesCatalog ServicesCatalogGetter, asnLookup ASNLookup, rebindingConfig *config.RebindingConfig, filteringConfig *config.FilteringConfig) *IPFilter {
	fltrManager := &IPFilter{
		Cache:           cache,
		Proxy:           dnsProxy,
		ServicesCatalog: servicesCatalog,
		ASNLookup:       asnLookup,
		RebindingConfig: rebindingConfig,
		FilteringConfig: filteringConfig,
	}
	fltrManager.stages = []stage{
		{StageServices, fltrManager.filterServices},
		{StageRebinding, fltrManager.filterRebinding},
		{StageCustomRules, fltrManager.filterCustomRules},
		{StageCNAME, fltrManager.filterCNAME},
	}
	return fltrManager
}

// Execute performs all stages of filtering DNS responses. Any stage failure
// yields StatusUnavailable even though an upstream answer exists: an answer
// that could not be checked is not returned.
func (f *IPFilter) Execute(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (err error) {
	err = runStages(FilterTypeIP, f.stages, f.Metrics, reqCtx, dctx)

	var finalFltrRes model.FilterResult
	if err != nil {
		finalFltrRes = model.FilterResult{Status: model.StatusUnavailable}
	} else {
		finalFltrRes = getFinalFilteringResult(reqCtx.PartialFilteringResults)
	}
	e := reqCtx.Logger.Debug().Str("Query status", string(finalFltrRes.Status)).Strs("Reasons", finalFltrRes.Reasons).Str("qtype", dns.Type(dctx.Req.Question[0].Qtype).String()).Str("filter_type", FilterTypeIP)
	reqCtx.AddClientIP(e, dctx.Addr.Addr().String())
	reqCtx.AddDomain(e, dctx.Req.Question[0].Name).Msg("Final filtering result")
	reqCtx.FilterResult = finalFltrRes

	return err
}
