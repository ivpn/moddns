// Package dnssec holds the proxy's DNSSEC request/response helpers: setting the
// request flags that make recursors return the Authenticated Data flag and
// Extended DNS Errors, and capturing/classifying those EDE codes so a DNSSEC
// validation failure can be surfaced on the query log.
package dnssec

import (
	"sync"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

// ReasonFailed is appended to a query log's reasons when the recursor reports a
// DNSSEC validation failure via an Extended DNS Error (RFC 8914). The frontend
// renders it as a "DNSSEC validation failed" chip.
const ReasonFailed = "dnssec_failed"

// ApplyRequestFlags configures the upstream request's DNSSEC-related bits.
//
// The logged DNSSEC-validation status (QueryLog.DNSRequest.DNSSEC, sourced from the
// response AD bit) is deliberately decoupled from the client-facing send_do_bit
// setting: validation happens at the recursor regardless of whether DNSSEC RRs are
// returned to the end device.
//   - validation enabled  -> set the request AD bit so the recursor returns, and the
//     dnsproxy library preserves (filterMsg keeps AD when the request's AD or DO bit
//     is set), the Authenticated Data flag — even when the DO bit is not sent.
//   - validation disabled -> set CD (CheckingDisabled) so the recursor skips validation.
//
// EDNS(0) is attached whenever validation is enabled — so the recursor can return
// Extended DNS Errors (carried in the OPT record) on validation failure, which
// happens whenever the query carries EDNS, independent of the DO bit — or when the
// client asked for DNSSEC RRs (sendDoBit). The DO bit, set to sendDoBit, governs
// returning RRSIG/DNSKEY records to the client.
func ApplyRequestFlags(req *dns.Msg, dnssecEnabled, sendDoBit bool) {
	req.Extra = make([]dns.RR, 0)
	if dnssecEnabled {
		req.AuthenticatedData = true
	} else {
		req.CheckingDisabled = true
	}

	if dnssecEnabled || sendDoBit {
		req.SetEdns0(2048, sendDoBit)
	}
}

// IsFailureEDE reports whether an EDE InfoCode denotes a DNSSEC *validation
// failure* (bogus zone), as opposed to merely insecure/indeterminate. RFC 8914:
//
//	6 DNSSEC Bogus, 7 Signature Expired, 8 Signature Not Yet Valid,
//	9 DNSKEY Missing, 10 RRSIGs Missing, 11 No Zone Key Bit Set, 12 NSEC Missing.
//
// Codes 1/2/5 (unsupported algorithm/digest, indeterminate) mean the zone is
// treated as insecure, not failed, so they are deliberately excluded — an
// unsigned/insecure domain must never be flagged. Verified against sdns and
// knot-resolver v6.4.0, which both emit codes in this range on SERVFAIL.
func IsFailureEDE(code uint16) bool {
	return code >= 6 && code <= 12
}

// FailureEDE returns the first DNSSEC-failure EDE InfoCode found in msg's OPT
// record, if any.
func FailureEDE(msg *dns.Msg) (uint16, bool) {
	if msg == nil {
		return 0, false
	}
	opt := msg.IsEdns0()
	if opt == nil {
		return 0, false
	}
	for _, o := range opt.Option {
		if ede, ok := o.(*dns.EDNS0_EDE); ok && IsFailureEDE(ede.InfoCode) {
			return ede.InfoCode, true
		}
	}
	return 0, false
}

// EDEStore correlates a captured DNSSEC-failure EDE code with the request that
// produced it, keyed by the request *dns.Msg pointer. dnsproxy passes the same
// dctx.Req pointer to the upstream Exchange and later exposes it to EmitQueryLog,
// so the pointer is a stable per-request key. Entries are set by CapturingUpstream
// at exchange time and drained by EmitQueryLog. Only DNSSEC-failure responses store
// an entry, so the map stays tiny and short-lived.
type EDEStore struct{ m sync.Map }

// Set records the EDE code for req.
func (s *EDEStore) Set(req *dns.Msg, code uint16) {
	if s == nil {
		return
	}
	s.m.Store(req, code)
}

// Take returns and removes the stored EDE code for req. Nil-safe so a caller
// constructed without an EDEStore (e.g. in unit tests) is a harmless no-op.
func (s *EDEStore) Take(req *dns.Msg) (uint16, bool) {
	if s == nil {
		return 0, false
	}
	v, ok := s.m.LoadAndDelete(req)
	if !ok {
		return 0, false
	}
	return v.(uint16), true
}

// CapturingUpstream wraps an upstream to capture DNSSEC-failure EDE codes from
// responses BEFORE dnsproxy's filterMsg strips the OPT record (which happens
// before the query log is emitted, so the EDE is otherwise unavailable at log
// time). Address()/Close() come from the embedded upstream; only Exchange is
// intercepted.
type CapturingUpstream struct {
	upstream.Upstream
	store *EDEStore
}

// NewCapturingUpstream wraps u so DNSSEC-failure EDE codes are captured into store.
func NewCapturingUpstream(u upstream.Upstream, store *EDEStore) *CapturingUpstream {
	return &CapturingUpstream{Upstream: u, store: store}
}

func (u *CapturingUpstream) Exchange(req *dns.Msg) (*dns.Msg, error) {
	resp, err := u.Upstream.Exchange(req)
	if err == nil {
		if code, ok := FailureEDE(resp); ok {
			u.store.Set(req, code)
		}
	}
	return resp, err
}
