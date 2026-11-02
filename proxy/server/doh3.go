package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/ivpn/dns/proxy/config"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	// doh3IdleTimeout closes QUIC connections without traffic, and HTTP/3
	// connections without open requests.
	doh3IdleTimeout = 30 * time.Second

	// doh3MaxUniStreams leaves room for the control and two QPACK streams the
	// peer must open (RFC 9114 §6.2, RFC 9204 §4.2).
	doh3MaxUniStreams = 8

	// Above this rate of new connection attempts, QUIC Retry (RFC 9000 §8.1.2)
	// makes each attempt prove its source address first.
	doh3RetryRate  = 200
	doh3RetryBurst = 400

	// doh3MaxHeaderBytes caps a request's header section, read before the
	// handler and its deadline run. A DoH GET carries the query in the URL.
	doh3MaxHeaderBytes = 8 << 10
)

// Reasons for proxy_doh3_connections_rejected_total.
const (
	doh3RejectTotal      = "total"      // DOH_HTTP3_MAX_CONNECTIONS reached
	doh3RejectSource     = "source"     // DOH_HTTP3_MAX_CONNECTIONS_PER_IP reached
	doh3RejectUnverified = "unverified" // half the source's slots held by unvalidated addresses
)

// httpVersion returns the label for proxy_doh_requests_total.
func httpVersion(r *http.Request) string {
	switch {
	case r.ProtoMajor == 3:
		return "3"
	case r.ProtoMajor == 2:
		return "2"
	case r.ProtoMajor == 1 && r.ProtoMinor == 1:
		return "1.1"
	case r.ProtoMajor == 1 && r.ProtoMinor == 0:
		return "1.0"
	default:
		return "other"
	}
}

// forwardedAddrHeaders are the headers dnsproxy reads a client address from.
// HTTP/3 never arrives through a reverse proxy, so they are dropped before
// dnsproxy sees the request.
var forwardedAddrHeaders = []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP", "X-Forwarded-For"}

// doh3Server serves DoH over HTTP/3 (RFC 9114) with dnsproxy as the handler.
type doh3Server struct {
	listenAddr     netip.AddrPort
	tlsConf        *tls.Config
	handler        http.Handler
	limits         config.DoH3Limits
	requestTimeout time.Duration
	conns          *connLimiter
	metrics        doh3Metrics
	// requests holds one token per running handler (MAX_GOROUTINES); nil
	// means unbounded.
	requests chan struct{}

	mu        sync.Mutex
	conn      *net.UDPConn
	transport *quic.Transport
	listener  *quic.EarlyListener
	server    *http3.Server
}

func newDoH3Server(listenAddr netip.AddrPort, tlsConf *tls.Config, handler http.Handler, limits config.DoH3Limits, maxRequests uint, ipv6PrefixLen int, metrics doh3Metrics) *doh3Server {
	if metrics == nil {
		metrics = noopMetrics{}
	}
	d := &doh3Server{
		listenAddr:     listenAddr,
		tlsConf:        tlsConf,
		handler:        handler,
		limits:         limits,
		requestTimeout: dohTimeout,
		conns:          newConnLimiter(limits.MaxConns, limits.MaxConnsPerIP, ipv6PrefixLen, metrics),
		metrics:        metrics,
	}
	if maxRequests > 0 {
		d.requests = make(chan struct{}, maxRequests)
	}
	return d
}

func newDoH3QUICConfig(limits config.DoH3Limits) *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:        doh3IdleTimeout,
		MaxIncomingStreams:    int64(limits.MaxStreams),
		MaxIncomingUniStreams: doh3MaxUniStreams,
		Allow0RTT:             true,
	}
}

// newDoH3RetryPolicy returns a quic.Transport.VerifySourceAddress callback
// that requires address validation once a source holds half its connection
// cap, or once attempts exceed r per second.
func newDoH3RetryPolicy(conns *connLimiter, r rate.Limit, burst int) func(net.Addr) bool {
	limiter := rate.NewLimiter(r, burst)
	return func(addr net.Addr) bool { return conns.needsValidation(addr) || !limiter.Allow() }
}

// start binds the UDP socket and serves in the background.
func (d *doh3Server) start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	conn, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(d.listenAddr))
	if err != nil {
		return fmt.Errorf("doh3: listening on %s: %w", d.listenAddr, err)
	}

	d.conn = conn
	d.transport = &quic.Transport{
		Conn:                conn,
		VerifySourceAddress: newDoH3RetryPolicy(d.conns, doh3RetryRate, doh3RetryBurst),
		ConnContext:         d.conns.admit,
	}

	d.listener, err = d.transport.ListenEarly(http3.ConfigureTLSConfig(d.tlsConf.Clone()), newDoH3QUICConfig(d.limits))
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("doh3: quic listener: %w", err)
	}

	d.server = &http3.Server{
		Handler:        http.HandlerFunc(d.serveHTTP),
		IdleTimeout:    doh3IdleTimeout,
		MaxHeaderBytes: doh3MaxHeaderBytes,
	}

	srv, ln := d.server, d.listener
	go func() {
		if err := srv.ServeListener(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("doh3: serving")
		}
	}()

	log.Info().Str("addr", conn.LocalAddr().String()).Msg("Listening to DoH over HTTP/3")
	return nil
}

