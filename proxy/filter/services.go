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

func (f *IPFilter) filterServices(ctx context.Context, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierServices}
	if f.ServicesCatalog == nil || f.ASNLookup == nil {
		return result, nil
	}
	if dctx == nil || dctx.Res == nil {
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

	blockedSvcs := make(map[string]servicescatalog.Service, len(blockedServices))
	for _, id := range blockedServices {
		if id == "" {
			continue
		}
		if svc, ok := cat.FindByID(id); ok {
			blockedSvcs[svc.ID] = svc
		}
	}
	if len(blockedSvcs) == 0 {
		return result, nil
	}

	matchedServices := make(map[string]struct{})

	ips := extractIPsFromAnswer(dctx.Res.Answer)
	for _, ip := range ips {
		asn, err := f.ASNLookup.ASN(ip)
		if err != nil || asn == 0 {
			continue
		}

		for _, svc := range blockedSvcs {
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

// extractIPsFromAnswer collects IP addresses from DNS answer records.
// Handles A, AAAA, and HTTPS/SVCB records (ipv4hint/ipv6hint parameters).
func extractIPsFromAnswer(answers []dns.RR) []net.IP {
	var ips []net.IP
	for _, rr := range answers {
		switch v := rr.(type) {
		case *dns.A:
			ips = append(ips, v.A)
		case *dns.AAAA:
			ips = append(ips, v.AAAA)
		case *dns.HTTPS:
			ips = append(ips, extractIPsFromSVCB(v.Value)...)
		case *dns.SVCB:
			ips = append(ips, extractIPsFromSVCB(v.Value)...)
		}
	}
	return ips
}

// extractIPsFromSVCB extracts ipv4hint and ipv6hint IPs from SVCB key-value pairs.
func extractIPsFromSVCB(values []dns.SVCBKeyValue) []net.IP {
	var ips []net.IP
	for _, kv := range values {
		switch v := kv.(type) {
		case *dns.SVCBIPv4Hint:
			ips = append(ips, v.Hint...)
		case *dns.SVCBIPv6Hint:
			ips = append(ips, v.Hint...)
		}
	}
	return ips
}
