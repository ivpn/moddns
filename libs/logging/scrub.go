package logging

import (
	"regexp"
	"strings"
)

// SensitiveLogKeys lists structured-log field keys whose values must never
// leave the host in telemetry (e.g. Sentry events). On-host log output is not
// affected. Matching is exact on the lowercased key; libs/telemetry applies
// this list at delivery time.
var SensitiveLogKeys = map[string]struct{}{
	"email":           {},
	"token":           {},
	"session_id":      {},
	"preauth_id":      {},
	"account_id":      {},
	"subscription_id": {},
	"search":          {},
	"key":             {},
	"computed_hash":   {},
	"preauth_hash":    {},
	"client_ip":       {},
	"ip":              {},
	"addr":            {},
	"domain":          {},
	"profile_id":      {},
	"user":            {},
}

// emailPattern matches email addresses embedded in free text, such as
// provider error strings ("550 <addr> rejected").
var emailPattern = regexp.MustCompile(`[\w.+-]+@[\w.-]+\.\w+`)

// ScrubFields deletes entries listed in SensitiveLogKeys from fields in
// place. Safe to call with a nil map.
func ScrubFields(fields map[string]any) {
	for key := range fields {
		if _, sensitive := SensitiveLogKeys[strings.ToLower(key)]; sensitive {
			delete(fields, key)
		}
	}
}

// RedactEmails replaces every email address embedded in s with
// "[redacted-email]".
func RedactEmails(s string) string {
	if s == "" {
		return s
	}
	return emailPattern.ReplaceAllString(s, "[redacted-email]")
}
