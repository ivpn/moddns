package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/dnssec"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// specRef: proxy-request-admission-behaviour.md #Q10 #Q16
func TestNewProxyConfig_DoHHTTP3ServedOutsideDNSProxy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			s := &Server{
				Upstreams: map[string]*proxy.CustomUpstreamConfig{},
				edeStore:  &dnssec.EDEStore{},
			}

			conf, err := s.newProxyConfig(newTestServerConfig(t, &config.DoHConfig{ListenAddr: 443, HTTP3Enabled: enabled}))
			require.NoError(t, err)
			require.NotNil(t, conf.HTTPConfig)
			assert.False(t, conf.HTTPConfig.HTTP3Enabled, "HTTP/3 is served by doh3Server, not by dnsproxy")
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestNewDoH3QUICConfig(t *testing.T) {
	conf := newDoH3QUICConfig(config.DoH3Limits{MaxStreams: 7})

	assert.EqualValues(t, 7, conf.MaxIncomingStreams)
	assert.EqualValues(t, 8, conf.MaxIncomingUniStreams)
	assert.Equal(t, 30*time.Second, conf.MaxIdleTimeout)
	assert.True(t, conf.Allow0RTT)
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestDoH3RetryPolicy(t *testing.T) {
	conns, _ := newTestConnLimiter(8, 8)
	needsRetry := newDoH3RetryPolicy(conns, 0, 2)
	addr := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 5000}

	assert.False(t, needsRetry(addr))
	assert.False(t, needsRetry(addr))
	assert.True(t, needsRetry(addr), "attempts above the burst must prove their address")

	// A source holding half its slots must prove its address even when the
	// global rate allows the attempt.
	conns, _ = newTestConnLimiter(8, 2)
	needsRetry = newDoH3RetryPolicy(conns, rate.Inf, 1)
	assert.False(t, needsRetry(addr))
	_, err := admitTest(t, conns, addr.String(), true)
	require.NoError(t, err)
	assert.True(t, needsRetry(addr))
}

func newTestConnLimiter(maxTotal, maxPerIP int) (*connLimiter, *recordingMetrics) {
	m := &recordingMetrics{}
	return newConnLimiter(maxTotal, maxPerIP, 64, m), m
}