// StartDoH3 starts the HTTP/3 DoH listener, if one is configured.
func (s *Server) StartDoH3() error {
	if s.doh3 == nil {
		return nil
	}
	return s.doh3.start()
}

// ShutdownDoH3 stops the HTTP/3 DoH listener, if one is running.
func (s *Server) ShutdownDoH3(ctx context.Context) error {
	if s.doh3 == nil {
		return nil
	}
	return s.doh3.shutdown(ctx)
}

func (d *doh3Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	for _, h := range forwardedAddrHeaders {
		r.Header.Del(h)
	}

	// Bounds reading the body and writing the response, as dohTimeout does
	// for the HTTP/2 server.
	deadline := time.Now().Add(d.requestTimeout)
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)

	if d.requests != nil {
		wait := time.NewTimer(time.Until(deadline))
		defer wait.Stop()
		select {
		case d.requests <- struct{}{}:
			defer func() { <-d.requests }()
		case <-wait.C:
			d.metrics.RecordDoH3Busy()
			// The request deadline has passed; allow the short error reply out.
			_ = rc.SetWriteDeadline(time.Now().Add(time.Second))
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		case <-r.Context().Done():
			return
		}
	}

	d.handler.ServeHTTP(w, r)
}

func (d *doh3Server) addr() net.Addr {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.LocalAddr()
}

// shutdown lets in-flight requests finish for at most one request timeout,
// then closes the remaining connections and the socket.
func (d *doh3Server) shutdown(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.server == nil {
		return nil
	}

	graceCtx, cancel := context.WithTimeout(ctx, d.requestTimeout)
	defer cancel()

	var err error
	if d.server.Shutdown(graceCtx) != nil {
		err = d.server.Close()
	}
	// The transport does not close a socket it was given.
	return errors.Join(err, d.listener.Close(), d.transport.Close(), d.conn.Close())
}

// connLimiter caps concurrent QUIC connections in total and per source.
// IPv6 sources are grouped by prefix, as in internal/ratelimit.
type connLimiter struct {
	maxTotal      int
	maxPerIP      int
	ipv6PrefixLen int
	metrics       doh3Metrics

	mu    sync.Mutex
	total int
	perIP map[netip.Addr]int
}

func newConnLimiter(maxTotal, maxPerIP, ipv6PrefixLen int, metrics doh3Metrics) *connLimiter {
	if ipv6PrefixLen < 1 || ipv6PrefixLen > 128 {
		ipv6PrefixLen = 64
	}
	return &connLimiter{
		maxTotal:      maxTotal,
		maxPerIP:      maxPerIP,
		ipv6PrefixLen: ipv6PrefixLen,
		metrics:       metrics,
		perIP:         make(map[netip.Addr]int),
	}
}

var errDoH3ConnLimit = errors.New("doh3: connection limit reached")

func (l *connLimiter) key(addr net.Addr) (netip.Addr, bool) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}
	ip := udpAddr.AddrPort().Addr().Unmap()
	if ip.Is6() {
		if prefix, err := ip.Prefix(l.ipv6PrefixLen); err == nil {
			ip = prefix.Addr()
		}
	}
	return ip, true
}

// unverifiedCap is how many slots of one source unvalidated addresses may hold.
func (l *connLimiter) unverifiedCap() int {
	return max(1, l.maxPerIP/2)
}

// needsValidation reports whether a new attempt from addr must prove its
// address first: unvalidated attempts may hold at most half a source's slots.
func (l *connLimiter) needsValidation(addr net.Addr) bool {
	key, ok := l.key(addr)
	if !ok {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.perIP[key] >= l.unverifiedCap()
}

// admit is a quic.Transport.ConnContext callback. It runs before the handshake;
// the slot is released when the connection's context ends.
func (l *connLimiter) admit(ctx context.Context, info *quic.ClientInfo) (context.Context, error) {
	key, ok := l.key(info.RemoteAddr)
	if !ok {
		return ctx, fmt.Errorf("doh3: unexpected remote address type %T", info.RemoteAddr)
	}

	l.mu.Lock()
	reason := ""
	switch {
	case l.total >= l.maxTotal:
		reason = doh3RejectTotal
	case !info.AddrVerified && l.perIP[key] >= l.unverifiedCap():
		reason = doh3RejectUnverified
	case l.perIP[key] >= l.maxPerIP:
		reason = doh3RejectSource
	}
	if reason != "" {
		l.mu.Unlock()
		l.metrics.RecordDoH3ConnRejected(reason)
		return ctx, errDoH3ConnLimit
	}
	l.total++
	l.perIP[key]++
	l.mu.Unlock()

	context.AfterFunc(ctx, func() { l.release(key) })
	return ctx, nil
}

func (l *connLimiter) release(key netip.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.total--
	if l.perIP[key]--; l.perIP[key] <= 0 {
		delete(l.perIP, key)
	}
}
