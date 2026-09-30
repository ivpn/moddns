package model

// Filter reason tokens as stored in FilterResult.Reasons and query logs. The
// values are read by the frontend and the statistics classifier; never change them.
const (
	FilterReasonDefaultRule          = "default_rule"
	FilterReasonBlocklists           = "blocklists"
	FilterReasonBlocklistsSubdomains = "blocklists_subdomains_rule"
	// FilterReasonBlocklistPrefix is followed by the blocklist id.
	FilterReasonBlocklistPrefix = "blocklist: "
	FilterReasonServices        = "services"
	// FilterReasonServicePrefix is followed by the service id.
	FilterReasonServicePrefix   = "service: "
	FilterReasonCustomRules     = "custom_rules"
	FilterReasonRebinding       = "rebinding_protection"
	FilterReasonCnameUncloaking = "cname_uncloaking"
	// FilterReasonDNSSECFailed is appended to a query log's reasons when the recursor
	// reports a validation failure via an Extended DNS Error (RFC 8914).
	FilterReasonDNSSECFailed = "dnssec_failed"
)
