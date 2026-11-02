// Package filterreasons holds the filter reason tokens the proxy stores in query
// logs and the API reads back, so writer and reader cannot drift.
package filterreasons

// Stored values: the frontend and the statistics classifier read them too; never change them.
const (
	DefaultRule          = "default_rule"
	Blocklists           = "blocklists"
	BlocklistsSubdomains = "blocklists_subdomains_rule"
	// BlocklistPrefix is followed by the blocklist id.
	BlocklistPrefix = "blocklist: "
	Services        = "services"
	// ServicePrefix is followed by the service id.
	ServicePrefix   = "service: "
	CustomRules     = "custom_rules"
	Rebinding       = "rebinding_protection"
	CnameUncloaking = "cname_uncloaking"
	// DNSSECFailed is appended to a query log's reasons when the recursor reports a
	// validation failure via an Extended DNS Error (RFC 8914).
	DNSSECFailed = "dnssec_failed"
)
