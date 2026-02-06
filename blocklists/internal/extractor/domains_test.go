package extractor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDomainsExtractor_Convert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "basic input",
			input: "example.com\nexample.org",
			want:  "example.com\nexample.org",
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
	}

	extractor := NewDomainsExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.Convert([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestDomainsExtractor_ExtractMetadata(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantLastModified time.Time
		wantVersion      string
		wantNumEntries   int
		wantErr          bool
		checkDateApprox  bool // if true, check date is recent instead of exact
	}{
		{
			name: "block list project format with ISO date and comma-separated count",
			input: `# Title: BlockListProject Gambling
# Last modified: 2024-01-15
# Entries: 12,345
example.com
example.org`,
			wantLastModified: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantVersion:      "",
			wantNumEntries:   12345,
			wantErr:          false,
		},
		{
			name: "block list project format with full timestamp",
			input: `# Last modified: 2024-03-20 10:30:00 UTC
# Number of Entries: 5000
gambling-site.com
poker-online.net`,
			wantLastModified: time.Date(2024, 3, 20, 10, 30, 0, 0, time.UTC),
			wantVersion:      "",
			wantNumEntries:   5000,
			wantErr:          false,
		},
		{
			name: "no headers at all - fallback behavior for UT1/ShadowWhisperer",
			input: `dating-site.com
match-making.net
love-app.org`,
			wantVersion:     "",
			wantNumEntries:  3,
			wantErr:         false,
			checkDateApprox: true,
		},
		{
			name:            "empty input",
			input:           "",
			wantVersion:     "",
			wantNumEntries:  0,
			wantErr:         false,
			checkDateApprox: true,
		},
		{
			name: "comment-only input",
			input: `# This is a blocklist
# Another comment
! AdBlock-style comment`,
			wantVersion:     "",
			wantNumEntries:  0,
			wantErr:         false,
			checkDateApprox: true,
		},
		{
			name: "mixed whitespace and comments",
			input: `# Header comment
  example.com
	tabs.org
! adblock comment

normal.net`,
			wantVersion:     "",
			wantNumEntries:  3,
			wantErr:         false,
			checkDateApprox: true,
		},
		{
			name: "entries header without date",
			input: `# Entries: 2
one.com
two.com`,
			wantVersion:     "",
			wantNumEntries:  2,
			wantErr:         false,
			checkDateApprox: true,
		},
		{
			name: "date header without entries count",
			input: `# Last modified: 2024-06-01
domain1.com
domain2.com
domain3.com`,
			wantLastModified: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			wantVersion:      "",
			wantNumEntries:   3,
			wantErr:          false,
		},
	}

	extractor := NewDomainsExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastModified, version, numEntries, err := extractor.ExtractMetadata([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantNumEntries, numEntries)

			if tt.checkDateApprox {
				// Date should be recent (within last minute)
				assert.WithinDuration(t, time.Now().UTC(), lastModified, time.Minute)
			} else {
				assert.Equal(t, tt.wantLastModified, lastModified)
			}
		})
	}
}

func TestDomainsExtractor_ProcessLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    string
		wantErr bool
	}{
		{
			name: "normal domain",
			line: "example.com",
			want: "example.com",
		},
		{
			name: "domain with leading whitespace",
			line: "  example.com  ",
			want: "example.com",
		},
		{
			name: "domain with tabs",
			line: "\texample.com\t",
			want: "example.com",
		},
		{
			name: "hash comment",
			line: "# This is a comment",
			want: "",
		},
		{
			name: "exclamation comment",
			line: "! AdBlock comment",
			want: "",
		},
		{
			name: "empty line",
			line: "",
			want: "",
		},
		{
			name: "whitespace-only line",
			line: "   ",
			want: "",
		},
	}

	extractor := NewDomainsExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.ProcessLine(tt.line)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
		ok    bool
	}{
		{
			name:  "ISO date",
			input: "2024-01-15",
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			ok:    true,
		},
		{
			name:  "ISO datetime with timezone",
			input: "2024-01-15 10:30:00 UTC",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			ok:    true,
		},
		{
			name:  "RFC3339",
			input: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			ok:    true,
		},
		{
			name:  "Hagezi-like format",
			input: "15 Jan 2024 10:30 UTC",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			ok:    true,
		},
		{
			name:  "unparseable",
			input: "not a date",
			want:  time.Time{},
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseFlexibleDate(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
