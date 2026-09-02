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
	importantModifier = "important"
	regexDelimiter    = "/"
	wildcard          = "*"
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

// Convert transforms AdGuard format rules into a simple domain list. Rules
// whose syntax the extractor does not understand are dropped rather than
// widened into unconditional blocks (fail open), counted per reason in the
// returned stats.
func (e *AdguardExtractor) Convert(blocklistBytes []byte) ([]byte, ConversionStats, error) {
	domains := make([]string, 0)
	disabled := make(map[string]struct{})
	var stats ConversionStats
	scanner := bufio.NewScanner(bytes.NewReader(blocklistBytes))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines
		if isCommentOrEmpty(line) {
			continue
		}

		// Exception rules are the list's built-in allowlist; skipped for now.
		// This also covers @@…$badfilter, which disables the exception itself.
		if strings.HasPrefix(line, exceptionPrefix) {
			stats.SkippedExceptions++
			continue
		}

		// Regex rules can contain '$' in the expression, so they must be
		// recognized before modifier parsing.
		if strings.HasPrefix(line, regexDelimiter) {
			stats.SkippedInvalid++
			continue
		}

		pattern, modifiers := splitModifiers(line)

		// A $badfilter rule disables the rule matching its remaining text
		// instead of adding one
		// (https://adguard-dns.io/kb/general/dns-filtering-syntax/#badfilter).
		if hasModifier(modifiers, badfilterModifier) {
			stats.SkippedBadfilter++
			if domain := processRule(line); domain != "" {
				disabled[domain] = struct{}{}
			}
			continue
		}

		// $important only strengthens a block at DNS level, so the block is
		// kept. Every other modifier conditions, scopes or rewrites the rule
		// ($dnstype, $client, $dnsrewrite, …) — keeping the bare pattern
		// would over-block, so the rule is dropped instead.
		if hasUnsupportedModifier(modifiers) {
			stats.SkippedModifiers++
			continue
		}

		if strings.Contains(pattern, wildcard) {
			stats.SkippedWildcards++
			continue
		}

		// A single-pipe pattern ending with a dot (`|load.gtm.`) matches
		// hostnames *starting with* the token; extracting it would emit the
		// literal token as a bogus domain.
		if isPrefixRule(pattern) {
			stats.SkippedPrefixes++
			continue
		}

		// Process the line to extract the domain
		if domain := processRule(line); domain != "" {
			domains = append(domains, domain)
		} else {
			stats.SkippedInvalid++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, ConversionStats{}, fmt.Errorf("error scanning blocklist: %w", err)
	}

	if len(disabled) > 0 {
		kept := domains[:0]
		for _, d := range domains {
			if _, ok := disabled[d]; ok {
				stats.SkippedBadfilter++
				continue
			}
			kept = append(kept, d)
		}
		domains = kept
	}

	return []byte(strings.Join(domains, "\n")), stats, nil
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

// splitModifiers splits a rule at the first '$' into its pattern and its
// comma-separated modifier list (nil when the rule has none).
func splitModifiers(rule string) (string, []string) {
	pattern, modifiers, found := strings.Cut(rule, modifierSeparator)
	if !found {
		return pattern, nil
	}
	return pattern, strings.Split(modifiers, ",")
}

func hasModifier(modifiers []string, name string) bool {
	for _, m := range modifiers {
		if m == name {
			return true
		}
	}
	return false
}

// hasUnsupportedModifier reports whether the rule carries any modifier other
// than the bare $important (the only one that leaves a DNS-level block a
// block).
func hasUnsupportedModifier(modifiers []string) bool {
	for _, m := range modifiers {
		if m != importantModifier {
			return true
		}
	}
	return false
}

// isPrefixRule reports whether the pattern is a single-pipe hostname-prefix
// match: anchored to the name start and ending with a dot, e.g. `|load.gtm.`.
func isPrefixRule(pattern string) bool {
	return strings.HasPrefix(pattern, "|") &&
		!strings.HasPrefix(pattern, "||") &&
		strings.HasSuffix(pattern, ".")
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
