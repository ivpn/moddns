package extractor

import "testing"

// specRef: #D15 #D16 — NormalizeDomain canonicalizes case, CRLF, whitespace,
// BOM and trailing dots.
func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"Example.COM":             "example.com",          // case
		"ads.example.net\r":       "ads.example.net",      // CR (CRLF leftover)
		"  spaced.example.org  ":  "spaced.example.org",   // surrounding whitespace
		"trailing.example.com.":   "trailing.example.com", // trailing dot
		"trailing.example.com...": "trailing.example.com", // multiple trailing dots
		"\uFEFFbom.example.com":   "bom.example.com",      // BOM prefix
		"":                        "",
	}
	for in, want := range cases {
		if got := NormalizeDomain(in); got != want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

// specRef: #D17 #D18 — ValidDomain accepts real domains (incl. punycode TLDs,
// digit labels, underscores) and rejects comments, wildcards and injection.
func TestValidDomain(t *testing.T) {
	valid := []string{
		"example.com",
		"0.beer",
		"tencent.xn--io0a7i", // punycode TLD
		"xn--80aaa9acdxhj7e.xn--p1ai",
		"f9e79f670c.000491b06a.com", // digit labels
		"_dmarc.example.com",        // underscore
		"a-b.c-d.co.uk",
	}
	for _, d := range valid {
		if !ValidDomain(d) {
			t.Errorf("ValidDomain(%q) = false, want true", d)
		}
	}

	invalid := []string{
		"",
		"com",               // single label
		"xn--fiqs8s",        // bare punycode TLD, no dot
		"two words.com",     // space
		"*.ads.example.com", // wildcard
		"<html>",            // markup
		"a..b.com",          // empty label
		".leading.com",      // leading dot
		"0.0.0.0",           // IP-like
		"# comment",         // comment
		"café.com",          // raw non-ASCII (must be punycode)
	}
	for _, d := range invalid {
		if ValidDomain(d) {
			t.Errorf("ValidDomain(%q) = true, want false", d)
		}
	}
}
