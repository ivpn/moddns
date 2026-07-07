package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseListFields_RenderAsEmptyArray is the guard test for the API list
// contract: every customer-facing response struct must serialize its list fields
// as an empty JSON array ([]) when empty — never `null`, never omitted. If a new
// list field is added without a MarshalJSON normalizing it (or someone re-adds
// json omitempty / removes the marshaler), the relevant case here fails.
//
// When you add a response struct with list fields, add it to this table.
func TestResponseListFields_RenderAsEmptyArray(t *testing.T) {
	cases := []struct {
		name string
		val  any
		keys []string // JSON keys that must be present and equal to []
	}{
		{"Privacy", Privacy{}, []string{"blocklists", "services"}},
		{"ProfileSettings", ProfileSettings{}, []string{"custom_rules"}},
		// NOTE: CustomRuleGroups (block/allow) is intentionally omitted here. It is a
		// value struct that renders as {} (never null), and it is reused by the export
		// contract, so forcing block/allow to [] leaks into the export file. Giving it
		// the []-treatment is deferred to the response-DTO refactor (moddns-shadow).
		{"Blocklist", Blocklist{}, []string{"tags", "intensity"}},
		{"QueryLog", QueryLog{}, []string{"reasons"}},
		{"Account", Account{}, []string{"profiles", "auth_methods"}},
		{"TOTPBackup", TOTPBackup{}, []string{"backup_codes"}},
		{"MfaData", MfaData{}, []string{"methods"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.val)
			require.NoError(t, err)

			var m map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &m))

			for _, k := range tc.keys {
				v, present := m[k]
				assert.Truef(t, present, "%s: list key %q must be present (not omitted); got %s", tc.name, k, raw)
				if present {
					assert.JSONEqf(t, "[]", string(v), "%s: list key %q must be [] (not null); got %s", tc.name, k, string(v))
				}
			}
		})
	}
}
