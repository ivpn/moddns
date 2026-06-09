package downloader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// retryRec is a RetryMetric that counts RecordRetry calls.
type retryRec struct {
	mu sync.Mutex
	n  int
}

func (r *retryRec) RecordRetry(string) {
	r.mu.Lock()
	r.n++
	r.mu.Unlock()
}

func (r *retryRec) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

// sleepRec records requested backoff durations and returns immediately, so
// retry/timing tests are deterministic and fast.
type sleepRec struct {
	mu    sync.Mutex
	calls []time.Duration
}

func (s *sleepRec) fn(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	s.calls = append(s.calls, d)
	s.mu.Unlock()
	return nil
}

func (s *sleepRec) snapshot() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.calls))
	copy(out, s.calls)
	return out
}

// newTestDownloader builds a Downloader with a non-blocking sleep recorder and
// deterministic jitter (rng returns 0 → equal-jitter floor).
func newTestDownloader(t *testing.T, cfg Config, metric RetryMetric) (*Downloader, *sleepRec) {
	t.Helper()
	d := New(cfg, metric)
	rec := &sleepRec{}
	d.sleep = rec.fn
	d.rng = func(int64) int64 { return 0 }
	return d, rec
}

// specRef: #B1, #B11 — 200 succeeds; descriptive User-Agent sent on every request.
func TestFetch_SuccessSendsUserAgent(t *testing.T) {
	const body = "example.com\nads.example.net\n"
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	d, _ := newTestDownloader(t, Config{}, nil)
	got, err := d.Fetch(context.Background(), "src", srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
	if gotUA != DefaultUserAgent {
		t.Fatalf("User-Agent = %q, want %q", gotUA, DefaultUserAgent)
	}
}

// specRef: #B9, #B14 — 5xx is retried with backoff; each retry is metered.
func TestFetch_RetriesOn5xxThenSucceeds(t *testing.T) {
	const body = "ok.example.com\n"
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&reqs, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	metric := &retryRec{}
	d, _ := newTestDownloader(t, Config{}, metric)
	got, err := d.Fetch(context.Background(), "src", srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
	if n := atomic.LoadInt32(&reqs); n != 2 {
		t.Fatalf("requests = %d, want 2", n)
	}
	if metric.count() != 1 {
		t.Fatalf("retry metric = %d, want 1", metric.count())
	}
}

// specRef: #B8 — 429 is retried, honouring the Retry-After header.
func TestFetch_Honors429RetryAfter(t *testing.T) {
	const body = "ok.example.com\n"
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&reqs, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	d, rec := newTestDownloader(t, Config{}, nil)
	if _, err := d.Fetch(context.Background(), "src", srv.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("backoff sleeps = %d, want 1", len(calls))
	}
	if calls[0] != time.Second {
		t.Fatalf("honoured backoff = %v, want 1s (Retry-After)", calls[0])
	}
}

// specRef: #B7 — an abrupt reset/EOF is a transient error and is retried.
func TestFetch_RetriesOnConnectionReset(t *testing.T) {
	const body = "ok.example.com\n"
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&reqs, 1) == 1 {
			// Simulate an abrupt reset/EOF: hijack and close without responding.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("ResponseWriter is not a Hijacker")
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Errorf("Hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	metric := &retryRec{}
	d, _ := newTestDownloader(t, Config{}, metric)
	got, err := d.Fetch(context.Background(), "src", srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
	if metric.count() != 1 {
		t.Fatalf("retry metric = %d, want 1", metric.count())
	}
}

// specRef: #B10 — a non-429 4xx aborts immediately (no retry).
func TestFetch_NonRetryable404(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqs, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	metric := &retryRec{}
	d, _ := newTestDownloader(t, Config{}, metric)
	if _, err := d.Fetch(context.Background(), "src", srv.URL); err == nil {
		t.Fatal("Fetch err = nil, want bad status")
	}
	if n := atomic.LoadInt32(&reqs); n != 1 {
		t.Fatalf("requests = %d, want 1 (404 must not retry)", n)
	}
	if metric.count() != 0 {
		t.Fatalf("retry metric = %d, want 0", metric.count())
	}
}

// specRef: #B4 — an over-size body returns ErrTooLarge and is not retried.
func TestFetch_TooLargeNotRetried(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqs, 1)
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	}))
	defer srv.Close()

	d, _ := newTestDownloader(t, Config{MaxBodySize: 16}, nil)
	_, err := d.Fetch(context.Background(), "src", srv.URL)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if n := atomic.LoadInt32(&reqs); n != 1 {
		t.Fatalf("requests = %d, want 1 (too-large must not retry)", n)
	}
}

