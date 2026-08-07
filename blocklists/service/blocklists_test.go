package service

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivpn/dns/blocklists/config"
	"github.com/ivpn/dns/blocklists/internal/extractor"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/ivpn/dns/libs/dislock"
	"go.mongodb.org/mongo-driver/mongo"
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

// fakeStore implements db.Db, serving fixed metadata and recording deletions.
type fakeStore struct {
	metadata       []model.BlocklistMetadata
	deletedMeta    []map[string]any
	deletedContent []map[string]any
}

func (f *fakeStore) GetClient() *mongo.Client { return nil }
func (f *fakeStore) Disconnect() error        { return nil }
func (f *fakeStore) Migrate() error           { return nil }
func (f *fakeStore) UpsertMetadata(_ context.Context, _ model.BlocklistMetadata) error {
	return nil
}
func (f *fakeStore) UpsertContent(_ context.Context, _ model.BlocklistContent) error { return nil }
func (f *fakeStore) GetMetadata(_ context.Context, _ map[string]any) ([]model.BlocklistMetadata, error) {
	return f.metadata, nil
}
func (f *fakeStore) GetContent(_ context.Context, _ map[string]any) ([]model.BlocklistContent, error) {
	return nil, nil
}
func (f *fakeStore) Delete(_ context.Context, filter map[string]any) error {
	f.deletedContent = append(f.deletedContent, filter)
	return nil
}
func (f *fakeStore) DeleteMetadata(_ context.Context, filter map[string]any) error {
	f.deletedMeta = append(f.deletedMeta, filter)
	return nil
}

// fakeCache implements cache.Cache, recording blocklist deletions.
type fakeCache struct {
	deleted []string
}

func (f *fakeCache) CreateOrUpdateBlocklist(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (f *fakeCache) DeleteBlocklist(_ context.Context, blocklistId string) error {
	f.deleted = append(f.deleted, blocklistId)
	return nil
}
func (f *fakeCache) Ping(_ context.Context) error { return nil }
func (f *fakeCache) Locker(_ string) *dislock.Locker {
	return nil
}

func metaFor(ids ...string) []model.BlocklistMetadata {
	out := make([]model.BlocklistMetadata, 0, len(ids))
	for _, id := range ids {
		out = append(out, model.BlocklistMetadata{BlocklistID: id})
	}
	return out
}

func newPurgeService(minSources int, stored []model.BlocklistMetadata) (*Service, *fakeStore, *fakeCache) {
	store := &fakeStore{metadata: stored}
	cache := &fakeCache{}
	cfg := config.Config{Updater: &config.UpdaterConfig{MinSources: minSources}}
	return &Service{Cfg: cfg, Store: store, Cache: cache, Metrics: metrics.NoopUpdates{}}, store, cache
}

// specRef: #H4 — an empty source set must never classify the whole database as
// stale; the purge is refused outright.
func TestPurgeStale_RefusesEmptySources(t *testing.T) {
	for _, sources := range [][]model.BlocklistMetadata{nil, {}} {
		s, store, cache := newPurgeService(0, metaFor("blp_a", "blp_b"))

		s.PurgeStale(sources)

		if len(store.deletedMeta) != 0 || len(store.deletedContent) != 0 || len(cache.deleted) != 0 {
			t.Fatalf("PurgeStale(%v) deleted meta=%v content=%v cache=%v, want nothing",
				sources, store.deletedMeta, store.deletedContent, cache.deleted)
		}
	}
}

// specRef: #H4 — a source set below the configured minimum is treated as a
// broken read (partial mount), not as a mass removal.
func TestPurgeStale_RefusesBelowMinSources(t *testing.T) {
	s, store, cache := newPurgeService(3, metaFor("blp_a", "blp_b", "blp_c"))

	s.PurgeStale(metaFor("blp_a", "blp_b"))

	if len(store.deletedMeta) != 0 || len(store.deletedContent) != 0 || len(cache.deleted) != 0 {
		t.Fatalf("PurgeStale below min deleted meta=%v content=%v cache=%v, want nothing",
			store.deletedMeta, store.deletedContent, cache.deleted)
	}
}

// specRef: #H2 — with a healthy source set, IDs absent from sources are still
// removed from metadata, content and cache.
func TestPurgeStale_StillPurgesGenuinelyStale(t *testing.T) {
	s, store, cache := newPurgeService(1, metaFor("blp_keep", "blp_stale"))

	s.PurgeStale(metaFor("blp_keep"))

	if len(store.deletedMeta) != 1 || store.deletedMeta[0]["blocklist_id"] != "blp_stale" {
		t.Fatalf("deleted metadata = %v, want [blp_stale]", store.deletedMeta)
	}
	if len(store.deletedContent) != 1 || store.deletedContent[0]["blocklist_id"] != "blp_stale" {
		t.Fatalf("deleted content = %v, want [blp_stale]", store.deletedContent)
	}
	if len(cache.deleted) != 1 || cache.deleted[0] != "blp_stale" {
		t.Fatalf("deleted cache = %v, want [blp_stale]", cache.deleted)
	}
}

func newReadService(minSources int, dir string) *Service {
	cfg := config.Config{Updater: &config.UpdaterConfig{SourcesDir: dir, MinSources: minSources}}
	return &Service{Cfg: cfg, Metrics: metrics.NoopUpdates{}}
}

func writeSourceFile(t *testing.T, dir string, ids ...string) {
	t.Helper()
	entries := make([]string, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, `{"blocklist_id":"`+id+`","name":"`+id+`","source_url":"https://example.com/`+id+`","syntax":"domains","schedule":"@hourly"}`)
	}
	content := "[" + strings.Join(entries, ",") + "]"
	if err := os.WriteFile(filepath.Join(dir, "sources.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
}

// specRef: #G1a — an existing but empty sources directory is a fatal
// misconfiguration: ReadSources must error rather than return an empty set.
func TestReadSources_EmptyDirErrors(t *testing.T) {
	s := newReadService(0, t.TempDir())

	sources, err := s.ReadSources()
	if err == nil {
		t.Fatalf("ReadSources on empty dir = (%v, nil), want error", sources)
	}
}

// specRef: #G1a — fewer parsed sources than UPDATER_MIN_SOURCES is an error.
func TestReadSources_BelowMinErrors(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "blp_a", "blp_b")
	s := newReadService(3, dir)

	if _, err := s.ReadSources(); err == nil {
		t.Fatal("ReadSources below minimum returned nil error, want error")
	}
}

// specRef: #G1a — a source set meeting the minimum parses normally.
func TestReadSources_MeetsMinimum(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "blp_a", "blp_b")
	s := newReadService(2, dir)

	sources, err := s.ReadSources()
	if err != nil {
		t.Fatalf("ReadSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("ReadSources returned %d sources, want 2", len(sources))
	}
}
