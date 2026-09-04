package dns

import (
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/dnscheck/config"
	"github.com/dnscheck/internal/maxmind"
	"github.com/miekg/dns"
)

const (
	testDomain    = "check.example.test"
	testSubdomain = "abcdefghijkl-profile1"
	testOurASN    = 64512
)

type fakeGeoLookup struct {
	result *maxmind.GeoLookup
	err    error
}

func (f *fakeGeoLookup) GetGeoLookup(ip string) (*maxmind.GeoLookup, error) {
	return f.result, f.err
}

type memCache struct {
	saved map[string][]byte
}

func (c *memCache) SaveQueryData(key string, value []byte) error {
	if c.saved == nil {
		c.saved = map[string][]byte{}
	}
	c.saved[key] = value
	return nil
}
func (c *memCache) GetQueryData(key string) ([]byte, error) { return c.saved[key], nil }
func (c *memCache) DeleteQueryData(key string) error        { delete(c.saved, key); return nil }

// tcpCaptureWriter reports a TCP remote address so both transports are covered.
type tcpCaptureWriter struct{ captureWriter }

func (w *tcpCaptureWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 40000}
}
func (w *tcpCaptureWriter) Network() string { return "tcp" }

func newTestHandler(geo GeoLookuper, cache *memCache) *Handler {
	return &Handler{srv: &DNSServer{
		Config: &config.Config{
			Server: &config.AuthoritativeDNSServerConfig{
				Domain:    testDomain,
				IPAddress: "192.0.2.1",
				ASN:       testOurASN,
				IPRange:   "198.51.100.",
			},
			Cache: &config.CacheConfig{HMACKey: "test-key"},
		},
		Cache:     cache,
		GeoLookup: geo,
	}}
}

func checkQuery() *dns.Msg {
	req := new(dns.Msg)
	req.SetQuestion(testSubdomain+"."+testDomain+".", dns.TypeA)
	return req
}

func savedRecord(t *testing.T, c *memCache) DNSLogRecord {
	t.Helper()
	if len(c.saved) != 1 {
		t.Fatalf("expected exactly one saved record, got %d", len(c.saved))
	}
	var rec DNSLogRecord
	for _, raw := range c.saved {
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("saved record is not JSON: %v", err)
		}
	}
	return rec
}

// A failing GeoIP lookup must degrade to "no ASN information", not crash or go
// silent: the A answer is still written and the record is saved with status
// derived from the IP range alone.
//
// specRef: dnscheck-behaviour.md #D6
func TestServeDNSDegradesWhenGeoLookupFails(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{err: errors.New("lookup failed")}, cache)
	w := &captureWriter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ServeDNS panicked on a failed GeoIP lookup: %v", r)
		}
	}()

	h.ServeDNS(w, checkQuery())

	if w.msg == nil || len(w.msg.Answer) != 1 {
		t.Fatalf("expected an A answer despite the lookup failure, got %+v", w.msg)
	}
	rec := savedRecord(t, cache)
	if rec.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", rec.Status, StatusUnconfigured)
	}
	if rec.ASN != 0 || rec.ASNOrganization != "" {
		t.Errorf("expected empty ASN fields, got ASN=%d org=%q", rec.ASN, rec.ASNOrganization)
	}
	if rec.IPAddress != "203.0.113.5" {
		t.Errorf("IPAddress = %q, want 203.0.113.5", rec.IPAddress)
	}
}

// specRef: dnscheck-behaviour.md #D7
func TestServeDNSMarksConfiguredWhenASNMatches(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{result: &maxmind.GeoLookup{
		IPAddress: "203.0.113.5", ASN: testOurASN, ASNOrganization: "OURS",
	}}, cache)

	req := checkQuery()
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.Option = append(opt.Option, &dns.EDNS0_LOCAL{Code: ProfileIdAdditionalSectionCode, Data: []byte("profile1")})
	req.Extra = append(req.Extra, opt)

	h.ServeDNS(&captureWriter{}, req)

	rec := savedRecord(t, cache)
	if rec.Status != StatusConfigured {
		t.Errorf("status = %q, want %q", rec.Status, StatusConfigured)
	}
	if rec.ProfileId != "profile1" {
		t.Errorf("profile_id = %q, want profile1", rec.ProfileId)
	}
	if rec.ASN != testOurASN || rec.ASNOrganization != "OURS" {
		t.Errorf("ASN fields not carried into record: %+v", rec)
	}
}

// specRef: dnscheck-behaviour.md #D8
func TestServeDNSMarksUnconfiguredWhenNeitherASNNorRangeMatch(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{result: &maxmind.GeoLookup{
		IPAddress: "203.0.113.5", ASN: 15169, ASNOrganization: "GOOGLE",
	}}, cache)

	h.ServeDNS(&captureWriter{}, checkQuery())

	rec := savedRecord(t, cache)
	if rec.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", rec.Status, StatusUnconfigured)
	}
	if rec.ASN != 15169 {
		t.Errorf("ASN = %d, want 15169", rec.ASN)
	}
}

// The remote address is taken from the transport as-is; it is never resolved.
//
// specRef: dnscheck-behaviour.md #D4
func TestServeDNSExtractsClientIPOverTCP(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{result: &maxmind.GeoLookup{}}, cache)

	h.ServeDNS(&tcpCaptureWriter{}, checkQuery())

	rec := savedRecord(t, cache)
	if rec.IPAddress != "203.0.113.5" {
		t.Errorf("IPAddress = %q, want 203.0.113.5", rec.IPAddress)
	}
}

// A queries outside the check domain get the authoritative A answer and leave no
// trace: no lookup, no cache entry.
//
// specRef: dnscheck-behaviour.md #D2
func TestServeDNSAnswersForeignDomainWithoutRecord(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{err: errors.New("must not be called")}, cache)
	w := &captureWriter{}

	req := new(dns.Msg)
	req.SetQuestion("www.other.example.", dns.TypeA)
	h.ServeDNS(w, req)

	if w.msg == nil || len(w.msg.Answer) != 1 {
		t.Fatalf("expected an A answer, got %+v", w.msg)
	}
	if len(cache.saved) != 0 {
		t.Errorf("expected no saved record, got %d", len(cache.saved))
	}
}

// specRef: dnscheck-behaviour.md #D3
func TestServeDNSIgnoresMalformedSubdomain(t *testing.T) {
	cache := &memCache{}
	h := newTestHandler(&fakeGeoLookup{err: errors.New("must not be called")}, cache)
	w := &captureWriter{}

	req := new(dns.Msg)
	req.SetQuestion("short-profile1."+testDomain+".", dns.TypeA)
	h.ServeDNS(w, req)

	if len(cache.saved) != 0 {
		t.Errorf("expected no saved record for a malformed subdomain, got %d", len(cache.saved))
	}
	if w.msg != nil {
		t.Errorf("expected no response for a malformed subdomain, got %+v", w.msg)
	}
}
