package asnlookup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ivpn/dns/libs/geoipdb"
)

// Lookup answers ASN queries from a GeoLite2-ASN file and follows the file
// when it is refreshed on disk.
type Lookup struct {
	db *geoipdb.Reader
}

func New(mmdbPath string) (*Lookup, error) {
	if mmdbPath == "" {
		return nil, errors.New("ASN MMDB path is required")
	}
	db, err := geoipdb.Open(mmdbPath)
	if err != nil {
		return nil, err
	}
	return &Lookup{db: db}, nil
}

func (l *Lookup) ASN(ip net.IP) (uint, error) {
	if l == nil || l.db == nil {
		return 0, nil
	}
	if ip == nil {
		return 0, nil
	}
	rec, err := l.db.ASN(ip)
	if err != nil {
		return 0, fmt.Errorf("asn lookup: %w", err)
	}
	if rec == nil {
		return 0, nil
	}
	return rec.AutonomousSystemNumber, nil
}

// Reload checks the file now and reopens it if it changed; see
// geoipdb.Reader.Reload. Production relies on Watch, which runs the same check
// on a timer; Reload is the synchronous entry point for tests and for a manual
// "reload now" trigger.
func (l *Lookup) Reload() (bool, error) {
	if l == nil || l.db == nil {
		return false, nil
	}
	return l.db.Reload()
}

// Watch reloads the database on a timer until ctx is cancelled.
func (l *Lookup) Watch(ctx context.Context, every time.Duration) {
	if l == nil || l.db == nil {
		return
	}
	l.db.Watch(ctx, every)
}

// Stats reports the build time and reload counters of the loaded database.
func (l *Lookup) Stats() geoipdb.Stats {
	if l == nil || l.db == nil {
		return geoipdb.Stats{}
	}
	return l.db.Stats()
}

func (l *Lookup) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}
