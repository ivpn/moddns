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
	// HageziTimeFormat is the date format used in Hagezi blocklists
	HageziTimeFormat = "02 Jan 2006 15:04 MST"
)

var (
	// Pre-compiled regular expressions for better performance
	reLastModified = regexp.MustCompile(`# Last modified: (.+)`)
	reVersion      = regexp.MustCompile(`# Version: (.+)`)
	reNumEntries   = regexp.MustCompile(`# Number of entries: (\d+)`)
)

// HageziExtractor implements the Extractor interface for Hagezi format blocklists
type HageziExtractor struct{}

// NewHageziExtractor creates a new instance of HageziExtractor
func NewHageziExtractor() *HageziExtractor {
	return &HageziExtractor{}
}

// Convert processes the blocklist bytes and returns them unchanged
// as Hagezi format is already in the desired format
func (e *HageziExtractor) Convert(blocklistBytes []byte) ([]byte, ConversionResult, error) {
	if len(blocklistBytes) == 0 {
		return []byte{}, ConversionResult{}, nil
	}
	return blocklistBytes, ConversionResult{}, nil
}

// ExtractMetadata extracts metadata from the blocklist including:
// - Last modified time
// - Version
// - Number of entries
//
// Returns an error if required metadata is missing or invalid
func (e *HageziExtractor) ExtractMetadata(blocklistBytes []byte) (time.Time, string, int, error) {
	var (
		lastModified time.Time
		version      string
		numEntries   int
		foundDate    bool
		foundEntries bool
	)

	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))
	for scanner.Scan() {
		line := scanner.Text()

		// Extract last modified date
		if matches := reLastModified.FindStringSubmatch(line); matches != nil {
			var err error
			lastModified, err = time.Parse(HageziTimeFormat, matches[1])
			if err != nil {
				return time.Time{}, "", 0, fmt.Errorf("invalid last modified date format: %w", err)
			}
			foundDate = true
		}

		// Extract version
		if matches := reVersion.FindStringSubmatch(line); matches != nil {
			version = matches[1]
		}

		// Extract number of entries
		if matches := reNumEntries.FindStringSubmatch(line); matches != nil {
			var err error
			numEntries, err = strconv.Atoi(matches[1])
			if err != nil {
				return time.Time{}, "", 0, fmt.Errorf("invalid number of entries: %w", err)
			}
			foundEntries = true
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, "", 0, fmt.Errorf("error scanning blocklist: %w", err)
	}

	if !foundDate || !foundEntries {
		return time.Time{}, "", 0, fmt.Errorf("missing required metadata fields")
	}

	return lastModified, version, numEntries, nil
}
func (e *HageziExtractor) ProcessLine(line string) (string, error) {
	if strings.HasPrefix(line, "#") {
		return "", nil // Skip comment lines
	}
	return line, nil
}
