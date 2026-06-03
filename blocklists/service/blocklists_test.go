package service

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/internal/extractor"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
)

// fakeMetrics records validation-rejection reasons for assertions.
type fakeMetrics struct {
	metrics.NoopUpdates
	rejected []string
}

func (f *fakeMetrics) RecordValidationRejected(_, reason string) {
	f.rejected = append(f.rejected, reason)
}

func newGateService(threshold float64, m metrics.Updates) *Service {
	cfg := config.Config{Updater: &config.UpdaterConfig{ShrinkThreshold: threshold}}
	return &Service{Cfg: cfg, Metrics: m}
}

func prevMeta(entries int) []model.BlocklistMetadata {
	return []model.BlocklistMetadata{{Entries: entries}}
}

func TestCheckValidationGate(t *testing.T) {
	tests := []struct {
		name       string // specRef
		prev       []model.BlocklistMetadata
		newCount   int
		header     int
		wantErr    bool
		wantReason string
	}{
		// specRef: #J1 — empty result is always rejected.
		{name: "J1_empty", prev: prevMeta(100), newCount: 0, wantErr: true, wantReason: metrics.ReasonEmpty},
		// specRef: #J2 — shrink beyond threshold is rejected.
		{name: "J2_shrink", prev: prevMeta(100), newCount: 10, wantErr: true, wantReason: metrics.ReasonShrink},
		// specRef: #J3 — shrink within threshold is allowed.
		{name: "J3_within", prev: prevMeta(100), newCount: 60, wantErr: false},
		// specRef: #J4 — first run (no previous metadata) only fails on empty.
		{name: "J4_firstrun", prev: nil, newCount: 10, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm := &fakeMetrics{}
			s := newGateService(0.5, fm)

			err := s.checkValidationGate("blp_test", tc.prev, tc.newCount, tc.header)

			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantReason != "" {
				if len(fm.rejected) != 1 || fm.rejected[0] != tc.wantReason {
					t.Fatalf("rejected reasons = %v, want [%s]", fm.rejected, tc.wantReason)
				}
			} else if len(fm.rejected) != 0 {
				t.Fatalf("expected no rejection, got %v", fm.rejected)
			}
		})
	}
}

// specRef: #I10 — an oversized line surfaces as a scanner error so the caller
// can abort, rather than silently truncating the rest of the source.
func TestScanValidatedDomains_OversizedLineErrors(t *testing.T) {
	extr, err := extractor.NewExtractor("blp_test")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	// One line larger than bufio.Scanner's default 64KB cap.
	oversized := strings.Repeat("a", bufio.MaxScanTokenSize+1)
	_, err = scanValidatedDomains(extr, strings.NewReader(oversized+"\nexample.com\n"))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("scanValidatedDomains err = %v, want bufio.ErrTooLong", err)
	}
}

// specRef: #I10 — normal input scans cleanly into validated domains.
func TestScanValidatedDomains_ParsesDomains(t *testing.T) {
	extr, err := extractor.NewExtractor("blp_test")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got, err := scanValidatedDomains(extr, strings.NewReader("example.com\n# comment\n\nads.example.net\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"example.com", "ads.example.net"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("scanValidatedDomains = %v, want %v", got, want)
	}
}

// specRef: #B4 — a download exceeding the size limit is rejected (not truncated).
func TestDownload_TooLargeRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	}))
	defer srv.Close()

	// Lower the limit so 64 bytes exceeds it, then restore.
	orig := maxBlocklistSize
	maxBlocklistSize = 16
	defer func() { maxBlocklistSize = orig }()

	s := &Service{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.download(ctx, srv.URL)
	if !errors.Is(err, errDownloadTooLarge) {
		t.Fatalf("download err = %v, want errDownloadTooLarge", err)
	}
}

// specRef: #B4 — a download within the limit succeeds.
func TestDownload_WithinLimitOK(t *testing.T) {
	body := "example.com\nads.example.net\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &Service{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := s.download(ctx, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != body {
		t.Fatalf("download body = %q, want %q", got, body)
	}
}
