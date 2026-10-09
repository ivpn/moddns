// Package geoip enriches client IPs with ASN and country from MaxMind
// databases. Every database is optional: a missing or unreadable file leaves
// the corresponding fields nil and never fails a lookup.
package geoip

import (
	"context"
	"net/netip"
	"time"

	"github.com/ivpn/dns/libs/geoipdb"
	"github.com/rs/zerolog/log"
)

// Config names the database files; an empty path disables that database.
type Config struct {
	ASNFile     string
	CountryFile string
	ReloadEvery time.Duration
}

// Info is the enrichment for one IP; a nil field means unknown.
type Info struct {
	ASN     *uint32
	ASOrg   *string
	Country *string
}

// Enricher is safe for concurrent use and for a nil receiver.
type Enricher struct {
	asn         *geoipdb.Reader
	country     *geoipdb.Reader
	reloadEvery time.Duration
}

// New opens the configured databases. A database that is not configured or
// fails to open is logged once as a warning and skipped.
func New(cfg Config) *Enricher {
	e := &Enricher{reloadEvery: cfg.ReloadEvery}
	e.asn = openOptional("GEOIP_DB_ASN_FILE", cfg.ASNFile, geoipdb.Open)
	e.country = openOptional("GEOIP_DB_COUNTRY_FILE", cfg.CountryFile, geoipdb.OpenCountry)
	return e
}

func openOptional(envVar, path string, open func(string) (*geoipdb.Reader, error)) *geoipdb.Reader {
	if path == "" {
		log.Warn().Str("env", envVar).Msg("GeoIP database not configured; client enrichment fields will be null")
		return nil
	}
	r, err := open(path)
	if err != nil {
		log.Warn().Err(err).Str("env", envVar).Str("path", path).Msg("GeoIP database unavailable; client enrichment fields will be null")
		return nil
	}
	return r
}

// Start reloads the opened databases when their files are replaced, until ctx
// is cancelled.
func (e *Enricher) Start(ctx context.Context) {
	if e == nil {
		return
	}
	if e.asn != nil {
		go e.asn.Watch(ctx, e.reloadEvery)
	}
	if e.country != nil {
		go e.country.Watch(ctx, e.reloadEvery)
	}
}

// Lookup returns what the databases know about ip. An unparsable address or a
// failed lookup yields the zero Info.
func (e *Enricher) Lookup(ip string) Info {
	var info Info
	if e == nil {
		return info
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return info
	}
	if e.asn != nil {
		if rec, err := e.asn.ASN(addr); err == nil && rec != nil && rec.AutonomousSystemNumber != 0 {
			asn := uint32(rec.AutonomousSystemNumber) //nolint:gosec // ASNs are 32-bit
			info.ASN = &asn
			if org := rec.AutonomousSystemOrganization; org != "" {
				info.ASOrg = &org
			}
		}
	}
	if e.country != nil {
		if rec, err := e.country.Country(addr); err == nil && rec != nil && rec.Country.ISOCode != "" {
			cc := rec.Country.ISOCode
			info.Country = &cc
		}
	}
	return info
}
