package announcements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const loaderSample = `---
id: a1
category: news
severity: info
title: First
published_at: 2026-01-01
---
hello
`

func TestLoader_EmptyURLIsNoop(t *testing.T) {
	l, err := New("", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := l.Get(); len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
	// Reload and Start must be safe no-ops.
	if err := l.Reload(context.Background()); err != nil {
		t.Fatalf("reload error: %v", err)
	}
}

func TestLoader_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(loaderSample))
	}))
	defer srv.Close()

	l, err := New(srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := l.Get()
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("expected one announcement a1, got %+v", got)
	}
}

func TestLoader_Non200DoesNotFailStartup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	l, err := New(srv.URL, 0)
	if err != nil {
		t.Fatalf("New must not fail on upstream error, got: %v", err)
	}
	if got := l.Get(); len(got) != 0 {
		t.Fatalf("expected empty cache, got %d", len(got))
	}
}

func TestLoader_LastKnownGoodOnFailure(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(loaderSample))
	}))
	defer srv.Close()

	l, err := New(srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(l.Get()) != 1 {
		t.Fatalf("expected initial good load")
	}

	// Upstream now fails; the cache must retain the last known-good value.
	fail.Store(true)
	if err := l.Reload(context.Background()); err == nil {
		t.Fatal("expected reload error on upstream failure")
	}
	if got := l.Get(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("expected last known-good retained, got %+v", got)
	}
}

func TestLoader_CancelledContextRetainsCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(loaderSample))
	}))
	defer srv.Close()

	l, err := New(srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled -> fetch should error
	if err := l.Reload(ctx); err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if got := l.Get(); len(got) != 1 {
		t.Fatalf("expected cache retained after cancelled reload, got %d", len(got))
	}
}
