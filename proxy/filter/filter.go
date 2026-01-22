package filter

import (
	"errors"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/requestcontext"
)

type Filter interface {
	Execute(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) error
}

const (
	FilterTypeDomain = "domain"
	FilterTypeIP     = "ip"
)

func NewFilter(proxy *proxy.Proxy, cache cache.Cache, filterType string) (Filter, error) {
	switch filterType { // nolint
	case FilterTypeDomain:
		return NewDomainFilter(proxy, cache), nil
	case FilterTypeIP:
		return NewIPFilter(proxy, cache, nil, nil), nil
	}
	return nil, errors.New("unknown filter type")
}
