package filterreasons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The tokens are stored in query logs and read by the API, the frontend and the
// statistics classifier, so they must never change.
//
// specRef: proxy-statistics-behaviour.md #Y12
// specRef: api-endpoint-behaviour.md #J24
func TestTokens_StoredValuesAreStable(t *testing.T) {
	assert.Equal(t, "default_rule", DefaultRule)
	assert.Equal(t, "blocklists", Blocklists)
	assert.Equal(t, "blocklists_subdomains_rule", BlocklistsSubdomains)
	assert.Equal(t, "blocklist: ", BlocklistPrefix)
	assert.Equal(t, "services", Services)
	assert.Equal(t, "service: ", ServicePrefix)
	assert.Equal(t, "custom_rules", CustomRules)
	assert.Equal(t, "rebinding_protection", Rebinding)
	assert.Equal(t, "cname_uncloaking", CnameUncloaking)
	assert.Equal(t, "dnssec_failed", DNSSECFailed)
}
