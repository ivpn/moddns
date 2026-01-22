package asnlookup

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

type Lookup struct {
	db *geoip2.Reader
}

func New(mmdbPath string) (*Lookup, error) {
	if mmdbPath == "" {
		return nil, nil
	}
	db, err := geoip2.Open(mmdbPath)
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

func (l *Lookup) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}
