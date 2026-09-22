package server

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

// Test buildDNSCheckResponse ensures Tier 3 response construction behaves as expected.
func TestBuildDNSCheckResponse(t *testing.T) {
	type tc struct {
		name          string
		qtype         uint16
		setupUpstream func(req *dns.Msg) *dns.Msg
		assert        func(t *testing.T, req, upstream, resp *dns.Msg)
	}

	makeA := func(host string) dns.RR {
		rr, _ := dns.NewRR(host + " 60 IN A 203.0.113.10")
		return rr
	}
	makeAAAA := func(host string) dns.RR {
		rr, _ := dns.NewRR(host + " 60 IN AAAA 2001:db8::10")
		return rr
	}
	makeNS := func(host, ns string) dns.RR {
		rr, _ := dns.NewRR(host + " 300 IN NS " + ns)
		return rr
	}

	cases := []tc{
		{
			name:  "A query copies answer and strips OPT",
			qtype: dns.TypeA,
			setupUpstream: func(req *dns.Msg) *dns.Msg {
				up := new(dns.Msg)
				up.SetReply(req)
				up.Rcode = dns.RcodeSuccess
				up.Answer = []dns.RR{makeA(req.Question[0].Name)}
				up.Ns = []dns.RR{makeNS(req.Question[0].Name, "ns1.example.net.")}
				// Add an OPT we expect to be removed
				opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
				opt.SetUDPSize(4096)
				up.Extra = []dns.RR{opt}
				return up
			},
			assert: func(t *testing.T, req, upstream, resp *dns.Msg) {
				require.True(t, resp.MsgHdr.Response, "QR flag must be set")
				require.Equal(t, req.Id, resp.Id, "ID must match request")
				require.Equal(t, upstream.Rcode, resp.Rcode)
				require.Len(t, resp.Answer, len(upstream.Answer))
				// OPT must be stripped
				for _, rr := range resp.Extra {
					_, isOpt := rr.(*dns.OPT)
					require.False(t, isOpt, "OPT record should have been stripped")
				}
			},
		},
		{
			name:  "AAAA query copies answer",
			qtype: dns.TypeAAAA,
			setupUpstream: func(req *dns.Msg) *dns.Msg {
				up := new(dns.Msg)
				up.SetReply(req)
				up.Rcode = dns.RcodeSuccess
				up.Answer = []dns.RR{makeAAAA(req.Question[0].Name)}
				return up
			},
			assert: func(t *testing.T, req, upstream, resp *dns.Msg) {
				require.True(t, resp.MsgHdr.Response)
				require.Equal(t, req.Id, resp.Id)
				require.Len(t, resp.Answer, 1)
			},
		},
		{
			name:  "Upstream Rcode propagated",
			qtype: dns.TypeA,
			setupUpstream: func(req *dns.Msg) *dns.Msg {
				up := new(dns.Msg)
				up.SetReply(req)
				up.Rcode = dns.RcodeNameError
				return up
			},
			assert: func(t *testing.T, req, upstream, resp *dns.Msg) {
				require.Equal(t, dns.RcodeNameError, resp.Rcode)
			},
		},
	}

	for _, c := range cases {
		c := c
		it := t
		it.Run(c.name, func(t *testing.T) {
			// Create request with EDNS OPT to ensure it isn't copied.
			req := new(dns.Msg)
			req.SetQuestion("subprofile.test.staging.ivpndns.net.", c.qtype)
			opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
			opt.SetUDPSize(1232)
			req.Extra = append(req.Extra, opt)

			upstream := c.setupUpstream(req)
			server := &Server{}
			resp := server.buildDNSCheckResponse(req, upstream)
			c.assert(t, req, upstream, resp)
		})
	}
}