func admitTest(t *testing.T, l *connLimiter, addr string, verified bool) (context.CancelFunc, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	_, err := l.admit(ctx, &quic.ClientInfo{
		RemoteAddr:   net.UDPAddrFromAddrPort(netip.MustParseAddrPort(addr)),
		AddrVerified: verified,
	})
	if err != nil {
		cancel()
	}
	return cancel, err
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestConnLimiter_UnverifiedAddressesUseHalfThePerIPCap(t *testing.T) {
	l, m := newTestConnLimiter(100, 4)
	src := "192.0.2.1:5000"

	for i := range 2 {
		_, err := admitTest(t, l, src, false)
		require.NoError(t, err, "unverified %d", i)
	}
	assert.True(t, l.needsValidation(net.UDPAddrFromAddrPort(netip.MustParseAddrPort(src))))
	_, err := admitTest(t, l, src, false)
	require.ErrorIs(t, err, errDoH3ConnLimit, "unverified attempts stop at half the cap")

	for i := range 2 {
		_, err := admitTest(t, l, src, true)
		require.NoError(t, err, "verified %d", i)
	}
	_, err = admitTest(t, l, src, true)
	require.ErrorIs(t, err, errDoH3ConnLimit)
	assert.Equal(t, []string{doh3RejectUnverified, doh3RejectSource}, m.snapshot().doh3Rejected)
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestConnLimiter_GroupsIPv6ByPrefixAndUnmapsIPv4(t *testing.T) {
	l, _ := newTestConnLimiter(100, 2)

	_, err := admitTest(t, l, "[2001:db8:1:2::1]:5000", true)
	require.NoError(t, err)
	_, err = admitTest(t, l, "[2001:db8:1:2::ffff]:5000", true)
	require.NoError(t, err)
	_, err = admitTest(t, l, "[2001:db8:1:2:aaaa::1]:5000", true)
	require.ErrorIs(t, err, errDoH3ConnLimit, "same /64 shares one per-IP cap")
	_, err = admitTest(t, l, "[2001:db8:1:3::1]:5000", true)
	require.NoError(t, err, "another /64 has its own cap")

	_, err = admitTest(t, l, "[::ffff:192.0.2.7]:5000", true)
	require.NoError(t, err)
	_, err = admitTest(t, l, "192.0.2.7:5001", true)
	require.NoError(t, err)
	_, err = admitTest(t, l, "192.0.2.7:5002", true)
	require.ErrorIs(t, err, errDoH3ConnLimit, "4-mapped-6 and native IPv4 share one cap")
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestConnLimiter_ReleasesSlotWhenConnectionEnds(t *testing.T) {
	l, m := newTestConnLimiter(1, 1)
	cancel, err := admitTest(t, l, "192.0.2.1:5000", true)
	require.NoError(t, err)
	_, err = admitTest(t, l, "192.0.2.2:5000", true)
	require.ErrorIs(t, err, errDoH3ConnLimit)
	assert.Equal(t, []string{doh3RejectTotal}, m.snapshot().doh3Rejected)

	cancel()
	require.Eventually(t, func() bool {
		c, err := admitTest(t, l, "192.0.2.2:5000", true)
		if err == nil {
			c()
		}
		return err == nil
	}, time.Second, 10*time.Millisecond)
}

// seenRequest is what the stub handler observed for one DoH request.
type seenRequest struct {
	profile         string
	device          string
	protoMajor      int
	addr            netip.Addr
	isPrivateClient bool
}

// dohTestEnv runs dnsproxy's DoH listener (HTTP/2) and doh3Server (HTTP/3) on
// loopback ports in front of a stub handler.
type dohTestEnv struct {
	h2Port  int
	h3      *doh3Server
	seen    func() seenRequest
	metrics *recordingMetrics
}

// dohTestOpts configures startDoHTestEnv; zero values get test defaults.
type dohTestOpts struct {
	limits         config.DoH3Limits
	trustedProxies []string
	maxRequests    uint
	// block, if set, holds every handler call until it is closed.
	block <-chan struct{}
}

func startDoHTestEnv(t *testing.T, opts dohTestOpts) *dohTestEnv {
	t.Helper()

	limits := opts.limits
	if limits == (config.DoH3Limits{}) {
		limits = config.DoH3Limits{MaxConns: 8, MaxConnsPerIP: 8, MaxStreams: 8}
	}

	s := &Server{
		Upstreams: map[string]*proxy.CustomUpstreamConfig{},
		edeStore:  &dnssec.EDEStore{},
		Config:    &config.Config{Server: &config.ServerConfig{}},
	}
	serverConfig := newTestServerConfig(t, &config.DoHConfig{ListenAddr: 443, HTTP3Enabled: true, HTTP3: limits})
	if len(opts.trustedProxies) > 0 {
		serverConfig.TrustedProxies = opts.trustedProxies
	}
	conf, err := s.newProxyConfig(serverConfig)
	require.NoError(t, err)
	conf.HTTPConfig.ListenAddresses = []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0")}

	var (
		mu   sync.Mutex
		last seenRequest
	)
	conf.RequestHandler = proxy.HandlerFunc(func(_ context.Context, _ *proxy.Proxy, dctx *proxy.DNSContext) error {
		if opts.block != nil {
			<-opts.block
		}
		profile, device, idErr := s.clientIDFromDNSContext(dctx)
		if idErr != nil {
			return idErr
		}

		mu.Lock()
		last = seenRequest{
			profile:         profile,
			device:          device,
			protoMajor:      dctx.HTTPRequest.ProtoMajor,
			addr:            dctx.Addr.Addr(),
			isPrivateClient: dctx.IsPrivateClient,
		}
		mu.Unlock()

		resp := new(dns.Msg).SetReply(dctx.Req)
		resp.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{Name: dctx.Req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.IPv4(192, 0, 2, 1),
		}}
		dctx.Res = resp
		return nil
	})

	p, err := proxy.New(conf)
	require.NoError(t, err)
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	// Dual-stack wildcard, as in production; tests connect over IPv4.
	metrics := &recordingMetrics{}
	h3 := newDoH3Server(netip.MustParseAddrPort("[::]:0"), conf.TLSConfig, p, limits, opts.maxRequests, 64, metrics)
	require.NoError(t, h3.start())
	t.Cleanup(func() { _ = h3.shutdown(context.Background()) })

	tcpAddr, ok := p.Addr(proxy.ProtoHTTPS).(*net.TCPAddr)
	require.True(t, ok)

	return &dohTestEnv{
		h2Port:  tcpAddr.Port,
		h3:      h3,
		metrics: metrics,
		seen: func() seenRequest {
			mu.Lock()
			defer mu.Unlock()
			return last
		},
	}
}

func (e *dohTestEnv) h3URL(path string) string {
	return fmt.Sprintf("https://127.0.0.1:%d%s", e.h3.addr().(*net.UDPAddr).Port, path)
}

func (e *dohTestEnv) h2URL(path string) string {
	return fmt.Sprintf("https://127.0.0.1:%d%s", e.h2Port, path)
}

var insecureTLS = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // self-signed test certificate

func newH3Client(t *testing.T) *http.Client {
	t.Helper()
	tr := &http3.Transport{TLSClientConfig: insecureTLS.Clone()}
	t.Cleanup(func() { _ = tr.Close() })
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

func newH2Client(t *testing.T) *http.Client {
	t.Helper()
	tr := &http.Transport{TLSClientConfig: insecureTLS.Clone(), ForceAttemptHTTP2: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

func newDoHRequest(t *testing.T, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	return req
}

func packedQuery(t *testing.T) []byte {
	t.Helper()
	q := new(dns.Msg).SetQuestion("example.com.", dns.TypeA)
	q.Id = 0 // RFC 8484 §4.1
	body, err := q.Pack()
	require.NoError(t, err)
	return body
}

func dohQuery(t *testing.T, client *http.Client, url string, xff string) *dns.Msg {
	t.Helper()

	req := newDoHRequest(t, url, bytes.NewReader(packedQuery(t)))
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
		req.Header.Set("X-Real-IP", xff)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	msg := new(dns.Msg)
	require.NoError(t, msg.Unpack(raw))
	return msg
}

// specRef: proxy-request-admission-behaviour.md #Q15 #Q16
func TestDoH_HTTP3AndHTTP2ServeSameProfile(t *testing.T) {
	env := startDoHTestEnv(t, dohTestOpts{})
	path := "/dns-query/3mdq3851b9/laptop"

	for _, tc := range []struct {
		name       string
		client     *http.Client
		url        string
		protoMajor int
	}{
		{"h3", newH3Client(t), env.h3URL(path), 3},
		{"h2", newH2Client(t), env.h2URL(path), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := dohQuery(t, tc.client, tc.url, "")
			require.Len(t, msg.Answer, 1)

			got := env.seen()
			assert.Equal(t, tc.protoMajor, got.protoMajor)
			assert.Equal(t, "3mdq3851b9", got.profile)
			assert.Equal(t, "laptop", got.device)
			assert.Equal(t, netip.MustParseAddr("127.0.0.1"), got.addr)
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestDoH3_IgnoresForwardedAddressHeaders(t *testing.T) {
	// Loopback is trusted here, so HTTP/2 honours the headers while HTTP/3
	// keeps the QUIC peer address for every vendor stage too.
	env := startDoHTestEnv(t, dohTestOpts{trustedProxies: []string{"127.0.0.0/8"}})
	path := "/dns-query/3mdq3851b9"

	for _, tc := range []struct {
		name        string
		client      *http.Client
		url         string
		wantAddr    netip.Addr
		wantPrivate bool
	}{
		{"h2", newH2Client(t), env.h2URL(path), netip.MustParseAddr("1.2.3.4"), false},
		{"h3", newH3Client(t), env.h3URL(path), netip.MustParseAddr("127.0.0.1"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dohQuery(t, tc.client, tc.url, "1.2.3.4")

			got := env.seen()
			assert.Equal(t, tc.wantAddr, got.addr)
			assert.Equal(t, tc.wantPrivate, got.isPrivateClient)
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestDoH3_ConnectionCaps(t *testing.T) {
	for _, tc := range []struct {
		name       string
		limits     config.DoH3Limits
		wantReason string
	}{
		{"per IP", config.DoH3Limits{MaxConns: 8, MaxConnsPerIP: 1, MaxStreams: 8}, doh3RejectSource},
		{"global", config.DoH3Limits{MaxConns: 1, MaxConnsPerIP: 8, MaxStreams: 8}, doh3RejectTotal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := startDoHTestEnv(t, dohTestOpts{limits: tc.limits})
			url := env.h3URL("/dns-query/3mdq3851b9")

			// Each client holds its own QUIC connection.
			first := newH3Client(t)
			dohQuery(t, first, url, "")

			second := newH3Client(t)
			second.Timeout = time.Second
			_, err := second.Do(newDoHRequest(t, url, bytes.NewReader(packedQuery(t))))
			require.Error(t, err, "a connection above the cap must be refused")
			assert.Contains(t, env.metrics.snapshot().doh3Rejected, tc.wantReason)

			first.Transport.(*http3.Transport).Close()
			require.Eventually(t, func() bool {
				third := newH3Client(t)
				third.Timeout = time.Second
				resp, err := third.Do(newDoHRequest(t, url, bytes.NewReader(packedQuery(t))))
				if err != nil {
					return false
				}
				_ = resp.Body.Close()
				return resp.StatusCode == http.StatusOK
			}, 5*time.Second, 100*time.Millisecond, "a closed connection must free its slot")
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestDoH3_RequestDeadline(t *testing.T) {
	env := startDoHTestEnv(t, dohTestOpts{})
	env.h3.requestTimeout = 200 * time.Millisecond

	// The body never completes; the request must still end within the deadline.
	body, bodyWriter := io.Pipe()
	t.Cleanup(func() { _ = bodyWriter.Close() })
	partial := packedQuery(t)[:4]
	go func() { _, _ = bodyWriter.Write(partial) }()

	client := newH3Client(t)
	start := time.Now()
	resp, err := client.Do(newDoHRequest(t, env.h3URL("/dns-query/3mdq3851b9"), body))
	if err == nil {
		_ = resp.Body.Close()
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	}
	assert.Less(t, time.Since(start), 3*time.Second)
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestNewProxy_DoH3ListenerOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			s := &Server{
				Upstreams: map[string]*proxy.CustomUpstreamConfig{},
				edeStore:  &dnssec.EDEStore{},
			}
			serverConfig := newTestServerConfig(t, &config.DoHConfig{ListenAddr: 443, HTTP3Enabled: enabled, HTTP3: config.DoH3Limits{MaxConns: 1, MaxConnsPerIP: 1, MaxStreams: 1}})

			p, err := s.newProxy(ProxyTypeAdguard, serverConfig)
			require.NoError(t, err)

			if !enabled {
				assert.Nil(t, s.doh3)
				assert.NoError(t, s.StartDoH3(), "no listener configured is a no-op")
				return
			}
			require.NotNil(t, s.doh3)
			assert.Same(t, p, s.doh3.handler)
			assert.Equal(t, netip.AddrPortFrom(netip.IPv6Unspecified(), 443), s.doh3.listenAddr)
			assert.Equal(t, serverConfig.DoH.HTTP3, s.doh3.limits)
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestDoH3_IPv4ClientOnDualStackSocketIsUnmapped(t *testing.T) {
	env := startDoHTestEnv(t, dohTestOpts{})
	dohQuery(t, newH3Client(t), env.h3URL("/dns-query/3mdq3851b9"), "")

	got := env.seen()
	assert.True(t, got.addr.Is4(), "got %s", got.addr)
	assert.True(t, got.isPrivateClient, "loopback must be recognised as private")
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestDoH3_RejectsOversizedHeaders(t *testing.T) {
	env := startDoHTestEnv(t, dohTestOpts{})

	req := newDoHRequest(t, env.h3URL("/dns-query/3mdq3851b9"), bytes.NewReader(packedQuery(t)))
	req.Header.Set("X-Padding", string(bytes.Repeat([]byte("a"), 32<<10)))

	resp, err := newH3Client(t).Do(req)
	if err == nil {
		_ = resp.Body.Close()
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	}
}

// specRef: proxy-request-admission-behaviour.md #Q17
func TestDoH3_ConcurrentRequestsBoundedByMaxGoroutines(t *testing.T) {
	block := make(chan struct{})
	var unblock sync.Once
	release := func() { unblock.Do(func() { close(block) }) }
	// Registered first so it runs last, after the proxy and listener stop.
	t.Cleanup(release)
	env := startDoHTestEnv(t, dohTestOpts{maxRequests: 1, block: block})
	env.h3.requestTimeout = 300 * time.Millisecond
	url := env.h3URL("/dns-query/3mdq3851b9")

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		resp, err := newH3Client(t).Do(newDoHRequest(t, url, bytes.NewReader(packedQuery(t))))
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	require.Eventually(t, func() bool { return len(env.h3.requests) == 1 }, 2*time.Second, 10*time.Millisecond)

	resp, err := newH3Client(t).Do(newDoHRequest(t, url, bytes.NewReader(packedQuery(t))))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, 1, env.metrics.snapshot().doh3BusyCount)

	release()
	<-firstDone
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestDoH3_ShutdownReleasesSocket(t *testing.T) {
	d := newDoH3Server(netip.MustParseAddrPort("127.0.0.1:0"), newTestTLSConfig(t), http.NotFoundHandler(), config.DoH3Limits{MaxConns: 1, MaxConnsPerIP: 1, MaxStreams: 1}, 0, 64, &recordingMetrics{})
	require.NoError(t, d.start())
	addr := d.addr().(*net.UDPAddr)
	require.NoError(t, d.shutdown(context.Background()))

	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err, "the port must be free after shutdown")
	_ = conn.Close()
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestDoH3_ReportsListenerState(t *testing.T) {
	limits := config.DoH3Limits{MaxConns: 1, MaxConnsPerIP: 1, MaxStreams: 1}

	metrics := &recordingMetrics{}
	d := newDoH3Server(netip.MustParseAddrPort("127.0.0.1:0"), newTestTLSConfig(t), http.NotFoundHandler(), limits, 0, 64, metrics)
	require.NoError(t, d.start())
	t.Cleanup(func() { _ = d.shutdown(context.Background()) })
	assert.Equal(t, []bool{true}, metrics.snapshot().doh3ListenerUp)

	busy := d.addr().(*net.UDPAddr).AddrPort()
	metrics = &recordingMetrics{}
	s := &Server{doh3: newDoH3Server(busy, newTestTLSConfig(t), http.NotFoundHandler(), limits, 0, 64, metrics)}
	require.Error(t, s.StartDoH3(), "the port is taken")
	assert.Equal(t, []bool{false}, metrics.snapshot().doh3ListenerUp)
}

func newTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	certPath, keyPath := writeSelfSignedCert(t)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestPrepareRequest_RecordsDoHHTTPVersion(t *testing.T) {
	for _, tc := range []struct {
		proto        proxy.Proto
		major, minor int
		want         []string
	}{
		{proxy.ProtoHTTPS, 1, 1, []string{"1.1"}},
		{proxy.ProtoHTTPS, 2, 0, []string{"2"}},
		{proxy.ProtoHTTPS, 3, 0, []string{"3"}},
		{proxy.ProtoTLS, 0, 0, nil},
	} {
		t.Run(fmt.Sprintf("%s/%d.%d", tc.proto, tc.major, tc.minor), func(t *testing.T) {
			s := newProfileRateLimitServer(mocks.NewCache(t), config.RateLimitResponseRefuse)
			m := &recordingMetrics{}
			s.Metrics = m

			dctx := &proxy.DNSContext{
				Req:   new(dns.Msg).SetQuestion("example.com.", dns.TypeA),
				Proto: tc.proto,
				Addr:  netip.MustParseAddrPort("192.0.2.1:5000"),
			}
			if tc.proto == proxy.ProtoHTTPS {
				dctx.HTTPRequest = &http.Request{URL: &url.URL{Path: "/dns-query"}, ProtoMajor: tc.major, ProtoMinor: tc.minor}
			} else {
				dctx.Conn = fakeTLSConn{}
			}

			_, _, _ = s.prepareRequest(context.Background(), nil, dctx)
			assert.Equal(t, tc.want, m.snapshot().dohVersions)
		})
	}
}

type fakeTLSConn struct{ net.Conn }

func (fakeTLSConn) ConnectionState() tls.ConnectionState { return tls.ConnectionState{} }
