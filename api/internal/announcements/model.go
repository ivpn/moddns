package announcements

import (
	"fmt"
	"time"
)

// Category classifies what an announcement is about. The webapp uses it to pick
// a section/icon.
type Category string

const (
	CategoryNews        Category = "news"
	CategoryFeature     Category = "feature"
	CategoryMaintenance Category = "maintenance"
	CategoryIncident    Category = "incident"
	CategorySecurity    Category = "security"
	CategoryPolicy      Category = "policy"
)

// Severity controls how prominently the webapp displays an announcement.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

var validCategories = map[Category]struct{}{
	CategoryNews:        {},
	CategoryFeature:     {},
	CategoryMaintenance: {},
	CategoryIncident:    {},
	CategorySecurity:    {},
	CategoryPolicy:      {},
}

var validSeverities = map[Severity]struct{}{
	SeverityInfo:     {},
	SeverityWarning:  {},
	SeverityCritical: {},
}

// Announcement is a single news/announcement entry. Metadata comes from the
// per-record YAML frontmatter; Body is the Markdown that follows the
// frontmatter block (so it is excluded from YAML unmarshalling).
type Announcement struct {
	ID          string     `json:"id" yaml:"id"`
	Category    Category   `json:"category" yaml:"category"`
	Severity    Severity   `json:"severity" yaml:"severity"`
	Title       string     `json:"title" yaml:"title"`
	Body        string     `json:"body" yaml:"-"`
	PublishedAt time.Time  `json:"published_at" yaml:"published_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" yaml:"expires_at"`
	Pinned      bool       `json:"pinned" yaml:"pinned"`
	Link        string     `json:"link,omitempty" yaml:"link"`
}

// Validate checks the required fields and enumerated values. Body is allowed to
// be empty (a title-only announcement is valid).
func (a *Announcement) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("id is required")
	}
	if a.Title == "" {
		return fmt.Errorf("%s: title is required", a.ID)
	}
	if _, ok := validCategories[a.Category]; !ok {
		return fmt.Errorf("%s: unknown category %q", a.ID, a.Category)
	}
	if _, ok := validSeverities[a.Severity]; !ok {
		return fmt.Errorf("%s: unknown severity %q", a.ID, a.Severity)
	}
	if a.PublishedAt.IsZero() {
		return fmt.Errorf("%s: published_at is required", a.ID)
	}
	return nil
}
