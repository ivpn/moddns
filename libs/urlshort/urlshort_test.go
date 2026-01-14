package urlshort

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestShortenWithDataReturnsEntryForExternalStorage(t *testing.T) {
	shortener := NewURLShortener(WithDefaultTTL(time.Hour), WithShortLength(8))

	token, entry, err := shortener.ShortenWithData("https://example.com/"+uuid.NewString(), []byte("payload"))
	if err != nil {
		t.Fatalf("ShortenWithData() error = %v", err)
	}

	if len(token) != 8 {
		t.Fatalf("unexpected token length: got %d", len(token))
	}

	if entry.TTL != time.Hour {
		t.Fatalf("unexpected TTL: got %v want %v", entry.TTL, time.Hour)
	}

	if string(entry.Data) != "payload" {
		t.Fatalf("unexpected data: %s", string(entry.Data))
	}
}

func TestShortenWithTTLOverridesDefault(t *testing.T) {
	shortener := NewURLShortener()

	customTTL := 15 * time.Minute
	token, entry, err := shortener.ShortenWithTTL("https://example.com", customTTL)
	if err != nil {
		t.Fatalf("ShortenWithTTL() error = %v", err)
	}

	if entry.TTL != customTTL {
		t.Fatalf("TTL not applied: got %v want %v", entry.TTL, customTTL)
	}

	if token == "" {
		t.Fatalf("token should not be empty")
	}
}

func TestShortenRejectsEmptyURL(t *testing.T) {
	shortener := NewURLShortener()

	if _, _, err := shortener.Shorten(""); err == nil {
		t.Fatalf("expected error for empty URL")
	}
}
