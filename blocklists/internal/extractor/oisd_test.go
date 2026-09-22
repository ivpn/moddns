package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOISDExtractor_Convert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "basic input",
			input: `# OISD Blocklist
example.com
example.org`,
			want: `# OISD Blocklist
example.com
example.org`,
		},
		{
			name: "with metadata",
			input: `# Last modified: 2023-11-20T15:04:05-0700
# Version: 2.0.1
# Entries: 1000
example.com`,
			want: `# Last modified: 2023-11-20T15:04:05-0700
# Version: 2.0.1
# Entries: 1000
example.com`,
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
	}

	extractor := NewOISDExtractor()

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

func TestOISDExtractor_ExtractMetadata(t *testing.T) {
	const layout = "2006-01-02 15:04:05 -0700 MST"
	tests := []struct {
		name             string
		input            string
		wantLastModified []string
		wantVersion      string
		wantNumEntries   int
		wantErr          bool
	}{
		{
			name: "valid metadata",
			input: `# Last modified: 2025-04-01T11:22:19+0000
					# Version: 2.0.1
					# Entries: 1000
					example.com`,
			wantLastModified: []string{"2025-04-01 11:22:19 +0000 UTC", "2025-04-01 11:22:19 +0000 +0000"},
			wantVersion:      "2.0.1",
			wantNumEntries:   1000,
			wantErr:          false,
		},
		{
			name: "metadata with comments",
			input: `# Title: OISD Blocklist
					# Last modified: 2025-04-01T11:22:19+0000
					# Description: Test blocklist
					# Version: 2.0.1
					# Entries: 1000
					example.com`,
			wantLastModified: []string{"2025-04-01 11:22:19 +0000 UTC", "2025-04-01 11:22:19 +0000 +0000"},
			wantVersion:      "2.0.1",
			wantNumEntries:   1000,
			wantErr:          false,
		},
		{
			name: "invalid date format",
			input: `# Last modified: 2023-11-20
					# Version: 2.0.1
					# Entries: 1000`,
			wantLastModified: nil,
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "invalid number of entries",
			input: `# Last modified: 2023-11-20T15:04:05-0700
					# Version: 2.0.1
					# Entries: invalid`,
			wantLastModified: nil,
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "missing metadata fields",
			input: `# Title: OISD Blocklist
					example.com
					example.org`,
			wantLastModified: nil,
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name:             "empty input",
			input:            "",
			wantLastModified: nil,
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "partial metadata",
			input: `# Last modified: 2023-11-20T15:04:05-0700
					# Version: 2.0.1`,
			wantLastModified: nil,
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
	}

	extractor := NewOISDExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastModified, version, numEntries, err := extractor.ExtractMetadata([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.wantLastModified != nil {
				assert.Contains(t, tt.wantLastModified, lastModified.Format(layout))
			}
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantNumEntries, numEntries)
		})
	}
}
