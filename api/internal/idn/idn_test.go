package idn_test

import (
	"testing"

	"github.com/ivpn/dns/api/internal/idn"
)

// TestContainsIDN verifies detection of xn-- labels in domain values.
//
// specRef:"S5"
func TestContainsIDN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		// specRef:"S5" -- case 1: legitimate German IDN
		{name: "german_idn", input: "xn--mller-kva.de", want: true},
		// specRef:"S5" -- case 2: homograph paypal look-alike
		{name: "homograph_paypal", input: "xn--pypal-2ve.com", want: true},
		// specRef:"S5" -- case 3: wildcard with IDN label
		{name: "wildcard_idn", input: "*.xn--pypal-2ve.com", want: true},
		// specRef:"S5" -- case 4: plain ASCII, no IDN
		{name: "plain_ascii", input: "ads.example.com", want: false},
		// specRef:"S5" -- case 5: malformed punycode still flags as IDN
		{name: "malformed_punycode", input: "xn--invalid--punycode-!@#.com", want: true},
		// specRef:"S5" -- case 6: uppercase ACE prefix
		{name: "uppercase_prefix", input: "XN--mller-kva.de", want: true},
		// specRef:"S5" -- case 7: mixed-case ACE prefix
		{name: "mixedcase_prefix", input: "Xn--mller-kva.de", want: true},
		// specRef:"S5" -- case 8: empty string
		{name: "empty", input: "", want: false},
		// specRef:"S5" -- case 9: whitespace only
		{name: "whitespace_only", input: "   ", want: false},
		// specRef:"S5" -- case 10: Japanese IDN, both labels punycode
		{name: "japanese_idn_both_labels", input: "xn--p8j9a0d9c9a.xn--q9jyb4c", want: true},
		// specRef:"S5" -- case 11: single-label, no dots
		{name: "single_label_plain", input: "local", want: false},
		// specRef:"S5" -- case 12: single-label punycode, no TLD
		{name: "single_label_punycode", input: "xn--bnk-mef", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := idn.ContainsIDN(tc.input)
			if got != tc.want {
				t.Errorf("ContainsIDN(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestDecode verifies Unicode decoding of xn-- labels.
//
// specRef:"S5"
func TestDecode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantDecoded string
		wantOK      bool
	}{
		// specRef:"S5" -- case 1: legitimate German IDN
		{name: "german_idn", input: "xn--mller-kva.de", wantDecoded: "müller.de", wantOK: true},
		// specRef:"S5" -- case 2: homograph paypal (Cyrillic U+042F)
		{name: "homograph_paypal", input: "xn--pypal-2ve.com", wantDecoded: "pypalЯ.com", wantOK: true},
		// specRef:"S5" -- case 3: wildcard with IDN label
		{name: "wildcard_idn", input: "*.xn--pypal-2ve.com", wantDecoded: "*.pypalЯ.com", wantOK: true},
		// specRef:"S5" -- case 4: plain ASCII passthrough
		{name: "plain_ascii", input: "ads.example.com", wantDecoded: "ads.example.com", wantOK: true},
		// specRef:"S5" -- case 5: malformed punycode returns (value, false)
		{name: "malformed_punycode", input: "xn--invalid--punycode-!@#.com", wantDecoded: "xn--invalid--punycode-!@#.com", wantOK: false},
		// specRef:"S5" -- case 6: uppercase ACE prefix decodes to same Unicode form
		{name: "uppercase_prefix", input: "XN--mller-kva.de", wantDecoded: "müller.de", wantOK: true},
		// specRef:"S5" -- case 7: mixed-case ACE prefix decodes to same Unicode form
		{name: "mixedcase_prefix", input: "Xn--mller-kva.de", wantDecoded: "müller.de", wantOK: true},
		// specRef:"S5" -- case 8: empty string passthrough
		{name: "empty", input: "", wantDecoded: "", wantOK: true},
		// specRef:"S5" -- case 9: whitespace-only passthrough
		{name: "whitespace_only", input: "   ", wantDecoded: "   ", wantOK: true},
		// specRef:"S5" -- case 10: Japanese IDN, both labels punycode
		{name: "japanese_idn_both_labels", input: "xn--p8j9a0d9c9a.xn--q9jyb4c", wantDecoded: "はじめよう.みんな", wantOK: true},
		// specRef:"S5" -- case 11: single-label plain passthrough
		{name: "single_label_plain", input: "local", wantDecoded: "local", wantOK: true},
		// specRef:"S5" -- case 12: single-label punycode, no TLD
		{name: "single_label_punycode", input: "xn--bnk-mef", wantDecoded: "bڡnk", wantOK: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotDecoded, gotOK := idn.Decode(tc.input)
			if gotOK != tc.wantOK {
				t.Errorf("Decode(%q) ok = %v, want %v", tc.input, gotOK, tc.wantOK)
			}
			if gotDecoded != tc.wantDecoded {
				t.Errorf("Decode(%q) decoded = %q, want %q", tc.input, gotDecoded, tc.wantDecoded)
			}
		})
	}
}
