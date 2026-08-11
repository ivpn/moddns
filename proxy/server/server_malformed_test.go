package server

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMalformedTestServer builds a minimal Server with rate limiting disabled so
// HandleBefore reaches the question-section validation.
func newMalformedTestServer() *Server {
	return &Server{
		Config: &config.Config{
			Server:    &config.ServerConfig{},
			RateLimit: &config.RateLimitConfig{},
		},
		RateLimiter:   ratelimit.New(ratelimit.Config{}, nil),
		LoggerFactory: logging.NewDefaultFactory(),
		Metrics:       noopMetrics{},
	}
}

func requireFormErr(t *testing.T, req *dns.Msg, err error) {
	t.Helper()
	require.Error(t, err)

	var befErr *proxy.BeforeRequestError
	require.True(t, errors.As(err, &befErr), "expected BeforeRequestError, got %v", err)
	require.NotNil(t, befErr.Response)
	assert.Equal(t, dns.RcodeFormatError, befErr.Response.Rcode)
	assert.True(t, befErr.Response.Response, "QR flag must be set")
	assert.Equal(t, req.Id, befErr.Response.Id, "response ID must match request")
}

// A DNS message with no question section must be answered with FORMERR before
// any profile extraction or logging touches Question[0].
//
// QDCOUNT (RFC 1035 §4.1.1) is a header field and is not validated against the
// body, so a header-only message declaring QDCOUNT=1 unpacks with no error and
// a nil Question slice. The vendor's own question-count check (validateRequest)
// runs only after HandleBefore, so HandleBefore sees the raw message.
func TestHandleBefore_EmptyQuestionSection(t *testing.T) {
	// 12-byte header, QDCOUNT=1, everything else zero. Passes the library's
	// length and accept checks.
	wire := []byte{
		0x00, 0x2a, // ID
		0x00, 0x00, // flags: QR=0, Opcode=QUERY
		0x00, 0x01, // QDCOUNT = 1 (lies — there is no question)
		0x00, 0x00, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT
	}

	req := new(dns.Msg)
	if err := req.Unpack(wire); err != nil {
		t.Fatalf("precondition failed: library rejected the packet (%v); "+
			"this test is meaningless if it never reaches the handler", err)
	}
	if len(req.Question) != 0 {
		t.Fatalf("precondition failed: expected an empty question section, got %#v", req.Question)
	}

	s := newMalformedTestServer()
	dctx := &proxy.DNSContext{
		Req:   req,
		Addr:  netip.MustParseAddrPort("192.0.2.1:53"),
		Proto: proxy.ProtoTCP,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HandleBefore panicked on a question-less message: %v", r)
		}
	}()

	err := s.HandleBefore(nil, dctx)
	requireFormErr(t, req, err)
}

// QDCOUNT > 1 in a QUERY-opcode message must get FORMERR (RFC 9619).
func TestHandleBefore_MultipleQuestions(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)
	req.Question = append(req.Question, dns.Question{
		Name: dns.Fqdn("example.org"), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET,
	})

	s := newMalformedTestServer()
	dctx := &proxy.DNSContext{
		Req:   req,
		Addr:  netip.MustParseAddrPort("192.0.2.1:53"),
		Proto: proxy.ProtoUDP,
	}

	err := s.HandleBefore(nil, dctx)
	requireFormErr(t, req, err)
}
