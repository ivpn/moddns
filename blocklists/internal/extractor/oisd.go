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
	// OISDTimeFormat is the date format used in OISD blocklists
	OISDTimeFormat = "2006-01-02T15:04:05-0700"
)

var (
	// Pre-compiled regular expressions for better performance
	oisdReLastModified = regexp.MustCompile(`# Last modified: (.+)`)
	oisdReVersion      = regexp.MustCompile(`# Version: (.+)`)
	oisdReEntries      = regexp.MustCompile(`# Entries: (\d+)`)
)

// OISDExtractor implements the Extractor interface for OISD format blocklists
type OISDExtractor struct{}

// NewOISDExtractor creates a new instance of OISDExtractor
func NewOISDExtractor() *OISDExtractor {
	return &OISDExtractor{}
}

// Convert processes the blocklist bytes and returns them unchanged
// as OISD format is already in the desired format
func (e *OISDExtractor) Convert(blocklistBytes []byte) ([]byte, ConversionResult, error) {
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
func (e *OISDExtractor) ExtractMetadata(blocklistBytes []byte) (time.Time, string, int, error) {
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
		if matches := oisdReLastModified.FindStringSubmatch(line); matches != nil {
			var err error
			lastModified, err = time.Parse(OISDTimeFormat, matches[1])
			if err != nil {
				return time.Time{}, "", 0, fmt.Errorf("invalid last modified date format: %w", err)
			}
			foundDate = true
		}

		// Extract version
		if matches := oisdReVersion.FindStringSubmatch(line); matches != nil {
			version = matches[1]
		}

		// Extract number of entries
		if matches := oisdReEntries.FindStringSubmatch(line); matches != nil {
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
		return time.Time{}, "", 0, fmt.Errorf("required metadata not found in blocklist")
	}

	return lastModified, version, numEntries, nil
}

func (e *OISDExtractor) ProcessLine(line string) (string, error) {
	if strings.HasPrefix(line, "#") {
		return "", nil // Skip comment lines
	}
	return line, nil
}
