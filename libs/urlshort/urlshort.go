package urlshort

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

// Option is a function that configures the URLShortener.
type Option func(*URLShortener)

// URLEntry carries metadata the caller can persist externally.
type URLEntry struct {
	OriginalURL string
	Data        []byte
	TTL         time.Duration
	CreatedAt   time.Time
}

// URLShortener generates short tokens; callers persist entries elsewhere.
type URLShortener struct {
	defaultTTL  time.Duration
	shortLength int
}

// NewURLShortener creates a new URL shortener with configurable defaults.
func NewURLShortener(options ...Option) *URLShortener {
	s := &URLShortener{
		defaultTTL:  time.Hour, // Default 1 hour expiration
		shortLength: 6,         // Default 6 character short URLs
	}

	for _, option := range options {
		option(s)
	}

	return s
}

// WithDefaultTTL sets the default time-to-live for generated URLs.
func WithDefaultTTL(ttl time.Duration) Option {
	return func(s *URLShortener) {
		if ttl > 0 {
			s.defaultTTL = ttl
		}
	}
}

// WithShortLength sets the length of generated short URLs.
func WithShortLength(length int) Option {
	return func(s *URLShortener) {
		if length > 0 {
			s.shortLength = length
		}
	}
}

// WithStatsLogging is kept for backward compatibility; it is a no-op because
// storage is external and stats collection is delegated to the backing cache.
func WithStatsLogging(_ time.Duration) Option {
	return func(_ *URLShortener) {}
}

// DefaultTTL returns the configured default TTL.
// func (s *URLShortener) DefaultTTL() time.Duration {
// 	return s.defaultTTL
// }

// Shorten creates a short token for the provided URL using the default TTL.
func (s *URLShortener) Shorten(originalURL string) (string, URLEntry, error) {
	return s.ShortenWithTTL(originalURL, s.defaultTTL)
}

// ShortenWithTTL creates a short token for the provided URL and TTL.
func (s *URLShortener) ShortenWithTTL(originalURL string, ttl time.Duration) (string, URLEntry, error) {
	if originalURL == "" {
		return "", URLEntry{}, errors.New("original URL cannot be empty")
	}

	if ttl <= 0 {
		ttl = s.defaultTTL
	}

	token, err := s.generateShortURLToken()
	if err != nil {
		return "", URLEntry{}, err
	}

	return token, URLEntry{
		OriginalURL: originalURL,
		TTL:         ttl,
		CreatedAt:   time.Now(),
	}, nil
}

// ShortenWithData creates a short token and returns entry metadata with data attached.
func (s *URLShortener) ShortenWithData(originalURL string, data []byte) (string, URLEntry, error) {
	if originalURL == "" {
		return "", URLEntry{}, errors.New("original URL cannot be empty")
	}

	token, err := s.generateShortURLToken()
	if err != nil {
		return "", URLEntry{}, err
	}

	return token, URLEntry{
		OriginalURL: originalURL,
		Data:        data,
		TTL:         s.defaultTTL,
		CreatedAt:   time.Now(),
	}, nil
}

// generateShortURL creates a random short URL of configured length.
func (s *URLShortener) generateShortURLToken() (string, error) {
	bytes := make([]byte, (s.shortLength*6+7)/8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	return encoded[:s.shortLength], nil
}
