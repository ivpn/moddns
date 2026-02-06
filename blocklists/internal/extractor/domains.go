package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// Common header patterns across plain domain list formats
	domainsReLastModified = regexp.MustCompile(`(?i)^[#!]\s*Last modified:\s*(.+)`)
	domainsReEntries      = regexp.MustCompile(`(?i)^[#!]\s*(?:Number of )?Entries:\s*([\d,]+)`)
)

// DomainsExtractor implements the Extractor interface for plain domain-per-line
// blocklists (Block List Project, UT1, ShadowWhisperer, etc.)
type DomainsExtractor struct{}

// NewDomainsExtractor creates a new instance of DomainsExtractor
func NewDomainsExtractor() *DomainsExtractor {
	return &DomainsExtractor{}
}

// Convert returns the blocklist bytes unchanged as they are already in domain format
func (e *DomainsExtractor) Convert(blocklistBytes []byte) ([]byte, error) {
	if len(blocklistBytes) == 0 {
		return []byte{}, nil
	}
	return blocklistBytes, nil
}

// ExtractMetadata extracts metadata from the blocklist. Unlike strict extractors,
// this gracefully falls back when headers are missing:
//   - Last modified: tries multiple date formats, falls back to time.Now()
//   - Version: always empty (these lists don't have versions)
//   - Number of entries: parses header if present, otherwise counts non-comment lines
func (e *DomainsExtractor) ExtractMetadata(blocklistBytes []byte) (time.Time, string, int, error) {
	var (
		lastModified time.Time
		numEntries   int
		foundDate    bool
		foundEntries bool
		domainCount  int
	)

	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := domainsReLastModified.FindStringSubmatch(line); matches != nil && !foundDate {
			if parsed, ok := parseFlexibleDate(strings.TrimSpace(matches[1])); ok {
				lastModified = parsed
				foundDate = true
			}
		}

		if matches := domainsReEntries.FindStringSubmatch(line); matches != nil && !foundEntries {
			cleaned := strings.ReplaceAll(matches[1], ",", "")
			if n, err := strconv.Atoi(cleaned); err == nil {
				numEntries = n
				foundEntries = true
			}
		}

		// Count non-empty, non-comment lines for fallback entry count
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "!") {
			domainCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, "", 0, err
	}

	if !foundDate {
		lastModified = time.Now().UTC().Truncate(time.Second)
	}

	if !foundEntries {
		numEntries = domainCount
	}

	return lastModified, "", numEntries, nil
}

// ProcessLine skips comment lines and empty lines, returning the trimmed domain
func (e *DomainsExtractor) ProcessLine(line string) (string, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
		return "", nil
	}
	return trimmed, nil
}

// parseFlexibleDate tries multiple date formats commonly found in blocklist headers
func parseFlexibleDate(s string) (time.Time, bool) {
	formats := []string{
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"02 Jan 2006 15:04 MST",
		"02 Jan 2006",
		"Jan 02, 2006",
		time.RFC3339,
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
