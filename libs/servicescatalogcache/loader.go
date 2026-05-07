package servicescatalogcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ivpn/dns/libs/servicescatalog"
	"github.com/rs/zerolog/log"
)

type Loader struct {
	path        string
	reloadEvery time.Duration

	mu       sync.RWMutex
	catalog  *servicescatalog.Catalog
	lastErr  error
	lastLoad time.Time
}

func New(path string, reloadEvery time.Duration) (*Loader, error) {
	if path == "" {
		return nil, errors.New("services catalog path is required")
	}
	if reloadEvery <= 0 {
		reloadEvery = 5 * time.Minute
	}
	l := &Loader{path: path, reloadEvery: reloadEvery}
	if err := l.Reload(); err != nil {
		return nil, fmt.Errorf("initial catalog load: %w", err)
	}
	return l, nil
}

func (l *Loader) Start(ctx context.Context) {
	if l == nil {
		return
	}

	ticker := time.NewTicker(l.reloadEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = l.Reload()
		}
	}
}

func (l *Loader) Reload() error {
	if l == nil {
		return nil
	}

	cat, err := servicescatalog.LoadFromFile(l.path)

	l.mu.Lock()
	l.lastLoad = time.Now()
	l.lastErr = err
	if err == nil {
		// Keep the last known-good catalog if reload fails.
		l.catalog = cat
	}
	l.mu.Unlock()

	if err != nil {
		log.Error().Err(err).Str("path", l.path).Msg("Failed to load services catalog")
		return err
	}

	log.Trace().Str("path", l.path).Int("services", len(cat.Services)).Msg("Services catalog loaded")
	return nil
}

func (l *Loader) Get() (*servicescatalog.Catalog, error) {
	if l == nil {
		return nil, nil
	}
	l.mu.RLock()
	cat := l.catalog
	err := l.lastErr
	l.mu.RUnlock()
	if cat != nil && err == nil {
		return cat, nil
	}
	if err := l.Reload(); err != nil {
		return nil, err
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.catalog, l.lastErr
}
