// Package downloader provides a "gentle" HTTP fetcher for blocklist sources.
//
// The blocklists service registers every source on an `@hourly` cron, and
// robfig/cron runs each job in its own goroutine, so at the top of the hour all
// sources fire at once. Several sources share a host (e.g. big/small/nsfw on
// *.oisd.nl, 20+ on raw.githubusercontent.com), so a naive fetch opens many
// simultaneous connections to the same server and provokes connection resets,
// EOFs and HTTP 429s.
//
// Downloader makes the fetch path well-behaved without changing the per-source
// scheduling model:
//
//   - a single reusable http.Client (keep-alives) with a descriptive User-Agent
//     instead of the default Go-http-client/1.1 bot signature,
//   - a per-host gate that allows only one in-flight request per host and
//     enforces a minimum spacing between consecutive requests to that host,
//   - a global concurrency cap so a thundering herd of cron jobs never opens
//     more than N connections at once,
//   - bounded retry with exponential backoff + jitter on transient failures
//     (network errors, 429, 5xx), honouring the Retry-After header.
//
// It is safe for concurrent use by many goroutines.
package downloader

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ErrTooLarge is returned when a response body exceeds Config.MaxBodySize. It is
// deliberately non-retryable: an oversized/abusive body should fail the swap, not
// be hammered again.
var ErrTooLarge = errors.New("download exceeded max size")

// RetryMetric records that a download was retried. The blocklists metrics.Updates
// interface satisfies it; a nil metric is replaced with a no-op.
type RetryMetric interface {
	RecordRetry(source string)
}

type noopMetric struct{}

func (noopMetric) RecordRetry(string) {}

// Config holds the tuning knobs for a Downloader. All fields have sane defaults
// applied by New, so a zero Config yields a working, conservative downloader.
type Config struct {
	// Timeout bounds a single attempt (connect + headers + body read).
	Timeout time.Duration
	// MaxConcurrency caps simultaneous in-flight downloads across all hosts.
	MaxConcurrency int
	// PerHostMinInterval is the minimum spacing between consecutive requests to
	// the same host. Combined with the per-host gate (one in-flight request per
	// host) this is what stops sibling sources from hammering a shared server.
	PerHostMinInterval time.Duration
	// MaxAttempts is the total number of attempts (1 = no retry).
	MaxAttempts int
	// BaseRetryDelay is the first backoff delay; it doubles each attempt.
	BaseRetryDelay time.Duration
	// MaxRetryDelay caps a single backoff delay (and a honoured Retry-After).
	MaxRetryDelay time.Duration
	// MaxBodySize is the largest accepted response body in bytes.
	MaxBodySize int64
	// UserAgent is sent on every request.
	UserAgent string
}

// Default configuration values. Chosen to be gentle on source servers while
// keeping a full refresh well within the service's 1-minute processing timeout.
const (
	defaultTimeout            = 30 * time.Second
	defaultMaxConcurrency     = 4
	defaultPerHostMinInterval = 2 * time.Second
	defaultMaxAttempts        = 3
	defaultBaseRetryDelay     = 1 * time.Second
	defaultMaxRetryDelay      = 30 * time.Second
	defaultMaxBodySize        = 100 * 1024 * 1024
	// DefaultUserAgent identifies the updater so CDNs/WAFs treat it as a known
	// client rather than an anonymous bot.
	DefaultUserAgent = "moddns-blocklists/1.0 (+https://www.moddns.net)"
)

