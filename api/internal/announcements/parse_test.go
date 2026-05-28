package announcements

import (
	"strings"
	"testing"
	"time"
)

const sampleFile = `# Announcements — header comment, ignored by the parser

---
id: a1
category: maintenance
severity: warning
title: First
published_at: 2026-05-28
expires_at: 2026-06-01
pinned: true
link: https://example.com/status
---
Body **one**.

- item one
- item two

---
id: a2
category: news
severity: info
title: Second
published_at: 2026-04-01
---
Body two.
`

func TestParse_MultipleRecords(t *testing.T) {
	got, err := Parse([]byte(sampleFile))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 announcements, got %d", len(got))
	}

	a1 := got[0]
	if a1.ID != "a1" || a1.Category != CategoryMaintenance || a1.Severity != SeverityWarning {
		t.Errorf("a1 metadata wrong: %+v", a1)
	}
	if !a1.Pinned {
		t.Errorf("a1 should be pinned")
	}
	if a1.Link != "https://example.com/status" {
		t.Errorf("a1 link wrong: %q", a1.Link)
	}
	if a1.PublishedAt.Year() != 2026 || a1.PublishedAt.Month() != time.May || a1.PublishedAt.Day() != 28 {
		t.Errorf("a1 published_at wrong: %v", a1.PublishedAt)
	}
	if a1.ExpiresAt == nil || a1.ExpiresAt.Day() != 1 || a1.ExpiresAt.Month() != time.June {
		t.Errorf("a1 expires_at wrong: %v", a1.ExpiresAt)
	}
	if !strings.Contains(a1.Body, "Body **one**.") || !strings.Contains(a1.Body, "- item one") {
		t.Errorf("a1 body markdown not preserved: %q", a1.Body)
	}

	a2 := got[1]
	if a2.ID != "a2" || a2.Category != CategoryNews || a2.ExpiresAt != nil || a2.Pinned {
		t.Errorf("a2 metadata wrong: %+v", a2)
	}
	if a2.Body != "Body two." {
		t.Errorf("a2 body wrong: %q", a2.Body)
	}
}

func TestParse_Empty(t *testing.T) {
	got, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 announcements, got %d", len(got))
	}
}

func TestParse_Errors(t *testing.T) {
	cases := map[string]string{
		"missing title": `---
id: x
category: news
severity: info
published_at: 2026-01-01
---
body`,
		"unknown category": `---
id: x
category: bogus
severity: info
title: T
published_at: 2026-01-01
---
body`,
		"unknown severity": `---
id: x
category: news
severity: loud
title: T
published_at: 2026-01-01
---
body`,
		"missing published_at": `---
id: x
category: news
severity: info
title: T
---
body`,
		"malformed yaml": `---
id: x
category: [unterminated
---
body`,
		"unterminated frontmatter": `---
id: x
category: news
`,
		"duplicate id": `---
id: dup
category: news
severity: info
title: One
published_at: 2026-01-01
---
a
---
id: dup
category: news
severity: info
title: Two
published_at: 2026-01-02
---
b`,
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(in)); err == nil {
				t.Fatalf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestParse_TitleOnlyBodyAllowed(t *testing.T) {
	in := `---
id: x
category: news
severity: info
title: Heads up
published_at: 2026-01-01
---
`
	got, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Body != "" {
		t.Fatalf("expected one announcement with empty body, got %+v", got)
	}
}
