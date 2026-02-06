package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewExtractor(t *testing.T) {
	tests := []struct {
		name        string
		blocklistID string
		wantType    string
		wantErr     bool
	}{
		{
			name:        "steven_black extractor",
			blocklistID: "steven_black_ads",
			wantType:    "*extractor.StevenBlackExtractor",
			wantErr:     false,
		},
		{
			name:        "steven_black with suffix",
			blocklistID: "steven_black_malware_gambling",
			wantType:    "*extractor.StevenBlackExtractor",
			wantErr:     false,
		},
		{
			name:        "hagezi extractor",
			blocklistID: "hagezi_pro",
			wantType:    "*extractor.HageziExtractor",
			wantErr:     false,
		},
		{
			name:        "oisd extractor",
			blocklistID: "oisd_big",
			wantType:    "*extractor.OISDExtractor",
			wantErr:     false,
		},
		{
			name:        "adguard extractor",
			blocklistID: "adguard_base",
			wantType:    "*extractor.AdguardExtractor",
			wantErr:     false,
		},
		{
			name:        "blp extractor",
			blocklistID: "blp_gambling",
			wantType:    "*extractor.DomainsExtractor",
			wantErr:     false,
		},
		{
			name:        "ut1 extractor",
			blocklistID: "ut1_dating",
			wantType:    "*extractor.DomainsExtractor",
			wantErr:     false,
		},
		{
			name:        "shadowwhisperer extractor",
			blocklistID: "shadowwhisperer_crypto",
			wantType:    "*extractor.DomainsExtractor",
			wantErr:     false,
		},
		{
			name:        "unknown extractor",
			blocklistID: "unknown_type",
			wantType:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExtractor(tt.blocklistID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tt.wantType, getTypeName(got))
		})
	}
}

func getTypeName(v interface{}) string {
	if v == nil {
		return ""
	}
	return getFullTypeName(v)
}

func getFullTypeName(v interface{}) string {
	switch v.(type) {
	case *StevenBlackExtractor:
		return "*extractor.StevenBlackExtractor"
	case *HageziExtractor:
		return "*extractor.HageziExtractor"
	case *OISDExtractor:
		return "*extractor.OISDExtractor"
	case *AdguardExtractor:
		return "*extractor.AdguardExtractor"
	case *DomainsExtractor:
		return "*extractor.DomainsExtractor"
	default:
		return "unknown"
	}
}
