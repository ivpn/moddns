package announcements

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	fetchTimeout = 10 * time.Second
	// MaxBodyBytes is the cap on the announcements feed the API will read from
	// ANNOUNCEMENTS_URL. Bytes past this point are silently truncated, so the
	// announcements-validate CLI warns as the file approaches it.
	MaxBodyBytes = 1 << 20 // 1 MB
)

// Loader fetches and caches announcements from an HTTP source. The cached value
// is served from memory so the request path never blocks on the upstream, and
// the last known-good list is retained if a refresh fails.
type Loader struct {
	url         string
	reloadEvery time.Duration
	client      *http.Client

	mu       sync.RWMutex
	cached   []Announcement
	lastErr  error
	lastLoad time.Time
}

// New creates a Loader that fetches announcements from url. If url is empty the
// loader is a no-op that always returns an empty list (feature disabled). An
// initial fetch is attempted but failure does not prevent startup; the
// background refresh in Start will retry.
func New(url string, reloadEvery time.Duration) (*Loader, error) {
	if reloadEvery <= 0 {
		reloadEvery = 5 * time.Minute
	}
	l := &Loader{
		url:         url,
		reloadEvery: reloadEvery,
		client:      &http.Client{Timeout: fetchTimeout},
		cached:      []Announcement{},
	}
	if url == "" {
		log.Info().Msg("ANNOUNCEMENTS_URL not set; announcements feature disabled")
		return l, nil
	}
	if err := l.Reload(context.Background()); err != nil {
		log.Warn().Err(err).Str("url", url).Msg("Initial announcements load failed; will retry on schedule")
	}
	return l, nil
}

// Start periodically refreshes the cache until ctx is cancelled.
func (l *Loader) Start(ctx context.Context) {
	if l == nil || l.url == "" {
		return
	}
	ticker := time.NewTicker(l.reloadEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = l.Reload(ctx)
		}
	}
}

// Reload fetches and parses the source, updating the cache. On failure the last
// known-good cache is kept.
func (l *Loader) Reload(ctx context.Context) error {
	if l == nil || l.url == "" {
		return nil
	}
	anns, err := l.fetch(ctx)

	l.mu.Lock()
	l.lastLoad = time.Now()
	l.lastErr = err
	if err == nil {
		l.cached = anns
	}
	l.mu.Unlock()

	if err != nil {
		log.Error().Err(err).Str("url", l.url).Msg("Failed to load announcements")
		return err
	}
	log.Trace().Str("url", l.url).Int("announcements", len(anns)).Msg("Announcements loaded")
	return nil
}

func (l *Loader) fetch(ctx context.Context) ([]Announcement, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status fetching announcements: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes))
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Get returns the cached announcements (last known-good). It never returns nil
// and never blocks on the upstream.
func (l *Loader) Get() []Announcement {
	if l == nil {
		return []Announcement{}
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.cached == nil {
		return []Announcement{}
	}
	return l.cached
}
