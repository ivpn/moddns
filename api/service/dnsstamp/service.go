// Package dnsstamp generates DNS Stamps (sdns:// strings) for modDNS profiles.
//
// Stamps are a compact, self-describing format consumed by clients that don't
// expose separate hostname/path/port fields — UniFi Network, dnscrypt-proxy,
// AdGuard Home upstreams, etc. See https://dnscrypt.info/stamps-specifications.
//
// Per-profile DoH/DoT/DoQ stamps are generated for the active modDNS profile,
// optionally scoped to a specific device label. DNSCrypt stamps are out of
// scope until the proxy gains DNSCrypt server-mode support.
package dnsstamp

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/libs/deviceid"
	"github.com/ivpn/dns/libs/dnsstamps"
	"github.com/ivpn/dns/libs/dohpath"
)

// defaultProps describes modDNS to clients: DNSSEC-validating, no logs, but we
// do filter (so NoFilter is intentionally NOT set). Setting NoFilter would be
// inaccurate advertising and harm clients deciding which resolvers to trust.
const defaultProps = dnsstamps.ServerInformalPropertyDNSSEC | dnsstamps.ServerInformalPropertyNoLog

// ErrNoServerAddress is returned when no anycast IP is configured. This should
// be caught at startup via config validation but is surfaced defensively in case
// a degenerate config slips through.
var ErrNoServerAddress = errors.New("dnsstamp: no anycast server address configured")

// DNSStampServicer is the public surface of the stamp service.
type DNSStampServicer interface {
	GenerateStamps(ctx context.Context, req requests.DNSStampReq) (responses.DNSStampResponse, error)
}

// DNSStampService builds DoH/DoT/DoQ stamps for a given profile.
//
// All fields are derived from config at construction time. The service holds
// no mutable state and is safe for concurrent use.
type DNSStampService struct {
	Domain      string                            // cfg.Server.DnsDomain, e.g. "dns.moddns.net"
	PrimaryIPv4 string                            // cfg.Server.ServerAddresses[0]
	DoTPort     int                               // cfg.Server.DoTPort (production: 853)
	DoQPort     int                               // cfg.Server.DoQPort (production: 853, NOT the library default 784)
	Props       dnsstamps.ServerInformalProperties
}

// NewDNSStampService constructs the service from config. If no anycast
// addresses are configured, PrimaryIPv4 will be empty and GenerateStamps
// returns ErrNoServerAddress on every call — startup validation should
// catch that earlier.
func NewDNSStampService(cfg *config.Config) DNSStampService {
	primary := ""
	if cfg != nil && cfg.Server != nil && len(cfg.Server.ServerAddresses) > 0 {
		primary = cfg.Server.ServerAddresses[0]
	}
	domain := ""
	dotPort, doqPort := 0, 0
	if cfg != nil && cfg.Server != nil {
		domain = cfg.Server.DnsDomain
		dotPort = cfg.Server.DoTPort
		doqPort = cfg.Server.DoQPort
	}
	return DNSStampService{
		Domain:      domain,
		PrimaryIPv4: primary,
		DoTPort:     dotPort,
		DoQPort:     doqPort,
		Props:       defaultProps,
	}
}

// GenerateStamps returns DoH, DoT, and DoQ sdns:// strings for the given
// profile (and optional device label). The caller is responsible for
// authentication and profile-ownership checks before invoking this.
//
// Stamps are reproducible — the same (profile, device) pair always yields the
// same three strings. Spec contract is locked in via libs/dohpath for the DoH
// path and clientid.go's SNI format for DoT/DoQ.
func (s DNSStampService) GenerateStamps(_ context.Context, req requests.DNSStampReq) (responses.DNSStampResponse, error) {
	if s.PrimaryIPv4 == "" {
		return responses.DNSStampResponse{}, ErrNoServerAddress
	}
	if s.Domain == "" {
		return responses.DNSStampResponse{}, errors.New("dnsstamp: no server domain configured")
	}

	// Device id arrives validated by the request validator (`device_id` tag → deviceid.Normalize).
	// We re-encode for each transport: URL-percent for DoH path, label form for DoT/DoQ SNI.
	deviceURL := deviceid.EncodeURL(req.DeviceId)
	deviceLabel := deviceid.EncodeLabel(req.DeviceId)

	// DoH — profile (+ optional device) lives in the URL path. DoH default port
	// is 443; the dnsstamps encoder strips :443 if present, so we omit it.
	dohStamp := dnsstamps.ServerStamp{
		Proto:         dnsstamps.StampProtoTypeDoH,
		Props:         s.Props,
		ServerAddrStr: s.PrimaryIPv4,
		ProviderName:  s.Domain,
		Path:          dohpath.For(req.ProfileId, req.DeviceId),
	}
	_ = deviceURL // captured implicitly via dohpath.For

	// DoT — profile (+ optional device) lives in the TLS SNI hostname.
	// Format mirrors proxy/server/clientid.go SNI parsing:
	//   <profile>.<domain>                       (no device)
	//   <encoded-device>-<profile>.<domain>      (with device, hyphen separator)
	dotSNI := req.ProfileId + "." + s.Domain
	if req.DeviceId != "" {
		dotSNI = deviceLabel + "-" + req.ProfileId + "." + s.Domain
	}

	// Port handling: the dnsstamps encoder strips a port suffix iff it equals
	// the library's hardcoded default (DoT=843, DoQ=784, DoH=443). modDNS's
	// production DoT/DoQ ports (typically both 853) do not match those defaults,
	// so we always include them explicitly. Defensive against future library
	// default changes too.
	dotStamp := dnsstamps.ServerStamp{
		Proto:         dnsstamps.StampProtoTypeTLS,
		Props:         s.Props,
		ServerAddrStr: s.PrimaryIPv4 + ":" + strconv.Itoa(s.DoTPort),
		ProviderName:  dotSNI,
	}
	doqStamp := dnsstamps.ServerStamp{
		Proto:         dnsstamps.StampProtoTypeDoQ,
		Props:         s.Props,
		ServerAddrStr: s.PrimaryIPv4 + ":" + strconv.Itoa(s.DoQPort),
		ProviderName:  dotSNI, // DoT and DoQ share the SNI format
	}

	resp := responses.DNSStampResponse{
		DoH: dohStamp.String(),
		DoT: dotStamp.String(),
		DoQ: doqStamp.String(),
	}

	// Defensive sanity: every produced string must round-trip via the library.
	// If it doesn't, returning a broken stamp to the client would silently fail
	// downstream — fail loud here instead.
	for proto, s := range map[string]string{"doh": resp.DoH, "dot": resp.DoT, "doq": resp.DoQ} {
		if _, err := dnsstamps.NewServerStampFromString(s); err != nil {
			return responses.DNSStampResponse{}, fmt.Errorf("dnsstamp: %s stamp failed round-trip: %w", proto, err)
		}
	}

	return resp, nil
}
