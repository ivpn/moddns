package model

// Values of the logs endpoint's status filter. Blocked and processed match
// QueryLog.Status as written by the proxy; unanswered is a pseudo-status
// resolved by outcome (query-log-outcomes-behaviour.md C5). The proxy also
// writes status "unavailable" (O11); those rows are reached through
// unanswered, never selected by status.
const (
	QueryLogStatusAll        = "all"
	QueryLogStatusBlocked    = "blocked"
	QueryLogStatusProcessed  = "processed"
	QueryLogStatusUnanswered = "unanswered"
)

// Resolution-outcome tokens written by the proxy into QueryLog.Outcome.
// Decision table: docs/specs/query-log-outcomes-behaviour.md (rows O1–O11).
const (
	OutcomeResolved          = "resolved"
	OutcomeNoData            = "nodata"
	OutcomeNXDomain          = "nxdomain"
	OutcomeBlocked           = "blocked"
	OutcomeServfailDNSSEC    = "servfail_dnssec"
	OutcomeServfailUpstream  = "servfail_upstream"
	OutcomeTimeout           = "timeout"
	OutcomeNetworkError      = "network_error"
	OutcomeRefused           = "refused"
	OutcomeFilterUnavailable = "filter_unavailable"
)

// UnansweredOutcomes is the "No answer" class for the logs status filter,
// the same set the collapsed-row label uses (query-log-outcomes-behaviour.md
// C3, C5): the service could not answer. Deliberate verdicts — blocked, and
// DNSSEC validation failures — are not failures and stay out. Every stored
// row carries an outcome: the field predates the longest retention window.
var UnansweredOutcomes = []string{
	OutcomeServfailUpstream,
	OutcomeTimeout,
	OutcomeNetworkError,
	OutcomeRefused,
	OutcomeFilterUnavailable,
}
