// Package idn provides utilities for detecting and decoding internationalized
// domain names (IDN) encoded in ASCII-compatible Punycode form (RFC 3492 / RFC 5891).
//
// Use ContainsIDN to detect whether any label in a domain name carries the
// "xn--" punycode prefix. Use Decode to obtain the Unicode rendering of such
// a domain for display in security-relevant warnings (the import path uses
// this to make IDN homograph rules visible to the user).
//
// The match is case-insensitive ("Xn--", "XN--" both count). Wildcard labels
// are tolerated -- "*.xn--bnk-mef.com" is flagged just like "xn--bnk-mef.com".
package idn

import (
	"strings"

	"golang.org/x/net/idna"
)

const acePrefix = "xn--"

// ContainsIDN reports whether value contains at least one dot-separated
// label that begins with the punycode ACE prefix "xn--" (case-insensitive).
// Empty or all-whitespace input returns false.
func ContainsIDN(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if strings.HasPrefix(strings.ToLower(label), acePrefix) {
			return true
		}
	}
	return false
}

// Decode returns the Unicode form of value. ok is false if value contains
// at least one xn-- label that fails to decode (malformed punycode).
// If value contains no xn-- labels, Decode returns (value, true) -- i.e.,
// plain ASCII passes through unchanged.
//
// Decode is intended for display only. Do not store the decoded form;
// re-encoding may not round-trip exactly for non-canonical inputs.
func Decode(value string) (decoded string, ok bool) {
	labels := strings.Split(value, ".")
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if !strings.HasPrefix(strings.ToLower(label), acePrefix) {
			// Not a punycode label -- pass through unchanged (covers wildcards,
			// plain ASCII labels, and empty labels from leading/trailing dots).
			out = append(out, label)
			continue
		}
		// Punycode.ToUnicode is case-sensitive; normalize the ACE prefix to
		// lowercase before decoding. The ASCII portion of a punycode label
		// (after the last hyphen delimiter) is already case-insensitive by
		// RFC 3492, so lowercasing the whole label is safe.
		unicode, err := idna.Punycode.ToUnicode(strings.ToLower(label))
		if err != nil {
			return value, false
		}
		out = append(out, unicode)
	}
	return strings.Join(out, "."), true
}
