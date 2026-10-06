package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The strings are stored in query logs and read by the frontend and the statistics
// classifier, so they must never change.
//
// specRef: proxy-statistics-behaviour.md #Y12
func TestFilterReasons_StoredValuesAreStable(t *testing.T) {
	assert.Equal(t, "default_rule", FilterReasonDefaultRule)
	assert.Equal(t, "blocklists", FilterReasonBlocklists)
	assert.Equal(t, "blocklists_subdomains_rule", FilterReasonBlocklistsSubdomains)
	assert.Equal(t, "blocklist: ", FilterReasonBlocklistPrefix)
	assert.Equal(t, "services", FilterReasonServices)
	assert.Equal(t, "service: ", FilterReasonServicePrefix)
	assert.Equal(t, "custom_rules", FilterReasonCustomRules)
	assert.Equal(t, "rebinding_protection", FilterReasonRebinding)
	assert.Equal(t, "cname_uncloaking", FilterReasonCnameUncloaking)
	assert.Equal(t, "dnssec_failed", FilterReasonDNSSECFailed)
}
