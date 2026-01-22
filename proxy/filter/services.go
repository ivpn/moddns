package filter

import (
	"context"
	"net"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/libs/servicescatalog"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
)

const (
	REASON_SERVICES = "services"
)

type ServicesCatalogGetter interface {
	Get() (*servicescatalog.Catalog, error)
}

type ASNLookup interface {
	ASN(ip net.IP) (uint, error)
}

func (f *IPFilter) filterServices(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierServices}
	if f.ServicesCatalog == nil || f.ASNLookup == nil {
		return result, nil
	}
	if dctx == nil || dctx.Res == nil {
		return result, nil
	}

	blockedServices, err := f.Cache.GetProfileServicesBlocked(context.Background(), reqCtx.ProfileId)
	if err != nil {
		// Missing key should be non-fatal; treat as disabled.
		return result, nil
	}
	if len(blockedServices) == 0 {
		return result, nil
	}

	cat, err := f.ServicesCatalog.Get()
	if err != nil || cat == nil {
		// Catalog load failure should not break DNS.
		return result, nil
	}

	blockedSet := make(map[string]struct{}, len(blockedServices))
	for _, id := range blockedServices {
		if id == "" {
			continue
		}
		blockedSet[id] = struct{}{}
	}
	if len(blockedSet) == 0 {
		return result, nil
	}

	matchedServices := make(map[string]struct{})

	for _, rr := range dctx.Res.Answer {
		var ip net.IP
		switch v := rr.(type) {
		case *dns.A:
			ip = v.A
		case *dns.AAAA:
			ip = v.AAAA
		default:
			continue
		}
		asn, err := f.ASNLookup.ASN(ip)
		if err != nil || asn == 0 {
			continue
		}

		for _, svc := range cat.Services {
			if _, ok := blockedSet[svc.ID]; !ok {
				continue
			}
			for _, svcASN := range svc.ASNs {
				if svcASN == asn {
					matchedServices[svc.ID] = struct{}{}
					break
				}
			}
		}
	}

	if len(matchedServices) == 0 {
		return result, nil
	}

	result.Decision = model.DecisionBlock
	result.Reasons = append(result.Reasons, REASON_SERVICES)
	for id := range matchedServices {
		result.Reasons = append(result.Reasons, "service: "+id)
	}
	return result, nil
}
