package model

import (
	"testing"

	intvldtr "github.com/ivpn/dns/api/internal/validator"
	"github.com/stretchr/testify/assert"
)

// NewCustomRuleSyntax classifies IPv4, FQDN, wildcard, and ASN inputs, and rejects
// malformed values with ErrInvalidCustomRuleSyntax.
func TestNewCustomRuleSyntax(t *testing.T) {
	vldtr, err := intvldtr.NewAPIValidator()
	if err != nil {
		t.Fatalf("Error creating validator: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantSyntax  CustomRuleSyntax
		expectedErr error
	}{
		{
			name:        "ValidIPv4Address",
			input:       "192.168.1.1",
			wantSyntax:  SYNTAX_IPV4,
			expectedErr: nil,
		},
		{
			name:        "InvalidIPv4Address",
			input:       "256.256.256.256",
			wantSyntax:  SYNTAX_UNKNOWN,
			expectedErr: ErrInvalidCustomRuleSyntax,
		},
		{
			name:        "ValidFQDN",
			input:       "google.com",
			wantSyntax:  SYNTAX_FQDN,
			expectedErr: nil,
		},
		{
			name:        "Valid FQDN with wildcard",
			input:       "*.google.com",
			wantSyntax:  SYNTAX_FQDN_WILDCARD,
			expectedErr: nil,
		},
		{
			name:        "Valid FQDN with wildcard 2",
			input:       "*ads.google.com",
			wantSyntax:  SYNTAX_FQDN_WILDCARD,
			expectedErr: nil,
		},
		{
			name:        "Valid ASN with prefix",
			input:       "AS15169",
			wantSyntax:  SYNTAX_ASN,
			expectedErr: nil,
		},
		{
			name:        "Valid ASN without prefix",
			input:       "15169",
			wantSyntax:  SYNTAX_ASN,
			expectedErr: nil,
		},
		{
			name:        "Invalid ASN (zero)",
			input:       "AS0",
			wantSyntax:  SYNTAX_UNKNOWN,
			expectedErr: ErrInvalidCustomRuleSyntax,
		},
		{
			name:        "EmptyInput",
			input:       "",
			wantSyntax:  SYNTAX_UNKNOWN,
			expectedErr: ErrInvalidCustomRuleSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSyntax, err := NewCustomRuleSyntax(vldtr.Validator, tt.input)

			assert.Equal(t, err, tt.expectedErr)
			assert.Equal(t, tt.wantSyntax, gotSyntax)
		})
	}
}

// names is a helper to pluck the ordered group names out of a list for assertions.
func names(list []CustomRuleGroup) []string {
	out := make([]string, len(list))
	for i, g := range list {
		out[i] = g.Name
	}
	return out
}

// TestCustomRuleGroups_Reorder exercises the display-order rewrite: exact reorder,
// unknown names ignored, omitted groups appended in place, comment preservation,
// per-list isolation, and no-op on an empty list.
// tableRef: I8
func TestCustomRuleGroups_Reorder(t *testing.T) {
	t.Run("reorders to match and preserves comments", func(t *testing.T) {
		g := CustomRuleGroups{Block: []CustomRuleGroup{
			{Name: "Ads", Comment: "a"}, {Name: "Social", Comment: "s"}, {Name: "Work", Comment: "w"},
		}}
		g.Reorder(ACTION_BLOCK, []string{"Work", "Ads", "Social"})
		assert.Equal(t, []string{"Work", "Ads", "Social"}, names(g.Block))
		assert.Equal(t, "w", g.Block[0].Comment, "comment must travel with the group")
	})

	t.Run("ignores unknown names", func(t *testing.T) {
		g := CustomRuleGroups{Block: []CustomRuleGroup{{Name: "Ads"}, {Name: "Work"}}}
		g.Reorder(ACTION_BLOCK, []string{"Work", "Ghost", "Ads"})
		assert.Equal(t, []string{"Work", "Ads"}, names(g.Block))
	})

	t.Run("appends omitted groups in their existing order", func(t *testing.T) {
		g := CustomRuleGroups{Block: []CustomRuleGroup{
			{Name: "Ads"}, {Name: "Social"}, {Name: "Work"},
		}}
		// Client only names one group; the rest keep their relative order at the end.
		g.Reorder(ACTION_BLOCK, []string{"Work"})
		assert.Equal(t, []string{"Work", "Ads", "Social"}, names(g.Block))
	})

	t.Run("ignores duplicate names in the order list", func(t *testing.T) {
		g := CustomRuleGroups{Block: []CustomRuleGroup{{Name: "Ads"}, {Name: "Work"}}}
		g.Reorder(ACTION_BLOCK, []string{"Work", "Work", "Ads"})
		assert.Equal(t, []string{"Work", "Ads"}, names(g.Block))
	})

	t.Run("is per-list: reordering block leaves allow untouched", func(t *testing.T) {
		g := CustomRuleGroups{
			Block: []CustomRuleGroup{{Name: "Ads"}, {Name: "Work"}},
			Allow: []CustomRuleGroup{{Name: "Ads"}, {Name: "Work"}},
		}
		g.Reorder(ACTION_BLOCK, []string{"Work", "Ads"})
		assert.Equal(t, []string{"Work", "Ads"}, names(g.Block))
		assert.Equal(t, []string{"Ads", "Work"}, names(g.Allow), "allow list must be unchanged")
	})

	t.Run("no-op on empty list", func(t *testing.T) {
		g := CustomRuleGroups{}
		g.Reorder(ACTION_BLOCK, []string{"Ghost"})
		assert.Nil(t, g.Block)
	})
}
