package extractor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdguardExtractor_Convert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "basic domains",
			input: `example.com
example.org`,
			want: "example.com\nexample.org",
		},
		{
			name: "with comments",
			input: `! Comment line
# Another comment
example.com
! More comments
example.org`,
			want: "example.com\nexample.org",
		},
		{
			name: "with empty lines",
			input: `

example.com

example.org

`,
			want: "example.com\nexample.org",
		},
		{
			name: "with exception rules",
			input: `example.com
@@exception.com
example.org`,
			want: "example.com\nexample.org",
		},
		{
			// specRef: #D2 — $important keeps the block; #D2d — any other
			// modifier drops the rule instead of widening it into an
			// unconditional block.
			name: "with modifiers",
			input: `example.com$important
example.org^$third-party
||example.net^`,
			want: "example.com\nexample.net",
		},
		{
			// specRef: #D2d — a conditioned rule ($dnstype scopes it to one
			// query type) must not compile to an unconditional block.
			name:  "dnstype modifier drops the rule",
			input: `||example.com^$dnstype=AAAA`,
			want:  "",
		},
		{
			// specRef: #D2d — fail open on modifiers that do not exist yet.
			name:  "unknown future modifier drops the rule",
			input: `||example.com^$frobnicate=1`,
			want:  "",
		},
		{
			// specRef: #D2e — wildcard patterns are unsupported.
			name: "wildcard pattern skipped",
			input: `||ads*.example.com^
||example.net^`,
			want: "example.net",
		},
		{
			// specRef: #D2f — a hostname-prefix rule must not leak the literal
			// token as a domain.
			name: "hostname prefix rule skipped",
			input: `|load.gtm.
||example.net^`,
			want: "example.net",
		},
		{
			// specRef: #D5 — regex rules fail validation and are dropped.
			name:  "regex rule skipped",
			input: `/^ad[0-9]+\.example\.com$/`,
			want:  "",
		},
		{
			name: "invalid domains",
			input: `not-a-domain
example.com
also-not-a-domain`,
			want: "example.com",
		},
		{
			// specRef: #D2a — a $badfilter rule disables a rule; it is never
			// emitted as a block itself.
			name:  "badfilter rule alone",
			input: `||example.com^$badfilter`,
			want:  "",
		},
		{
			// specRef: #D2b — a $badfilter rule removes its target from the
			// output, regardless of line order.
			name: "badfilter removes matching block rule",
			input: `||example.com^
||tracker.org^
||example.com^$badfilter`,
			want: "tracker.org",
		},
		{
			// specRef: #D2b — order-independent: badfilter before the block.
			name: "badfilter before matching block rule",
			input: `||example.com^$badfilter
||example.com^
||tracker.org^`,
			want: "tracker.org",
		},
		{
			// specRef: #D2b — bare-domain badfilter form.
			name: "bare domain badfilter",
			input: `wykop.pl$badfilter
||tracker.org^`,
			want: "tracker.org",
		},
		{
			// specRef: #D2b — badfilter among comma-separated modifiers.
			name: "badfilter combined with another modifier",
			input: `||example.com^$important,badfilter
||example.com^`,
			want: "",
		},
		{
			// specRef: #D2c — badfilter on an exception disables the
			// exception, not the block rule for the same domain.
			name: "badfilter on exception does not disable block",
			input: `@@||example.com^$badfilter
||example.com^`,
			want: "example.com",
		},
		{
			// specRef: #D2d — a modifier value containing "badfilter" is not
			// $badfilter; the rule is dropped as an unsupported modifier, not
			// treated as a disable directive.
			name:  "modifier value containing badfilter text",
			input: `||example.com^$dnstype=badfilter`,
			want:  "",
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
	}

	extractor := NewAdguardExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := extractor.Convert([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// specRef: #D2a #D2d #D2e #D2f — per-reason skip counts reported alongside the
// converted output, published as blocklists_rules_skipped{source,reason}.
func TestAdguardExtractor_ConvertStats(t *testing.T) {
	input := `! comment
||blocked.com^
@@||allowed.com^
||disabled.com^
||disabled.com^$badfilter
||typed.com^$dnstype=AAAA
||ads*.example.com^
|load.gtm.
/^regex$/
not a domain line
||important.com^$important`

	got, stats, err := NewAdguardExtractor().Convert([]byte(input))
	assert.NoError(t, err)
	assert.Equal(t, "blocked.com\nimportant.com", string(got))
	assert.Equal(t, ConversionStats{
		SkippedExceptions: 1,
		SkippedBadfilter:  2, // the $badfilter rule and the target it disabled
		SkippedModifiers:  1,
		SkippedWildcards:  1,
		SkippedPrefixes:   1,
		SkippedInvalid:    2, // regex rule + non-domain line
	}, stats)
}

func TestAdguardExtractor_ExtractMetadata(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantLastModified time.Time
		wantVersion      string
		wantNumEntries   int
		wantErr          bool
	}{
		{
			name: "valid metadata",
			input: `! Title: Test List
! Last modified: 2023-11-20T15:04:05.000Z
! Version: Not applicable for AdGuard
example.com
example.org`,
			wantLastModified: time.Date(2023, 11, 20, 15, 4, 5, 0, time.UTC),
			wantVersion:      "",
			wantNumEntries:   2,
			wantErr:          false,
		},
		{
			name: "with comments and empty lines",
			input: `! Title: Test List
! Last modified: 2023-11-20T15:04:05.000Z
! Description: Test description

# Comment
example.com
! Another comment
example.org

`,
			wantLastModified: time.Date(2023, 11, 20, 15, 4, 5, 0, time.UTC),
			wantVersion:      "",
			wantNumEntries:   2,
			wantErr:          false,
		},
		{
			name: "invalid date format",
			input: `! Title: Test List
! Last modified: 2023-11-20
example.com`,
			wantLastModified: time.Time{},
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "missing last modified",
			input: `! Title: Test List
example.com
example.org`,
			wantLastModified: time.Time{},
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name:             "empty input",
			input:            "",
			wantLastModified: time.Time{},
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
	}

	extractor := NewAdguardExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastModified, version, numEntries, err := extractor.ExtractMetadata([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantLastModified, lastModified)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantNumEntries, numEntries)
		})
	}
}

func TestProcessRule(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string
	}{
		{
			name: "simple domain",
			rule: "example.com",
			want: "example.com",
		},
		{
			name: "domain with modifier",
			rule: "example.com$important",
			want: "example.com",
		},
		{
			name: "domain with special chars",
			rule: "||example.com^",
			want: "example.com",
		},
		{
			name: "exception rule",
			rule: "@@example.com",
			want: "",
		},
		{
			name: "invalid domain",
			rule: "not-a-domain",
			want: "",
		},
		{
			name: "empty rule",
			rule: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processRule(tt.rule)
			assert.Equal(t, tt.want, got)
		})
	}
}
