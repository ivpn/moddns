package validator

import (
	"strings"
	"testing"
)

// specRef: #R2 — password policy (length + character-class composition).
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		// Valid: 12-64 chars with upper, lower, digit and a special char.
		{"minimal valid", "Abcdefghij1!", true},
		{"max length valid", "Aa1!" + strings.Repeat("a", 60), true}, // 64 chars
		{"space counts as special (OWASP: spaces allowed)", "Abcdefghij1 ", true},
		{"underscore special", "Abcdefghij1_", true},
		{"unicode symbol special", "Abcdefghij1€", true},

		// OWASP ASVS: any non-alphanumeric counts. These were previously rejected
		// because they were absent from the hand-listed special-char set.
		{"apostrophe special", "Abcdefghij1'", true},
		{"plus special", "Abcdefghij1+", true},
		{"slash special", "Abcdefghij1/", true},
		{"equals special", "Abcdefghij1=", true},
		{"backslash special", "Abcdefghij1\\", true},
		{"backtick special", "Abcdefghij1`", true},
		{"tilde special", "Abcdefghij1~", true},

		// Invalid: too short / too long.
		{"too short", "Abc1!", false},
		{"too long", "Aa1!" + strings.Repeat("a", 61), false}, // 65 chars
		{"empty", "", false},

		// Invalid: missing a required character class.
		{"no special (all alphanumeric)", "Abcdefghij12", false},
		{"no uppercase", "abcdefghij1!", false},
		{"no lowercase", "ABCDEFGHIJ1!", false},
		{"no digit", "Abcdefghijk!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePassword(tt.password); got != tt.want {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

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