// specRef: #B9 — persistent 5xx returns an error after MaxAttempts.
func TestFetch_ExhaustsRetriesReturnsError(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqs, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	metric := &retryRec{}
	d, _ := newTestDownloader(t, Config{MaxAttempts: 3}, metric)
	if _, err := d.Fetch(context.Background(), "src", srv.URL); err == nil {
		t.Fatal("Fetch err = nil, want bad status after exhausting retries")
	}
	if n := atomic.LoadInt32(&reqs); n != 3 {
		t.Fatalf("requests = %d, want 3 (MaxAttempts)", n)
	}
	if metric.count() != 2 {
		t.Fatalf("retry metric = %d, want 2 (attempts-1)", metric.count())
	}
}

// specRef: #B7 — a done caller context is terminal: no request, no retry.
func TestFetch_ContextCancelledNoRequest(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqs, 1)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d, _ := newTestDownloader(t, Config{}, nil)
	_, err := d.Fetch(ctx, "src", srv.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if n := atomic.LoadInt32(&reqs); n != 0 {
		t.Fatalf("requests = %d, want 0 (cancelled ctx)", n)
	}
}

// TestFetch_PerHostSerialized verifies the per-host gate keeps at most one
// in-flight request per host even under concurrent callers.
// specRef: #B12 — at most one in-flight request per host.
func TestFetch_PerHostSerialized(t *testing.T) {
	var inFlight, maxInFlight int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if cur <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer srv.Close()

	// Real sleep, negligible per-host interval so the test only measures the
	// one-in-flight guarantee.
	d := New(Config{PerHostMinInterval: time.Millisecond, MaxConcurrency: 8}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := d.Fetch(context.Background(), "src", srv.URL); err != nil {
				t.Errorf("Fetch: %v", err)
			}
		}()
	}
	wg.Wait()
	if m := atomic.LoadInt32(&maxInFlight); m != 1 {
		t.Fatalf("max concurrent same-host requests = %d, want 1", m)
	}
}

// TestFetch_GlobalConcurrencyCap verifies the global semaphore bounds total
// in-flight downloads across distinct hosts.
// specRef: #B13 — global cap bounds total in-flight downloads across hosts.
func TestFetch_GlobalConcurrencyCap(t *testing.T) {
	const cap = 2
	var inFlight, maxInFlight int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if cur <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, cur) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		_, _ = w.Write([]byte("ok\n"))
	})

	const hosts = 5
	servers := make([]*httptest.Server, hosts)
	for i := range servers {
		servers[i] = httptest.NewServer(handler)
		defer servers[i].Close()
	}

	d := New(Config{MaxConcurrency: cap, PerHostMinInterval: time.Millisecond}, nil)
	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			if _, err := d.Fetch(context.Background(), "src", url); err != nil {
				t.Errorf("Fetch: %v", err)
			}
		}(srv.URL)
	}
	wg.Wait()
	if m := atomic.LoadInt32(&maxInFlight); m > cap {
		t.Fatalf("max concurrent downloads = %d, want <= %d", m, cap)
	}
}

// TestFetch_PerHostMinInterval verifies consecutive requests to the same host
// are spaced by at least PerHostMinInterval.
// specRef: #B12 — consecutive same-host requests are spaced by the min interval.
func TestFetch_PerHostMinInterval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer srv.Close()

	const interval = 250 * time.Millisecond
	d, rec := newTestDownloader(t, Config{PerHostMinInterval: interval}, nil)

	// First request records lastReq; the second must wait out the interval.
	if _, err := d.Fetch(context.Background(), "src", srv.URL); err != nil {
		t.Fatalf("Fetch 1: %v", err)
	}
	if _, err := d.Fetch(context.Background(), "src", srv.URL); err != nil {
		t.Fatalf("Fetch 2: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("interval sleeps = %d, want 1", len(calls))
	}
	if calls[0] < interval-20*time.Millisecond {
		t.Fatalf("spacing wait = %v, want ~%v", calls[0], interval)
	}
}

// specRef: #B15 — the shared client reuses keep-alive connections across
// sequential fetches to the same host (same client source port).
func TestFetch_ReusesConnection(t *testing.T) {
	var mu sync.Mutex
	var remotes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remotes = append(remotes, r.RemoteAddr)
		mu.Unlock()
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer srv.Close()

	d, _ := newTestDownloader(t, Config{PerHostMinInterval: time.Millisecond}, nil)
	for i := 0; i < 2; i++ {
		if _, err := d.Fetch(context.Background(), "src", srv.URL); err != nil {
			t.Fatalf("Fetch %d: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(remotes) != 2 {
		t.Fatalf("requests = %d, want 2", len(remotes))
	}
	if remotes[0] != remotes[1] {
		t.Fatalf("client addrs = %v, want both equal (connection reused)", remotes)
	}
}

// specRef: #B8 — Retry-After parsing (seconds, HTTP-date, invalid/past).
func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{"-3", 0},
		{"garbage", 0},
		{now.Add(10 * time.Second).UTC().Format(http.TimeFormat), 10 * time.Second},
		{now.Add(-10 * time.Second).UTC().Format(http.TimeFormat), 0},
	}
	for _, c := range cases {
		if got := parseRetryAfter(c.in, now); got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
