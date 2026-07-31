package dns

import (
	"net"
	"testing"

	"github.com/dnscheck/config"
	"github.com/miekg/dns"
)

// captureWriter is a minimal dns.ResponseWriter that records the reply instead
// of putting it on a socket.
type captureWriter struct {
	msg *dns.Msg
}

func (w *captureWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53}
}
func (w *captureWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 40000}
}
func (w *captureWriter) WriteMsg(m *dns.Msg) error { w.msg = m; return nil }
func (w *captureWriter) Write([]byte) (int, error) { return 0, nil }
func (w *captureWriter) Close() error              { return nil }
func (w *captureWriter) TsigStatus() error         { return nil }
func (w *captureWriter) TsigTimersOnly(bool)       {}
func (w *captureWriter) Hijack()                   {}
func (w *captureWriter) Network() string           { return "udp" }

// A DNS message with no question section must not crash the server.
//
// QDCOUNT (RFC 1035 4.1.1) is a header field and is not validated against the
// body, so a header-only message declaring QDCOUNT=1 unpacks with no error and a
// nil Question slice. Indexing Question[0] then panics, and the handler runs in a
// goroutine per packet, so the panic would terminate the process -- taking the DNS
// listener and the HTTP API sharing it down together.
func TestServeDNSHandlesMissingQuestionSection(t *testing.T) {
	// 12-byte header, QDCOUNT=1, everything else zero. Passes the library's
	// length and accept checks.
	wire := []byte{
		0x00, 0x00, // ID
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

	h := &Handler{srv: &DNSServer{Config: &config.Config{}}}
	w := &captureWriter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ServeDNS panicked on a question-less message: %v", r)
		}
	}()

	h.ServeDNS(w, req)

	// Beyond not crashing, the server should answer rather than go silent, so
	// the client learns the request was malformed.
	if w.msg == nil {
		t.Fatal("no reply written for a malformed request; expected FORMERR")
	}
	if w.msg.Rcode != dns.RcodeFormatError {
		t.Errorf("expected FORMERR for a question-less message, got %s",
			dns.RcodeToString[w.msg.Rcode])
	}
}

// QDCOUNT=0 with no question: same code path, pinned for completeness.
func TestServeDNSHandlesZeroQuestionCount(t *testing.T) {
	req := new(dns.Msg)
	req.Id = 1234
	req.Question = nil // no question at all

	h := &Handler{srv: &DNSServer{Config: &config.Config{}}}
	w := &captureWriter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ServeDNS panicked on a zero-question message: %v", r)
		}
	}()

	h.ServeDNS(w, req)

	if w.msg == nil {
		t.Fatal("no reply written for a malformed request; expected FORMERR")
	}
}
