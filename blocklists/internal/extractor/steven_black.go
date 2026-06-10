package extractor

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// StevenBlackTimeFormat is the date format used in Steven Black hosts files
	// Example: "25 November 2023 20:53:07 (UTC)"
	StevenBlackTimeFormat = "02 January 2006 15:04:05 (MST)"

	// Alternative time format without seconds
	StevenBlackTimeFormatAlt = "02 January 2006 15:04 (MST)"
)

var (
	// Pre-compiled regular expressions for better performance
	stevenBlackDateRegex    = regexp.MustCompile(`# Date: (.+)`)
	stevenBlackDomainsRegex = regexp.MustCompile(`# Number of unique domains: ([\d,]+)`)

	// Hosts file entry regex - matches any IP address followed by domain/hostname
	hostsEntryRegex = regexp.MustCompile(`^(\S+)\s+(.+)$`)
)

// StevenBlackExtractor implements the Extractor interface for Steven Black hosts file format
type StevenBlackExtractor struct{}

// NewStevenBlackExtractor creates a new instance of StevenBlackExtractor
func NewStevenBlackExtractor() *StevenBlackExtractor {
	return &StevenBlackExtractor{}
}

// Convert transforms Steven Black hosts file format into a simple domain list
func (e *StevenBlackExtractor) Convert(blocklistBytes []byte) ([]byte, error) {
	if len(blocklistBytes) == 0 {
		return []byte{}, nil
	}

	domains := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Process hosts file entries
		if matches := hostsEntryRegex.FindStringSubmatch(line); matches != nil {
			ip := matches[1]
			domain := strings.TrimSpace(matches[2])

			// Only process 0.0.0.0 entries for blocklist
			if ip == "0.0.0.0" {
				// Skip the special 0.0.0.0 0.0.0.0 entry
				if domain == "0.0.0.0" {
					continue
				}

				// Normalize and validate using the shared domain validator.
				if nd := NormalizeDomain(domain); ValidDomain(nd) {
					domains = append(domains, nd)
				}
			}
			// All other IP addresses (127.0.0.1, IPv6, broadcast, etc.) are skipped
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning hosts file: %w", err)
	}

	return []byte(strings.Join(domains, "\n")), nil
}

// ExtractMetadata extracts metadata from the Steven Black hosts file including:
// - Last modified time from Date field
// - Number of entries from Number of unique domains field
//
// Returns an error if required metadata is missing or invalid
func (e *StevenBlackExtractor) ExtractMetadata(blocklistBytes []byte) (time.Time, string, int, error) {
	var (
		lastModified time.Time
		numEntries   int
		foundDate    bool
		foundEntries bool
	)

	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))
	for scanner.Scan() {
		line := scanner.Text()

		// Extract date
		if matches := stevenBlackDateRegex.FindStringSubmatch(line); matches != nil {
			dateStr := matches[1]
			var err error

			// Try primary time format first
			lastModified, err = time.Parse(StevenBlackTimeFormat, dateStr)
			if err != nil {
				// Try alternative time format without seconds
				lastModified, err = time.Parse(StevenBlackTimeFormatAlt, dateStr)
				if err != nil {
					return time.Time{}, "", 0, fmt.Errorf("invalid date format: %w", err)
				}
			}
			foundDate = true
		}

		// Extract number of unique domains
		if matches := stevenBlackDomainsRegex.FindStringSubmatch(line); matches != nil {
			var err error
			// Remove commas from the number string before parsing
			numStr := strings.ReplaceAll(matches[1], ",", "")
			numEntries, err = strconv.Atoi(numStr)
			if err != nil {
				return time.Time{}, "", 0, fmt.Errorf("invalid number of domains: %w", err)
			}
			foundEntries = true
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, "", 0, fmt.Errorf("error scanning hosts file: %w", err)
	}

	if !foundDate || !foundEntries {
		return time.Time{}, "", 0, fmt.Errorf("missing required metadata fields (date: %v, entries: %v)", foundDate, foundEntries)
	}

	return lastModified, "", numEntries, nil
}

// ProcessLine processes a single line from the hosts file
func (e *StevenBlackExtractor) ProcessLine(line string) (string, error) {
	line = strings.TrimSpace(line)

	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil
	}

	// Process hosts file entries
	if matches := hostsEntryRegex.FindStringSubmatch(line); matches != nil {
		ip := matches[1]
		domain := strings.TrimSpace(matches[2])

		// Only process 0.0.0.0 entries for blocklist
		if ip == "0.0.0.0" {
			// Skip the special 0.0.0.0 0.0.0.0 entry
			if domain == "0.0.0.0" {
				return "", nil
			}

			// Normalize and validate using the shared domain validator.
			if nd := NormalizeDomain(domain); ValidDomain(nd) {
				return nd, nil
			}
		}
		// All other IP addresses (127.0.0.1, IPv6, broadcast, etc.) are skipped
	}

	return "", nil
}
