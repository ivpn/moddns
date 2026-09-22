package maxmind

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// GeoLookupManager answers ASN lookups from a MaxMind database that is opened
// once and shared by every request; geoip2.Reader is safe for concurrent use.
type GeoLookupManager struct {
	asnDB *geoip2.Reader
}

// NewGeoLookupManager opens the ASN database and fails if the file is missing,
// unreadable or not an ASN-capable database type.
func NewGeoLookupManager(dbASNFile string) (*GeoLookupManager, error) {
	asnDB, err := geoip2.Open(dbASNFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open geoip ASN database %q: %w", dbASNFile, err)
	}

	// geoip2 only reports a database/method mismatch at lookup time, so probe
	// once here rather than on every request.
	if _, err := asnDB.ASN(net.IPv4(192, 0, 2, 1)); err != nil {
		asnDB.Close()
		return nil, fmt.Errorf("geoip database %q does not support ASN lookups: %w", dbASNFile, err)
	}

	return &GeoLookupManager{asnDB: asnDB}, nil
}

// Close releases the underlying database.
func (g *GeoLookupManager) Close() error {
	return g.asnDB.Close()
}

// GetGeoLookup returns the ASN record for ip. An address that is not in the
// database yields an empty record and no error.
func (g *GeoLookupManager) GetGeoLookup(ip string) (*GeoLookup, error) {
	ipnet := net.ParseIP(ip)
	if ipnet == nil {
		return nil, fmt.Errorf("invalid IP address %q", ip)
	}

	asn, err := g.asnDB.ASN(ipnet)
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
