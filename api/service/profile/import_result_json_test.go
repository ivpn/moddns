package profile

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportResult_ListFieldsRenderAsEmptyArray guards the API empty-list
// contract for the import response: createdProfileIds/createdProfileNames/warnings
// must serialize as [] when empty, never null (a zero-value ImportResult has nil
// slices).
func TestImportResult_ListFieldsRenderAsEmptyArray(t *testing.T) {
	raw, err := json.Marshal(ImportResult{})
	require.NoError(t, err)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))

	for _, k := range []string{"createdProfileIds", "createdProfileNames", "warnings"} {
		v, present := m[k]
		assert.Truef(t, present, "list key %q must be present; got %s", k, raw)
		if present {
			assert.JSONEqf(t, "[]", string(v), "list key %q must be [] (not null); got %s", k, string(v))
		}
	}
}
