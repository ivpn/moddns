package server

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/netutil"
	"github.com/AdguardTeam/golibs/service"
	"github.com/ivpn/dns/proxy/config"
	"github.com/rs/zerolog/log"
)

const (
	ProxyTypeAdguard = "adguard"
)

var _ service.Interface = (*proxy.Proxy)(nil)

func (s *Server) newProxy(proxyType string, serverConfig *config.Config) (dnsProxy *proxy.Proxy, err error) {
	switch proxyType {
	case ProxyTypeAdguard:
		config, err := s.newProxyConfig(serverConfig)
		if err != nil {
			return nil, err
		}

		dnsProxy, err = proxy.New(config)
		if err != nil {
			log.Fatal().AnErr("creating proxy: %s", err).Msg("Failed to create proxy")
		}
	default:
		return nil, errors.New("unknown proxy type")
	}

	return dnsProxy, nil
}

// This is Interface from library "github.com/AdguardTeam/golibs/service"
// Proxy implementation must satisfy this interface
// type Interface interface {
// 	// Start starts the service.  ctx is used for cancelation.
// 	//
// 	// It is recommended that Start returns only after the service has
// 	// completely finished its initialization.  If that cannot be done, the
// 	// implementation of Start must document that.
// 	Start(ctx context.Context) (err error)

// 	// Shutdown gracefully stops the service.  ctx is used to determine
// 	// a timeout before trying to stop the service less gracefully.
// 	//
// 	// It is recommended that Shutdown returns only after the service has
// 	// completely finished its termination.  If that cannot be done, the
// 	// implementation of Shutdown must document that.
// 	Shutdown(ctx context.Context) (err error)
// }

func (s *Server) newProxyConfig(serverConfig *config.Config) (*proxy.Config, error) {
	var defaultResolver *upstream.UpstreamResolver
	defaultUpstreamFound := false
	for name, addr := range serverConfig.Upstream.Upstreams {
		log.Info().Str("name", name).Str("address", addr).Msg("Adding proxy upstream")
		ups, err := upstream.AddressToUpstream(addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create upstream: %w", err)
		}
		upCfg := &proxy.UpstreamConfig{
			Upstreams: []upstream.Upstream{
				ups,
			},
		}
		customUpstreamConfig := proxy.NewCustomUpstreamConfig(
			upCfg, serverConfig.DNSCache.Enabled, serverConfig.DNSCache.Size, false,
		)
		s.Upstreams[name] = customUpstreamConfig

		log.Info().Str("upstream", serverConfig.Upstream.Default).Msg("Proxy upstream settings")
		if name == serverConfig.Upstream.Default {
			defaultResolver, err = upstream.NewUpstreamResolver(addr, nil)
			if err != nil {
				return nil, err
			}
			defaultUpstreamFound = true
		}
	}
	if !defaultUpstreamFound {
		return nil, errors.New("default upstream not found")
	}

	tlsConfig, err := newTLSConfig(0, 0, serverConfig.TLS.CertPaths, serverConfig.TLS.KeyPaths)
	if err != nil {
		return nil, err
	}
	trustedPrefixes := make([]netip.Prefix, 0, len(serverConfig.TrustedProxies))
	for _, cidr := range serverConfig.TrustedProxies {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy subnet %q: %w", cidr, err)
		}
		trustedPrefixes = append(trustedPrefixes, p)
	}

	conf := &proxy.Config{
		UpstreamConfig: &proxy.UpstreamConfig{
			Upstreams: []upstream.Upstream{
				defaultResolver,
			},
		},
		BeforeRequestHandler: s,
		RequestHandler:       s.RequestHandler(),
		ResponseHandler:      s.ResponseHandler(),
		TLSConfig:            tlsConfig,
		TrustedProxies:       trustedProxySet(trustedPrefixes),
		Ratelimit:            0,
	}

	if serverConfig.DNSCache.Enabled {
		conf.CacheEnabled = true
		conf.CacheSizeBytes = serverConfig.DNSCache.SizeBytes
		conf.CacheMinTTL = serverConfig.DNSCache.MinTTL
		conf.CacheMaxTTL = serverConfig.DNSCache.MaxTTL
		conf.CacheOptimistic = serverConfig.DNSCache.Optimistic
	}

	// All listeners bind the unspecified address (nil IP), which Go resolves to "::" dual-stack on
	// IPv6-capable hosts (serving IPv4 as v4-mapped) — so the proxy already accepts both families.
	// Enabling end-to-end IPv6 is therefore a deployment concern (bridge v6 + host routing + AAAA),
	// not a listener change. NOTE: v4 clients on these sockets appear as ::ffff:a.b.c.d — see the
	// .Unmap() at the client-IP use sites (rate limiter, query logs) and trustedProxySet below.
	if serverConfig.PlainDNS.UDPListenAddr != 0 {
		conf.UDPListenAddr = []*net.UDPAddr{{Port: serverConfig.PlainDNS.UDPListenAddr}}
	}
	if serverConfig.PlainDNS.TCPListenAddr != 0 {
		conf.TCPListenAddr = []*net.TCPAddr{{Port: serverConfig.PlainDNS.TCPListenAddr}}
	}
	if serverConfig.DoH.ListenAddr != 0 {
		conf.HTTPSListenAddr = []*net.TCPAddr{{Port: serverConfig.DoH.ListenAddr}}
	}
	if serverConfig.DoQ.ListenAddr != 0 {
		conf.QUICListenAddr = []*net.UDPAddr{{Port: serverConfig.DoQ.ListenAddr}}
	}
	if serverConfig.DoT.ListenAddr != 0 {
		conf.TLSListenAddr = []*net.TCPAddr{{Port: serverConfig.DoT.ListenAddr}}
	}
	return conf, nil
}

// trustedProxySet builds the TrustedProxies matcher used to decide whether to honor X-Forwarded-For
// (DoH). It unmaps the candidate address before matching so that an IPv4 proxy/peer arriving in
// IPv4-mapped form (::ffff:a.b.c.d) — as it does on the dual-stack listeners — still matches a plain
// IPv4 trusted-proxy CIDR. Without the Unmap, netip.Prefix.Contains returns false on the v4/v6 family
// mismatch and XFF would be silently rejected (every DoH client attributed to the proxy/gateway IP).
func trustedProxySet(prefixes []netip.Prefix) netutil.SubnetSet {
	inner := netutil.SliceSubnetSet(prefixes)
	return netutil.SubnetSetFunc(func(ip netip.Addr) bool {
		return inner.Contains(ip.Unmap())
	})
}
