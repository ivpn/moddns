package extractor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHageziExtractor_Convert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "basic input",
			input: `# Hagezi blocklist
example.com
example.org`,
			want: `# Hagezi blocklist
example.com
example.org`,
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
	}

	extractor := NewHageziExtractor()

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

func TestHageziExtractor_ExtractMetadata(t *testing.T) {
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
			input: `# Last modified: 20 Nov 2023 14:30 UTC
# Version: 2.0.1
# Number of entries: 1000
example.com
example.org`,
			wantLastModified: time.Date(2023, 11, 20, 14, 30, 0, 0, time.UTC),
			wantVersion:      "2.0.1",
			wantNumEntries:   1000,
			wantErr:          false,
		},
		{
			name: "metadata with comments",
			input: `# Title: Hagezi Blocklist
# Last modified: 20 Nov 2023 14:30 UTC
# Description: Test blocklist
# Version: 2.0.1
# Number of entries: 1000
example.com`,
			wantLastModified: time.Date(2023, 11, 20, 14, 30, 0, 0, time.UTC),
			wantVersion:      "2.0.1",
			wantNumEntries:   1000,
			wantErr:          false,
		},
		{
			name: "invalid date format",
			input: `# Last modified: 2023-11-20
# Version: 2.0.1
# Number of entries: 1000`,
			wantLastModified: time.Time{},
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "invalid number of entries",
			input: `# Last modified: 20 Nov 2023 14:30 UTC
# Version: 2.0.1
# Number of entries: invalid`,
			wantLastModified: time.Time{},
			wantVersion:      "",
			wantNumEntries:   0,
			wantErr:          true,
		},
		{
			name: "missing metadata fields",
			input: `# Title: Hagezi Blocklist
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

	extractor := NewHageziExtractor()

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
