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
