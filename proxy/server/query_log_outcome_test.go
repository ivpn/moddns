package server

// Tests for classifyOutcome — the query-log resolution-outcome taxonomy.
// Each case references a row of docs/specs/query-log-outcomes-behaviour.md.

import (
	"context"
	"errors"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

// timeoutErr implements net.Error with Timeout() == true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func outcomeDctx(rcode int, withAnswer bool) *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Rcode = rcode
	if withAnswer {
		res.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 30},
			A:   []byte{1, 2, 3, 4},
		}}
	}
	return &proxy.DNSContext{Req: req, Res: res}
}

func TestClassifyOutcome(t *testing.T) {
	tests := []struct {
		name         string
		specRef      string // row of query-log-outcomes-behaviour.md
		status       model.Status
		upstreamErr  error
		dctx         *proxy.DNSContext
		dnssecFailed bool
		want         string
	}{
		{"resolved answer", "O1", model.StatusProcessed, nil, outcomeDctx(dns.RcodeSuccess, true), false, "resolved"},
		{"empty NOERROR answer", "O2", model.StatusProcessed, nil, outcomeDctx(dns.RcodeSuccess, false), false, "nodata"},
		{"nxdomain", "O3", model.StatusProcessed, nil, outcomeDctx(dns.RcodeNameError, false), false, "nxdomain"},
		{"blocked wins over everything", "O4 OE1", model.StatusBlocked, errors.New("x"), outcomeDctx(dns.RcodeSuccess, true), false, "blocked"},
		{"dnssec servfail", "O5", model.StatusProcessed, nil, outcomeDctx(dns.RcodeServerFailure, false), true, "servfail_dnssec"},
		{"upstream servfail", "O6", model.StatusProcessed, nil, outcomeDctx(dns.RcodeServerFailure, false), false, "servfail_upstream"},
		{"deadline exceeded", "O7", model.StatusProcessed, context.DeadlineExceeded, outcomeDctx(dns.RcodeServerFailure, false), false, "timeout"},
		{"net.Error timeout", "O7", model.StatusProcessed, timeoutErr{}, outcomeDctx(dns.RcodeServerFailure, false), false, "timeout"},
		{"wrapped timeout", "O7 OE2", model.StatusProcessed, errors.Join(errors.New("resolving"), context.DeadlineExceeded), outcomeDctx(dns.RcodeServerFailure, false), false, "timeout"},
		{"non-timeout upstream error", "O8", model.StatusProcessed, errors.New("connection refused"), outcomeDctx(dns.RcodeServerFailure, false), false, "network_error"},
		{"upstream error with nil Res", "O8 OE3", model.StatusProcessed, errors.New("connection refused"), &proxy.DNSContext{Req: new(dns.Msg)}, false, "network_error"},
		{"refused", "O9", model.StatusProcessed, nil, outcomeDctx(dns.RcodeRefused, false), false, "refused"},
		{"nil Res without error is unknown", "O10", model.StatusProcessed, nil, &proxy.DNSContext{Req: new(dns.Msg)}, false, ""},
		{"unmapped rcode falls back to legacy", "OE4", model.StatusProcessed, nil, outcomeDctx(dns.RcodeFormatError, false), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCtx := &requestcontext.RequestContext{
				FilterResult: model.FilterResult{Status: tt.status},
				UpstreamErr:  tt.upstreamErr,
			}
			got := classifyOutcome(reqCtx, tt.dctx, tt.dnssecFailed)
			assert.Equal(t, tt.want, got, "row %s: %s", tt.specRef, tt.name)
		})
	}
}

// BenchmarkClassifyOutcome measures the per-log-entry classification cost.
// It runs on the EmitQueryLog goroutine (off the DNS response path) and only
// for logging-enabled profiles, so this bounds the logging-path CPU overhead.
func BenchmarkClassifyOutcome(b *testing.B) {
	cases := []struct {
		name   string
		reqCtx *requestcontext.RequestContext
		dctx   *proxy.DNSContext
	}{
		{"resolved", &requestcontext.RequestContext{FilterResult: model.FilterResult{Status: model.StatusProcessed}}, outcomeDctx(dns.RcodeSuccess, true)},
		{"blocked", &requestcontext.RequestContext{FilterResult: model.FilterResult{Status: model.StatusBlocked}}, outcomeDctx(dns.RcodeSuccess, true)},
		{"timeout", &requestcontext.RequestContext{FilterResult: model.FilterResult{Status: model.StatusProcessed}, UpstreamErr: context.DeadlineExceeded}, outcomeDctx(dns.RcodeServerFailure, false)},
	}
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				classifyOutcome(bc.reqCtx, bc.dctx, false)
			}
		})
	}
}
