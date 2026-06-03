package extractor

import (
	"bufio"
	"bytes"
	"net"
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

// Convert normalizes a plain domain list to one candidate domain per line. It
// is hosts-tolerant: some "domain" lists (e.g. blocklistproject/fakenews) are
// actually in hosts format (`0.0.0.0 domain`), so a leading IP field is
// stripped. Comments and blank lines are dropped; the shared NormalizeDomain +
// ValidDomain gate downstream does the final cleaning/validation.
func (e *DomainsExtractor) Convert(blocklistBytes []byte) ([]byte, error) {
	if len(blocklistBytes) == 0 {
		return []byte{}, nil
	}

	out := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))
	for scanner.Scan() {
		if domain := stripHostsIP(scanner.Text()); domain != "" {
			out = append(out, domain)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return []byte(strings.Join(out, "\n")), nil
}

// stripHostsIP returns the domain candidate from a plain-list or hosts-format
// line, or "" for comments and blank lines. For a `<ip> <domain>` line (first
// field parses as an IP) it returns the domain field; otherwise the trimmed
// line.
func stripHostsIP(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
		return ""
	}
	if fields := strings.Fields(trimmed); len(fields) >= 2 && net.ParseIP(fields[0]) != nil {
		return fields[1]
	}
	return trimmed
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

// ProcessLine skips comment/blank lines and returns the domain candidate,
// stripping a leading IP for hosts-format lines (see stripHostsIP).
func (e *DomainsExtractor) ProcessLine(line string) (string, error) {
	return stripHostsIP(line), nil
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
