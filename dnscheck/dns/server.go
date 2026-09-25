package dns

import (
	"context"
	"fmt"

	"github.com/dnscheck/cache"
	"github.com/dnscheck/config"
	"github.com/dnscheck/internal/maxmind"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
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
	// The file is refreshed on disk by geoipupdate; follow it without a restart.
	go geoLookup.Watch(context.Background(), config.GeoLookupConfig.ReloadEvery)
	log.Info().
		Str("path", config.GeoLookupConfig.DBASNFile).
		Time("build_time", geoLookup.Stats().BuildTime).
		Dur("reload_every", config.GeoLookupConfig.ReloadEvery).
		Msg("GeoIP ASN database loaded")

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
