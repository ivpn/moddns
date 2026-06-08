package announcements

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type parseState int

const (
	seekOpen parseState = iota
	inFrontmatter
	inBody
)

// Parse parses the announcements file format: a sequence of records, each a
// `---`-fenced YAML frontmatter block followed by a Markdown body. Any content
// before the first fence (e.g. a header comment) is ignored.
//
// Records are split on frontmatter fences, so a body line that is exactly "---"
// is treated as the start of the next record. Authors must use "***" for any
// in-body horizontal rule.
func Parse(data []byte) ([]Announcement, error) {
	lines := strings.Split(string(data), "\n")

	var (
		out      []Announcement
		state    = seekOpen
		fmBuf    []string
		bodyBuf  []string
		current  Announcement
		recordNo int
	)

	parseFrontmatter := func() error {
		current = Announcement{}
		if err := yaml.Unmarshal([]byte(strings.Join(fmBuf, "\n")), &current); err != nil {
			return fmt.Errorf("announcement #%d: invalid frontmatter: %w", recordNo, err)
		}
		fmBuf = nil
		return nil
	}

	flush := func() error {
		current.Body = strings.TrimSpace(strings.Join(bodyBuf, "\n"))
		if err := current.Validate(); err != nil {
			return fmt.Errorf("announcement #%d: %w", recordNo, err)
		}
		out = append(out, current)
		current = Announcement{}
		bodyBuf = nil
		return nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		isFence := strings.TrimSpace(line) == "---"

		switch state {
		case seekOpen:
			if isFence {
				recordNo++
				fmBuf = nil
				state = inFrontmatter
			}
		case inFrontmatter:
			if isFence {
				if err := parseFrontmatter(); err != nil {
					return nil, err
				}
				bodyBuf = nil
				state = inBody
			} else {
				fmBuf = append(fmBuf, line)
			}
		case inBody:
			if isFence {
				if err := flush(); err != nil {
					return nil, err
				}
				recordNo++
				fmBuf = nil
				state = inFrontmatter
			} else {
				bodyBuf = append(bodyBuf, line)
			}
		}
	}

	switch state {
	case inBody:
		if err := flush(); err != nil {
			return nil, err
		}
	case inFrontmatter:
		return nil, fmt.Errorf("announcement #%d: unterminated frontmatter (missing closing ---)", recordNo)
	}

	seen := make(map[string]struct{}, len(out))
	for _, a := range out {
		if _, ok := seen[a.ID]; ok {
			return nil, fmt.Errorf("duplicate announcement id: %q", a.ID)
		}
		seen[a.ID] = struct{}{}
	}

	return out, nil
}
