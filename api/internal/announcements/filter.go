package announcements

import (
	"sort"
	"time"
)

// Visible returns the announcements that should be shown at time now: those
// already published and not yet expired, sorted pinned-first then by
// published_at descending. The returned slice is never nil.
func Visible(all []Announcement, now time.Time) []Announcement {
	out := make([]Announcement, 0, len(all))
	for _, a := range all {
		if a.PublishedAt.After(now) {
			continue // not published yet
		}
		if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
			continue // expired
		}
		out = append(out, a)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].PublishedAt.After(out[j].PublishedAt)
	})

	return out
}
