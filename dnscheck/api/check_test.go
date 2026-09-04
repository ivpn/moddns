package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnscheck/cache"
	"github.com/dnscheck/config"
	"github.com/dnscheck/dns"
)

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

func (c *memCache) GetQueryData(key string) ([]byte, error) {
	v, ok := c.saved[key]
	if !ok {
		return nil, errors.New(ErrEntryNotFound)
	}
	return v, nil
}

func (c *memCache) DeleteQueryData(key string) error { delete(c.saved, key); return nil }

const (
	testHMACKey   = "test-key"
	testSubdomain = "abcdefghijkl-profile1"
	testHost      = testSubdomain + ".check.example.test"
)

func newTestServer(c *memCache) *APIServer {
	s := NewServer(&config.Config{
		API:   &config.APIConfig{ApiAllowOrigin: "*"},
		Cache: &config.CacheConfig{HMACKey: testHMACKey},
	}, c)
	s.RegisterRoutes()
	return s
}

func get(t *testing.T, s *APIServer, host string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	resp, err := s.App.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

// specRef: dnscheck-behaviour.md #A1
func TestDnsCheckRejectsHostWithoutSubdomain(t *testing.T) {
	resp, _ := get(t, newTestServer(&memCache{}), "localhost")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// specRef: dnscheck-behaviour.md #A1
func TestDnsCheckRejectsMalformedSubdomain(t *testing.T) {
	resp, _ := get(t, newTestServer(&memCache{}), "short.check.example.test")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// specRef: dnscheck-behaviour.md #A2
func TestDnsCheckReturnsDisconnectedWhenNoRecord(t *testing.T) {
	resp, body := get(t, newTestServer(&memCache{}), testHost)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	var er ErrResponse
	if err := json.Unmarshal(body, &er); err != nil || er.Error != StatusDisconnected {
		t.Errorf("body = %s, want error=%q", body, StatusDisconnected)
	}
}

// The record is keyed by an HMAC of the subdomain, only status and profile ID
// are returned, and the entry is deleted on first read.
//
// specRef: dnscheck-behaviour.md #A3, #A4
func TestDnsCheckReturnsNarrowRecordOnceOnly(t *testing.T) {
	c := &memCache{}
	rec, _ := json.Marshal(dns.DNSLogRecord{
		Status: dns.StatusConfigured, ProfileId: "profile1",
		IPAddress: "203.0.113.5", ASN: 64512, ASNOrganization: "OURS",
	})
	if err := c.SaveQueryData(cache.HMACKey(testHMACKey, testSubdomain), rec); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(c)

	resp, body := get(t, s, testHost)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body is not JSON: %s", body)
	}
	if got["status"] != dns.StatusConfigured || got["profile_id"] != "profile1" {
		t.Errorf("body = %s", body)
	}
	for _, leaked := range []string{"ip_address", "asn", "asn_organization"} {
		if _, ok := got[leaked]; ok {
			t.Errorf("response leaks %q: %s", leaked, body)
		}
	}

	if resp, _ := get(t, s, testHost); resp.StatusCode != http.StatusNotFound {
		t.Errorf("second read status = %d, want 404 (delete-on-read)", resp.StatusCode)
	}
}
