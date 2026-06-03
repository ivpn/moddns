package extractor

import (
	"regexp"
	"strings"
)

// bom is the UTF-8 byte order mark, which can appear at the start of the first
// line of a downloaded list and must be stripped before parsing.
const bom = "\uFEFF"

// maxDomainLen is the maximum length of a DNS name (RFC 1035).
const maxDomainLen = 253

// validDomainRegex matches a normalized (lowercased) domain. Labels allow
// letters, digits, hyphen and underscore; the final label (TLD) is either
// alphabetic or a punycode label (xn--…), so internationalized TLDs such as
// xn--fiqs8s (.中国) or xn--p1ai (.рф) are accepted rather than silently
// dropped. At least one dot is required, which rejects bare single labels,
// IPs and comment/garbage lines.
var validDomainRegex = regexp.MustCompile(`^([a-z0-9_-]+\.)+([a-z]{2,}|xn--[a-z0-9-]+)$`)

// NormalizeDomain canonicalizes a domain token so equivalent names map to a
// single cache entry and lookups match: it strips a leading UTF-8 BOM,
// surrounding whitespace (incl. a CR left by CRLF line endings), and any
// trailing dot(s), then lowercases. Returns "" when nothing remains.
func NormalizeDomain(s string) string {
	s = strings.TrimPrefix(s, bom)
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ".")
	return strings.ToLower(s)
}

// ValidDomain reports whether s (already normalized via NormalizeDomain) is a
// syntactically valid domain. It rejects whitespace, control characters,
// wildcard/injection syntax and over-long names, while accepting digit-only
// labels, underscores and punycode TLDs.
func ValidDomain(s string) bool {
	return len(s) <= maxDomainLen && validDomainRegex.MatchString(s)
}
