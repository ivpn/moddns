package announcements

import (
	"testing"
	"time"
)

func ann(id string, published time.Time, expires *time.Time, pinned bool) Announcement {
	return Announcement{
		ID:          id,
		Category:    CategoryNews,
		Severity:    SeverityInfo,
		Title:       id,
		PublishedAt: published,
		ExpiresAt:   expires,
		Pinned:      pinned,
	}
}

func TestVisible_FiltersFutureAndExpired(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)
	expired := now.Add(-time.Hour)

	all := []Announcement{
		ann("published", past, nil, false),
		ann("future", future, nil, false),
		ann("expired", past, &expired, false),
	}

	got := Visible(all, now)
	if len(got) != 1 || got[0].ID != "published" {
		t.Fatalf("expected only 'published', got %+v", got)
	}
}

func TestVisible_SortPinnedThenDateDesc(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	all := []Announcement{
		ann("old", now.Add(-72*time.Hour), nil, false),
		ann("pinned-old", now.Add(-96*time.Hour), nil, true),
		ann("new", now.Add(-24*time.Hour), nil, false),
	}

	got := Visible(all, now)
	want := []string{"pinned-old", "new", "old"}
	if len(got) != len(want) {
		t.Fatalf("expected %d, got %d", len(want), len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("position %d: want %q, got %q", i, id, got[i].ID)
		}
	}
}

func TestVisible_EmptyNeverNil(t *testing.T) {
	got := Visible(nil, time.Now())
	if got == nil {
		t.Fatal("Visible returned nil; expected empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}
