package dnssec

import (
	"errors"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

// mockUpstream implements upstream.Upstream for wrapper tests.
type mockUpstream struct {
	resp   *dns.Msg
	err    error
	gotReq *dns.Msg
}

func (m *mockUpstream) Exchange(req *dns.Msg) (*dns.Msg, error) {
	m.gotReq = req
	return m.resp, m.err
}
func (m *mockUpstream) Address() string { return "mock" }
func (m *mockUpstream) Close() error    { return nil }

// msgWithEDE builds a response carrying an OPT record with the given EDE InfoCode.
func msgWithEDE(rcode int, code uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("dnssec-failed.org"), dns.TypeA)
	m.Rcode = rcode
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.Option = append(opt.Option, &dns.EDNS0_EDE{InfoCode: code})
	m.Extra = append(m.Extra, opt)
	return m
}

func newReq() *dns.Msg {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
	// seed Extra to confirm it is reset
	req.Extra = []dns.RR{&dns.TXT{Hdr: dns.RR_Header{Name: "x.", Rrtype: dns.TypeTXT}, Txt: []string{"seed"}}}
	return req
}

// ApplyRequestFlags decouples logged validation status from the client-facing
// send_do_bit and always attaches EDNS when validation is enabled so the recursor
// can return EDE.
func TestApplyRequestFlags(t *testing.T) {
	t.Run("enabled, send_do_bit off: AD set, no CD, EDNS present but DO=0", func(t *testing.T) {
		req := newReq()
		ApplyRequestFlags(req, true, false)
		assert.True(t, req.AuthenticatedData, "AD bit must be set so validation is logged")
		assert.False(t, req.CheckingDisabled)
		if o := req.IsEdns0(); assert.NotNil(t, o, "EDNS(0) must be present so EDE can be returned") {
			assert.False(t, o.Do(), "DO must be off when send_do_bit is off")
		}
	})

	t.Run("enabled, send_do_bit on: AD set and DO set", func(t *testing.T) {
		req := newReq()
		ApplyRequestFlags(req, true, true)
		assert.True(t, req.AuthenticatedData)
		assert.False(t, req.CheckingDisabled)
		if o := req.IsEdns0(); assert.NotNil(t, o) {
			assert.True(t, o.Do())
		}
	})

	t.Run("disabled: CD set, AD not set, no EDNS", func(t *testing.T) {
		req := newReq()
		ApplyRequestFlags(req, false, false)
		assert.True(t, req.CheckingDisabled, "CD must be set so the recursor skips validation")
		assert.False(t, req.AuthenticatedData)
		assert.Nil(t, req.IsEdns0(), "no EDNS when validation is disabled")
	})

	t.Run("disabled, send_do_bit on: CD set, DO set, AD not set", func(t *testing.T) {
		req := newReq()
		ApplyRequestFlags(req, false, true)
		assert.True(t, req.CheckingDisabled)
		assert.False(t, req.AuthenticatedData)
		if o := req.IsEdns0(); assert.NotNil(t, o) {
			assert.True(t, o.Do())
		}
	})

	t.Run("Extra is reset (seed cleared)", func(t *testing.T) {
		req := newReq()
		ApplyRequestFlags(req, true, false)
		for _, rr := range req.Extra {
			_, isTXT := rr.(*dns.TXT)
			assert.False(t, isTXT, "seeded/stale RRs must be cleared")
		}
	})
}

func TestIsFailureEDE(t *testing.T) {
	// DNSSEC validation-failure codes 6..12 are failures.
	for _, c := range []uint16{6, 7, 8, 9, 10, 11, 12} {
		assert.True(t, IsFailureEDE(c), "code %d should be a DNSSEC failure", c)
	}
	// Insecure/indeterminate/other codes must NOT be treated as failures
	// (so unsigned domains are never flagged).
	for _, c := range []uint16{0, 1, 2, 3, 4, 5, 13, 29} {
		assert.False(t, IsFailureEDE(c), "code %d should NOT be a DNSSEC failure", c)
	}
}

// tableRef: logs-reason-display-behaviour #13
func TestFailureEDE(t *testing.T) {
	t.Run("SERVFAIL with EDE 9 -> detected", func(t *testing.T) {
		code, ok := FailureEDE(msgWithEDE(dns.RcodeServerFailure, 9))
		assert.True(t, ok)
		assert.Equal(t, uint16(9), code)
	})
	t.Run("EDE 5 (indeterminate) -> not a failure", func(t *testing.T) {
		_, ok := FailureEDE(msgWithEDE(dns.RcodeServerFailure, 5))
		assert.False(t, ok)
	})
	t.Run("no OPT/EDE -> not a failure", func(t *testing.T) {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
		_, ok := FailureEDE(m)
		assert.False(t, ok)
		_, ok = FailureEDE(nil)
		assert.False(t, ok)
	})
}

func TestEDEStore(t *testing.T) {
	s := &EDEStore{}
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("x.org"), dns.TypeA)

	_, ok := s.Take(req)
	assert.False(t, ok, "empty store returns nothing")

	s.Set(req, 9)
	code, ok := s.Take(req)
	assert.True(t, ok)
	assert.Equal(t, uint16(9), code)

	_, ok = s.Take(req)
	assert.False(t, ok, "Take must remove the entry")

	// nil-safe
	var ns *EDEStore
	ns.Set(req, 9)
	_, ok = ns.Take(req)
	assert.False(t, ok)
}

func TestCapturingUpstream(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("dnssec-failed.org"), dns.TypeA)

	t.Run("captures DNSSEC-failure EDE keyed by request pointer", func(t *testing.T) {
		store := &EDEStore{}
		u := NewCapturingUpstream(&mockUpstream{resp: msgWithEDE(dns.RcodeServerFailure, 9)}, store)
		_, err := u.Exchange(req)
		assert.NoError(t, err)
		code, ok := store.Take(req)
		assert.True(t, ok, "EDE must be captured for the exact request")
		assert.Equal(t, uint16(9), code)
	})

	t.Run("no capture for a clean response", func(t *testing.T) {
		store := &EDEStore{}
		clean := new(dns.Msg)
		clean.SetQuestion(dns.Fqdn("cloudflare.com"), dns.TypeA)
		clean.Rcode = dns.RcodeSuccess
		u := NewCapturingUpstream(&mockUpstream{resp: clean}, store)
		_, _ = u.Exchange(req)
		_, ok := store.Take(req)
		assert.False(t, ok)
	})

	t.Run("no capture on exchange error", func(t *testing.T) {
		store := &EDEStore{}
		u := NewCapturingUpstream(&mockUpstream{err: errors.New("timeout")}, store)
		_, err := u.Exchange(req)
		assert.Error(t, err)
		_, ok := store.Take(req)
		assert.False(t, ok)
	})
}
