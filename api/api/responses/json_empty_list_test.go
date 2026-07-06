package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchResponseListFields_RenderAsEmptyArray guards the API list contract for
// the custom-rules batch response: created/skipped must serialize as [] when
// empty, never null.
func TestBatchResponseListFields_RenderAsEmptyArray(t *testing.T) {
	raw, err := json.Marshal(CreateProfileCustomRulesBatchResponse{})
	require.NoError(t, err)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))

	for _, k := range []string{"created", "skipped"} {
		v, present := m[k]
		assert.Truef(t, present, "list key %q must be present; got %s", k, raw)
		if present {
			assert.JSONEqf(t, "[]", string(v), "list key %q must be [] (not null); got %s", k, string(v))
		}
	}
}
