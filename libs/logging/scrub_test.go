package logging

import (
	"strings"
	"testing"
)

// specRef: logging-behaviour.md D1 — SensitiveLogKeys removal
func TestScrubFieldsRemovesSensitiveKeys(t *testing.T) {
	fields := map[string]any{
		"email":           "user@example.com",
		"token":           "tok-123",
		"session_id":      "sess-123",
		"preauth_id":      "pa-123",
		"account_id":      "acc-123",
		"subscription_id": "sub-123",
		"search":          "example.org",
		"key":             "203.0.113.7",
		"computed_hash":   "aGFzaA==",
		"preauth_hash":    "aGFzaA==",
		"client_ip":       "203.0.113.7",
		"ip":              "203.0.113.7",
		"addr":            "203.0.113.7:53",
		"domain":          "example.org",
		"profile_id":      "prof-123",
		"user":            "user-123",
	}

	ScrubFields(fields)

	for key := range fields {
		t.Errorf("Expected key %q to be removed, but it survived", key)
	}
}

// specRef: logging-behaviour.md D1 — non-sensitive fields pass through
func TestScrubFieldsKeepsBenignKeys(t *testing.T) {
	fields := map[string]any{
		"layer":        "profile",
		"proto":        "udp",
		"duration":     42,
		"result_count": 7,
		"category":     "welcome",
	}

	ScrubFields(fields)

	for _, key := range []string{"layer", "proto", "duration", "result_count", "category"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("Expected benign key %q to survive scrubbing", key)
		}
	}
}

// specRef: logging-behaviour.md D1 — key matching is case-insensitive
func TestScrubFieldsCaseInsensitive(t *testing.T) {
	fields := map[string]any{
		"Email":      "user@example.com",
		"SESSION_ID": "sess-123",
	}

	ScrubFields(fields)

	if len(fields) != 0 {
		t.Errorf("Expected case-variant sensitive keys to be removed, got %v", fields)
	}
}

func TestScrubFieldsNilMap(t *testing.T) {
	ScrubFields(nil) // must not panic
}

// specRef: logging-behaviour.md D2 — email redaction in free text
func TestRedactEmails(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		wants []string // substrings expected in output
		bans  []string // substrings that must not appear
	}{
		{
			name:  "plain address",
			in:    "user@example.com",
			wants: []string{"[redacted-email]"},
			bans:  []string{"user@example.com"},
		},
		{
			name:  "embedded in error string",
			in:    `mail: address "john.doe+tag@mail.example.org" rejected by server`,
			wants: []string{"[redacted-email]", "rejected by server"},
			bans:  []string{"john.doe+tag@mail.example.org"},
		},
		{
			name:  "multiple addresses",
			in:    "from a@b.example to c@d.example",
			wants: []string{"from [redacted-email] to [redacted-email]"},
			bans:  []string{"a@b.example", "c@d.example"},
		},
		{
			name:  "no address unchanged",
			in:    "validation failed for input",
			wants: []string{"validation failed for input"},
		},
		{
			name: "empty string",
			in:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := RedactEmails(tc.in)
			for _, w := range tc.wants {
				if !strings.Contains(out, w) {
					t.Errorf("RedactEmails(%q) = %q, expected it to contain %q", tc.in, out, w)
				}
			}
			for _, b := range tc.bans {
				if strings.Contains(out, b) {
					t.Errorf("RedactEmails(%q) = %q, expected %q to be redacted", tc.in, out, b)
				}
			}
		})
	}
}
