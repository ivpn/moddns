package service

import (
	"bufio"
	"errors"
	"strings"
	"testing"

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
	// One line larger than bufio.Scanner's default 64KB cap.
	oversized := strings.Repeat("a", bufio.MaxScanTokenSize+1)
	_, err := scanValidatedDomains(strings.NewReader(oversized + "\nexample.com\n"))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("scanValidatedDomains err = %v, want bufio.ErrTooLong", err)
	}
}

// specRef: #D15 #D16 #D17 — normalization (CRLF/case/trailing-dot) and
// validation (comments and garbage dropped, punycode kept) applied to the
// Convert output that feeds both Redis and Mongo.
func TestScanValidatedDomains_NormalizesAndValidates(t *testing.T) {
	input := strings.Join([]string{
		"Example.COM",           // case -> lowercased
		"ads.example.net\r",     // CRLF -> CR stripped
		"trailing.example.org.", // trailing dot -> stripped
		"# a comment",           // comment -> rejected
		"",                      // blank -> skipped
		"two words.com",         // injected space -> rejected
		"*.wildcard.example",    // wildcard syntax -> rejected
		"tencent.xn--io0a7i",    // punycode TLD -> kept
	}, "\n")

	got, err := scanValidatedDomains(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"example.com", "ads.example.net", "trailing.example.org", "tencent.xn--io0a7i"}
	if len(got) != len(want) {
		t.Fatalf("scanValidatedDomains = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scanValidatedDomains[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// specRef: #D6 — steven_black hosts entries flow end-to-end (Convert -> shared
// validate) and are normalized; guards the Phase B unify-to-cache change which
// previously dropped them because ProcessLine re-parsed bare domains as hosts.
func TestStevenBlackEndToEnd(t *testing.T) {
	extr, err := extractor.NewExtractor("steven_black_test")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}
	hosts := "# Title: test\n0.0.0.0 Ads.Example.COM\n0.0.0.0 0.0.0.0\n127.0.0.1 skip.example.org\n"
	converted, err := extr.Convert([]byte(hosts))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got, err := scanValidatedDomains(strings.NewReader(string(converted)))
	if err != nil {
		t.Fatalf("scanValidatedDomains: %v", err)
	}
	if len(got) != 1 || got[0] != "ads.example.com" {
		t.Fatalf("steven_black end-to-end = %v, want [ads.example.com]", got)
	}
}
