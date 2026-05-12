package validator

import (
	"testing"
)

func Test_wildcardFQDNValidation(t *testing.T) {
	// Create a validator instance
	apiValidator, err := NewAPIValidator()
	if err != nil {
		t.Fatalf("Error creating APIValidator: %v", err)
	}

	// Create a test struct
	type testStruct struct {
		Value string `validate:"fqdn_wildcard"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Non-wildcard cases (should fail as they should be handled by other validators)
		{"regular IPv4", "192.168.1.1", true},
		{"regular IPv6", "2001:db8::1", true},
		{"regular FQDN", "example.com", true},

		// IPv4 wildcard cases
		{"IPv4 with wildcards 1", "192.168.*.*", true}, // TODO: support this case
		{"IPv4 with wildcards 2", "*.168.1.*", true},   // TODO: support this case
		{"IPv4 with invalid wildcard", "192.*.1.%", true},
		{"IPv4 with invalid format", "192.*.1", true},

		// IPv6 wildcard cases
		{"IPv6 with wildcards 1", "2001:*:*:*:*:*:*:1", true}, // TODO: support this case
		{"IPv6 with wildcards 2", "*:*:*:*:*:*:*:*", true},    // TODO: support this case
		{"IPv6 with wildcards compressed", "2001:*:1", true},  // TODO: support this case
		{"IPv6 with invalid wildcard", "2001:%::1", true},

		// FQDN wildcard cases
		{"FQDN with wildcard 1", "*.example.com", false},
		{"FQDN with wildcard 2", "*.sub.example.com", false},
		{"FQDN with wildcard 3", "*ads.example.com", false},
		{"FQDN with wildcard 4", "ads*.example.com", false},
		{"FQDN with wildcard 5", "ads-*-eu.example.com", false},
		{"FQDN with wildcard 6", "sub.*.example.com", true},
		{"FQDN with invalid wildcard", "%.example.com", true},
		{"FQDN with invalid format", "*.example", false}, // TODO: not sure what result should be
		{"Suffix wildcard matches", "ads.*", false},
		{"Suffix wildcard multi-label", "sub.ads.*", false},
		{"Suffix wildcard invalid extra label", "ads.*.com", true},
		{"Contains wildcard matches", "*ads*", false},
		{"Contains wildcard with dots", "*ads.example*", false},
		{"Contains wildcard missing middle", "**ads*", true},

		// Degenerate wildcard cases (match-everything footguns)
		{"Bare wildcard rejected", "*", true},
		{"Double wildcard rejected", "**", true},
		{"Trailing-dot wildcard rejected", "*.", true},
		{"Leading-dot wildcard rejected", ".*", true},
		{"Dot-only rejected", ".", true},

		// TLD-level wildcards are accepted (structurally identical to *.io etc.)
		{"TLD wildcard com", "*.com", false},
		{"TLD wildcard io", "*.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := testStruct{Value: tt.value}
			err := apiValidator.Validator.Struct(ts)

			if tt.wantErr && err == nil {
				t.Errorf("wildcardFQDNValidation() for value %v should return error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("wildcardFQDNValidation() for value %v should not return error, got %v", tt.value, err)
			}
		})
	}
}

func Test_safeNameValidation(t *testing.T) {
	apiValidator, err := NewAPIValidator()
	if err != nil {
		t.Fatalf("Error creating APIValidator: %v", err)
	}

	type testStruct struct {
		Value string `validate:"safe_name"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		// Accepted: international display names.
		{"plain ASCII", "Work", false},
		{"ASCII with spaces", "Home Network", false},
		{"latin diacritics", "Café", false},
		{"cyrillic", "Работа", false},
		{"CJK", "我的家", false},
		{"emoji", "🏠 Home", false},
		{"punctuation", "Kids' Devices (2026)", false},

		// Rejected: empty / whitespace-only.
		{"empty", "", true},
		{"ascii spaces only", "   ", true},
		{"unicode whitespace only", "\u00a0\u2003", true},

		// Rejected: Cc control characters.
		{"newline", "Work\nHome", true},
		{"tab", "Work\tHome", true},
		{"ansi escape", "Work\x1b[31mRed", true},
		{"null byte", "Work\x00", true},

		// Rejected: Cf format characters (bidi, zero-width).
		{"RLO bidi override", "Work\u202eHome", true},
		{"LRO bidi override", "\u202dSpoofed", true},
		{"zero-width space", "Work\u200bHome", true},
		{"zero-width joiner", "Work\u200dHome", true},
		{"BOM", "\ufeffWork", true},

		// Rejected: Cs surrogate (invalid Unicode in a string).
		// We can't construct a valid UTF-8 string containing a lone surrogate
		// in Go source, so this case is covered by the rune-walk loop
		// implicitly — skipped here.

		// Rejected: Co private-use.
		{"private use area", "Work\ue000Home", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := testStruct{Value: tt.value}
			err := apiValidator.Validator.Struct(ts)
			if tt.wantErr && err == nil {
				t.Errorf("safeNameValidation(%q) should return error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("safeNameValidation(%q) should not return error, got %v", tt.value, err)
			}
		})
	}
}

func Test_NormalizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trims ASCII spaces", "  Work  ", "Work"},
		{"trims unicode whitespace", " Work ", "Work"},
		// "Café" composed (U+00E9) vs decomposed (e + U+0301) — both must normalize to NFC.
		{"NFC decomposed -> composed", "Cafe\u0301", "Caf\u00e9"},
		{"NFC composed stays composed", "Caf\u00e9", "Caf\u00e9"},
		{"empty stays empty", "", ""},
		{"whitespace-only collapses to empty", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeName(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
