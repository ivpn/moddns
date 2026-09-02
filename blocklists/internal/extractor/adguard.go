package extractor

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// Time format used in AdGuard blocklists
	adGuardTimeFormat = "2006-01-02T15:04:05.000Z"

	// Comment prefixes
	commentPrefixExclamation = "!"
	commentPrefixHash        = "#"

	// Rule prefixes and special characters
	exceptionPrefix   = "@@"
	modifierSeparator = "$"
	badfilterModifier = "badfilter"
)

var (
	// Pre-compiled regex for better performance
	lastModifiedRegex = regexp.MustCompile(`! Last modified: (.+)`)
)

// AdguardExtractor implements the Extractor interface for AdGuard format blocklists
type AdguardExtractor struct{}

// NewAdguardExtractor creates a new instance of AdguardExtractor
func NewAdguardExtractor() *AdguardExtractor {
	return &AdguardExtractor{}
}

// Convert transforms AdGuard format rules into a simple domain list
func (e *AdguardExtractor) Convert(blocklistBytes []byte) ([]byte, error) {
	domains := make([]string, 0)
	disabled := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines
		if isCommentOrEmpty(line) {
			continue
		}

		// A $badfilter rule disables the rule matching its remaining text
		// instead of adding one
		// (https://adguard-dns.io/kb/general/dns-filtering-syntax/#badfilter).
		// On an exception rule it disables the exception, which is skipped
		// anyway, so only block-rule badfilters collect targets.
		if hasBadfilter(line) {
			if !strings.HasPrefix(line, exceptionPrefix) {
				if domain := processRule(line); domain != "" {
					disabled[domain] = struct{}{}
				}
			}
			continue
		}

		// Process the line to extract the domain
		if domain := processRule(line); domain != "" {
			domains = append(domains, domain)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning blocklist: %w", err)
	}

	if len(disabled) > 0 {
		kept := domains[:0]
		for _, d := range domains {
			if _, ok := disabled[d]; !ok {
				kept = append(kept, d)
			}
		}
		domains = kept
	}

	return []byte(strings.Join(domains, "\n")), nil
}

// ExtractMetadata extracts metadata from the blocklist including last modified time,
// version (if available), and number of entries
func (e *AdguardExtractor) ExtractMetadata(blocklistBytes []byte) (time.Time, string, int, error) {
	var lastModified time.Time
	var numEntries int
	foundLastModified := false

	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))
	for scanner.Scan() {
		line := scanner.Text()

		if !foundLastModified {
			if matches := lastModifiedRegex.FindStringSubmatch(line); matches != nil {
				var err error
				lastModified, err = time.Parse(adGuardTimeFormat, matches[1])
				if err != nil {
					return time.Time{}, "", 0, fmt.Errorf("invalid last modified date format: %w", err)
				}
				foundLastModified = true
			}
		}

		if !isCommentOrEmpty(line) {
			numEntries++
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, "", 0, fmt.Errorf("error scanning blocklist: %w", err)
	}

	if !foundLastModified {
		return time.Time{}, "", 0, fmt.Errorf("last modified date not found in blocklist")
	}

	return lastModified, "", numEntries, nil
}

// processRule processes an AdGuard rule and extracts the domain
func processRule(rule string) string {
	// Skip exception rules
	if strings.HasPrefix(rule, exceptionPrefix) {
		return ""
	}

	// Remove modifiers and special characters
	rule = strings.Split(rule, modifierSeparator)[0]
	rule = strings.ReplaceAll(rule, "^", "")
	rule = strings.ReplaceAll(rule, "|", "")
	rule = strings.TrimSpace(rule)

	// Normalize and validate using the shared domain validator (accepts
	// punycode TLDs and lowercases).
	if d := NormalizeDomain(rule); ValidDomain(d) {
		return d
	}

	return ""
}

// hasBadfilter reports whether the rule carries the $badfilter modifier
// (alone or comma-separated with others).
func hasBadfilter(rule string) bool {
	_, modifiers, found := strings.Cut(rule, modifierSeparator)
	if !found {
		return false
	}
	for _, m := range strings.Split(modifiers, ",") {
		if m == badfilterModifier {
			return true
		}
	}
	return false
}

// isCommentOrEmpty checks if a line is either empty or a comment
func isCommentOrEmpty(line string) bool {
	return line == "" ||
		strings.HasPrefix(line, commentPrefixExclamation) ||
		strings.HasPrefix(line, commentPrefixHash)
}

func (e *AdguardExtractor) ProcessLine(line string) (string, error) {
	return line, nil
}
