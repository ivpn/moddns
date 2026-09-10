package extractor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStevenBlackExtractor_Convert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "basic hosts file",
			input: `# Title: StevenBlack/hosts
# Date: 25 November 2023 20:53:07 (UTC)
# Number of unique domains: 3

127.0.0.1 localhost
127.0.0.1 localhost.localdomain
0.0.0.0 example.com
0.0.0.0 malicious.example.org
127.0.0.1 local.test
::1 localhost
255.255.255.255 broadcasthost
0.0.0.0 0.0.0.0`,
			want: `example.com
malicious.example.org`,
		},
		{
			name: "hosts file with comments and empty lines",
			input: `# This is a comment
# Another comment

0.0.0.0 ads.example.com
# More comments
0.0.0.0 tracker.example.net

# End of file`,
			want: `ads.example.com
tracker.example.net`,
		},
		{
			name: "hosts file with invalid domains",
			input: `0.0.0.0 valid-domain.com
0.0.0.0 invalid_domain_no_tld
0.0.0.0 another-valid.example.org
0.0.0.0 .invalid.com
127.0.0.1 localhost`,
			want: `valid-domain.com
another-valid.example.org`,
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name: "only comments and localhost",
			input: `# Comment only file
127.0.0.1 localhost
127.0.0.1 localhost.localdomain`,
			want: ``,
		},
		{
			name: "comprehensive test with all omitted entries",
			input: `# Test file with all types of entries that should be omitted
127.0.0.1 localhost
127.0.0.1 localhost.localdomain
127.0.0.1 local
255.255.255.255 broadcasthost
::1 localhost
::1 ip6-localhost
::1 ip6-loopback
fe80::1%lo0 localhost
ff00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
0.0.0.0 0.0.0.0
0.0.0.0 valid-domain.com
0.0.0.0 another-valid.example.org`,
			want: `valid-domain.com
another-valid.example.org`,
		},
	}

	extractor := NewStevenBlackExtractor()

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

func TestStevenBlackExtractor_ExtractMetadata(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantTime       string
		wantVersion    string
		wantNumEntries int
		wantErr        bool
	}{
		{
			name: "complete metadata",
			input: `# Title: StevenBlack/hosts
# Description: Consolidating and extending hosts files
# Date: 25 November 2023 20:53:07 (UTC)
# Number of unique domains: 174653

127.0.0.1 localhost
0.0.0.0 example.com
0.0.0.0 malicious.example.org`,
			wantTime:       "2023-11-25T20:53:07Z",
			wantVersion:    "",
			wantNumEntries: 174653,
			wantErr:        false,
		},
		{
			name: "alternative time format without seconds",
			input: `# Title: Custom Hosts List
# Date: 15 December 2023 14:30 (UTC)
# Number of unique domains: 1000

0.0.0.0 test.com`,
			wantTime:       "2023-12-15T14:30:00Z",
			wantVersion:    "",
			wantNumEntries: 1000,
			wantErr:        false,
		},
		{
			name: "comma-separated number of domains",
			input: `# Title: StevenBlack/hosts
# Date: 25 November 2023 20:53:07 (UTC)
# Number of unique domains: 230,923

0.0.0.0 example.com`,
			wantTime:       "2023-11-25T20:53:07Z",
			wantVersion:    "",
			wantNumEntries: 230923,
			wantErr:        false,
		},
		{
			name: "large comma-separated number with multiple commas",
			input: `# Title: Large Hosts List
# Date: 25 November 2023 20:53:07 (UTC)
# Number of unique domains: 1,234,567

0.0.0.0 example.com`,
			wantTime:       "2023-11-25T20:53:07Z",
			wantVersion:    "",
			wantNumEntries: 1234567,
			wantErr:        false,
		},
		{
			name: "missing date",
			input: `# Title: Test List
# Number of unique domains: 100
0.0.0.0 example.com`,
			wantErr: true,
		},
		{
			name: "missing number of domains",
			input: `# Title: Test List
# Date: 25 November 2023 20:53:07 (UTC)
0.0.0.0 example.com`,
			wantErr: true,
		},
		{
			name: "invalid date format",
			input: `# Title: Test List
# Date: invalid date format
# Number of unique domains: 100
0.0.0.0 example.com`,
			wantErr: true,
		},
		{
			name: "invalid number of domains",
			input: `# Title: Test List
# Date: 25 November 2023 20:53:07 (UTC)
# Number of unique domains: not_a_number
0.0.0.0 example.com`,
			wantErr: true,
		},
	}

	extractor := NewStevenBlackExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotVersion, gotNumEntries, err := extractor.ExtractMetadata([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.wantTime != "" {
				expectedTime, _ := time.Parse(time.RFC3339, tt.wantTime)
				assert.Equal(t, expectedTime.UTC(), gotTime.UTC())
			}

			assert.Equal(t, tt.wantVersion, gotVersion)
			assert.Equal(t, tt.wantNumEntries, gotNumEntries)
		})
	}
}

func TestStevenBlackExtractor_ProcessLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid hosts entry with 0.0.0.0",
			input: "0.0.0.0 example.com",
			want:  "example.com",
		},
		{
			name:  "non-0.0.0.0 entries should be skipped (127.0.0.1)",
			input: "127.0.0.1 test.example.org",
			want:  "",
		},
		{
			name:  "IPv6 entries should be skipped",
			input: "::1 localhost",
			want:  "",
		},
		{
			name:  "broadcast entries should be skipped",
			input: "255.255.255.255 broadcasthost",
			want:  "",
		},
		{
			name:  "special 0.0.0.0 entry should be skipped",
			input: "0.0.0.0 0.0.0.0",
			want:  "",
		},
		{
			name:  "localhost entries should be skipped (127.0.0.1)",
			input: "127.0.0.1 localhost",
			want:  "",
		},
		{
			name:  "localhost.localdomain entries should be skipped (127.0.0.1)",
			input: "127.0.0.1 localhost.localdomain",
			want:  "",
		},
		{
			name:  "IPv6 with special characters should be skipped",
			input: "fe80::1%lo0 localhost",
			want:  "",
		},
		{
			name:  "comment line should be skipped",
			input: "# This is a comment",
			want:  "",
		},
		{
			name:  "empty line should be skipped",
			input: "",
			want:  "",
		},
		{
			name:  "invalid domain format",
			input: "0.0.0.0 invalid_domain",
			want:  "",
		},
		{
			name:  "line with extra whitespace",
			input: "  0.0.0.0   example.com  ",
			want:  "example.com",
		},
	}

	extractor := NewStevenBlackExtractor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.ProcessLine(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewStevenBlackExtractor(t *testing.T) {
	extractor := NewStevenBlackExtractor()
	assert.NotNil(t, extractor)
	assert.IsType(t, &StevenBlackExtractor{}, extractor)
}