func (c *Config) applyDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = defaultMaxConcurrency
	}
	if c.PerHostMinInterval < 0 {
		c.PerHostMinInterval = 0
	} else if c.PerHostMinInterval == 0 {
		c.PerHostMinInterval = defaultPerHostMinInterval
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.BaseRetryDelay <= 0 {
		c.BaseRetryDelay = defaultBaseRetryDelay
	}
	if c.MaxRetryDelay <= 0 {
		c.MaxRetryDelay = defaultMaxRetryDelay
	}
	if c.MaxBodySize <= 0 {
		c.MaxBodySize = defaultMaxBodySize
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
}

// hostGate serializes and spaces requests to a single host. Holding mu for the
// whole fetch (including retries) guarantees one in-flight request per host;
// lastReq records when the previous request to this host completed so the next
// one can wait out PerHostMinInterval.
type hostGate struct {
	mu      sync.Mutex
	lastReq time.Time
}

// Downloader is a gentle, concurrency-limited HTTP fetcher. Construct it with New.
type Downloader struct {
	cfg    Config
	client *http.Client
	metric RetryMetric

	sem chan struct{} // global concurrency limiter

	gatesMu sync.Mutex
	gates   map[string]*hostGate

	// Injection points for deterministic tests.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
	rng   func(n int64) int64
}

// New returns a Downloader using cfg (defaults applied for zero fields). A nil
// metric is replaced with a no-op so call sites need not guard it.
func New(cfg Config, metric RetryMetric) *Downloader {
	cfg.applyDefaults()
	if metric == nil {
		metric = noopMetric{}
	}
	return &Downloader{
		cfg:    cfg,
		metric: metric,
		client: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		sem:   make(chan struct{}, cfg.MaxConcurrency),
		gates: make(map[string]*hostGate),
		now:   time.Now,
		sleep: sleepCtx,
		rng:   rand.Int63n,
	}
}

// Fetch downloads rawURL and returns the response body. source is used only for
// the retry metric label (pass "" when irrelevant). It blocks until a global
// concurrency slot and the per-host gate are free, then attempts the download
// with bounded retry/backoff.
func (d *Downloader) Fetch(ctx context.Context, source, rawURL string) ([]byte, error) {
	host := hostKey(rawURL)

	// Per-host gate first (one in-flight request per host + min spacing), THEN
	// the global slot. Acquiring the host gate before the global slot means a
	// goroutine waiting out the per-host interval is not holding a global slot,
	// so it cannot starve downloads to other hosts.
	gate := d.gateFor(host)
	gate.mu.Lock()
	defer func() {
		gate.lastReq = d.now()
		gate.mu.Unlock()
	}()
	if !gate.lastReq.IsZero() {
		if wait := d.cfg.PerHostMinInterval - d.now().Sub(gate.lastReq); wait > 0 {
			if err := d.sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
	}

	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-d.sem }()

	return d.fetchWithRetry(ctx, source, rawURL)
}

func (d *Downloader) fetchWithRetry(ctx context.Context, source, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= d.cfg.MaxAttempts; attempt++ {
		body, retryAfter, retryable, err := d.attempt(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable || attempt == d.cfg.MaxAttempts || ctx.Err() != nil {
			return nil, err
		}

		delay := d.backoff(attempt, retryAfter)
		d.metric.RecordRetry(source)
		log.Warn().
			Err(err).
			Str("source_url", rawURL).
			Int("attempt", attempt).
			Dur("retry_in", delay).
			Msg("Blocklist download failed, backing off before retry")
		if err := d.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// attempt performs a single GET. It returns the body on success, or a
// classification of the failure: retryAfter is the server-requested delay (0 if
// none), retryable indicates whether another attempt may help.
func (d *Downloader) attempt(ctx context.Context, rawURL string) (body []byte, retryAfter time.Duration, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, false, err // malformed URL — retrying cannot help
	}
	req.Header.Set("User-Agent", d.cfg.UserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		// A cancelled/expired caller context is terminal; any other transport
		// error (EOF, connection reset, timeout, DNS) may be transient.
		if ctx.Err() != nil {
			return nil, 0, false, err
		}
		return nil, 0, true, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var buf bytes.Buffer
		// Read MaxBodySize+1 so an oversized body is detected, not truncated.
		n, err := io.CopyBuffer(&buf, io.LimitReader(resp.Body, d.cfg.MaxBodySize+1), make([]byte, 32*1024))
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, false, err
			}
			return nil, 0, true, err // mid-stream reset/EOF — retryable
		}
		if n > d.cfg.MaxBodySize {
			return nil, 0, false, fmt.Errorf("%w: %d bytes", ErrTooLarge, d.cfg.MaxBodySize)
		}
		return buf.Bytes(), 0, false, nil

	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		drain(resp.Body)
		ra := parseRetryAfter(resp.Header.Get("Retry-After"), d.now())
		return nil, ra, true, fmt.Errorf("bad status: %s", resp.Status)

	default:
		drain(resp.Body)
		return nil, 0, false, fmt.Errorf("bad status: %s", resp.Status)
	}
}

// backoff returns the delay before the next attempt. A server-supplied
// Retry-After (capped at MaxRetryDelay) wins; otherwise it is exponential
// (BaseRetryDelay << (attempt-1)) capped at MaxRetryDelay, with equal jitter so
// sibling sources do not retry in lockstep.
func (d *Downloader) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > d.cfg.MaxRetryDelay {
			return d.cfg.MaxRetryDelay
		}
		return retryAfter
	}

	base := d.cfg.BaseRetryDelay
	for i := 1; i < attempt; i++ {
		base *= 2
		if base >= d.cfg.MaxRetryDelay {
			base = d.cfg.MaxRetryDelay
			break
		}
	}
	half := base / 2
	if half <= 0 {
		return base
	}
	return half + time.Duration(d.rng(int64(half)+1)) // [half, base]
}

func (d *Downloader) gateFor(host string) *hostGate {
	d.gatesMu.Lock()
	defer d.gatesMu.Unlock()
	g := d.gates[host]
	if g == nil {
		g = &hostGate{}
		d.gates[host] = g
	}
	return g
}

// hostKey returns the host (incl. port) used to group requests. A URL that does
// not parse falls back to the raw string so it still gets its own gate.
func hostKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

// drain reads and discards a bounded amount of the body so the connection can be
// reused by keep-alive, without slurping a large error page.
func drain(r io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(r, 4096))
}

// parseRetryAfter interprets a Retry-After header value, which is either a
// delay in seconds or an HTTP date. It returns 0 when absent/invalid/in the past.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
