package maxmind

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ivpn/dns/libs/geoipdb"
	"github.com/oschwald/geoip2-golang"
)

// GeoLookupManager answers ASN lookups from a MaxMind database that is opened
// once, shared by every request, and reopened in place when the file on disk
// is refreshed.
type GeoLookupManager struct {
	db *geoipdb.Reader
}

// NewGeoLookupManager opens the ASN database and fails if the file is missing,
// unreadable or not an ASN-capable database type.
func NewGeoLookupManager(dbASNFile string) (*GeoLookupManager, error) {
	db, err := geoipdb.Open(dbASNFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open geoip ASN database %q: %w", dbASNFile, err)
	}
	return &GeoLookupManager{db: db}, nil
}

// Reload checks the file now and reopens it if it changed; a broken replacement
// is rejected and the loaded database keeps serving. Production relies on
// Watch, which runs the same check on a timer; Reload is the synchronous
// entry point for tests and for a manual "reload now" trigger.
func (g *GeoLookupManager) Reload() (bool, error) {
	return g.db.Reload()
}

// Watch reloads the database on a timer until ctx is cancelled.
func (g *GeoLookupManager) Watch(ctx context.Context, every time.Duration) {
	g.db.Watch(ctx, every)
}

// Stats reports the build time and reload counters of the loaded database.
func (g *GeoLookupManager) Stats() geoipdb.Stats {
	return g.db.Stats()
}

// Close releases the underlying database.
func (g *GeoLookupManager) Close() error {
	return g.db.Close()
}

// GetGeoLookup returns the ASN record for ip. An address that is not in the
// database yields an empty record and no error.
func (g *GeoLookupManager) GetGeoLookup(ip string) (*GeoLookup, error) {
	ipnet := net.ParseIP(ip)
	if ipnet == nil {
		return nil, fmt.Errorf("invalid IP address %q", ip)
	}

	asn, err := g.db.ASN(ipnet)
	if err != nil {
		return nil, fmt.Errorf("cannot get ASN: %w", err)
	}
	if asn == nil {
		asn = &geoip2.ASN{}
	}

	return &GeoLookup{
		IPAddress:       ipnet.String(),
		ASN:             asn.AutonomousSystemNumber,
		ASNOrganization: asn.AutonomousSystemOrganization,
	}, nil
}
