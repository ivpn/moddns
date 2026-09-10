package dns

import (
	"fmt"

	"github.com/dnscheck/cache"
	"github.com/dnscheck/config"
	"github.com/dnscheck/internal/maxmind"
	"github.com/miekg/dns"
)

// GeoLookuper resolves a client IP to its ASN record.
type GeoLookuper interface {
	GetGeoLookup(ip string) (*maxmind.GeoLookup, error)
}

// DNSServer represents a DNS server
type DNSServer struct {
	Config *config.Config

	DNSUDP *dns.Server
	DNSTCP *dns.Server

	Cache     cache.Cache
	GeoLookup GeoLookuper
}

// New creates a new DNS server
func New(config *config.Config, cache cache.Cache) (*DNSServer, error) {
	srv := &DNSServer{
		Config: config,
		Cache:  cache,
	}

	geoLookup, err := maxmind.NewGeoLookupManager(config.GeoLookupConfig.DBASNFile)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", err)
	}
	srv.GeoLookup = geoLookup

	// DNS
	srv.DNSTCP = &dns.Server{Addr: ":53", Net: "tcp"}
	srv.DNSTCP.Handler = &Handler{
		srv: srv,
	}

	// DNS
	srv.DNSUDP = &dns.Server{Addr: ":53", Net: "udp"}
	srv.DNSUDP.Handler = &Handler{
		srv: srv,
	}

	return srv, nil
}
