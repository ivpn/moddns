package model

const (
	StatusBlocked   Status = "blocked"
	StatusProcessed Status = "processed"
	// StatusUnavailable means a settings-store read failed before filtering could
	// complete. The query is never resolved upstream and is answered SERVFAIL.
	StatusUnavailable Status = "unavailable"
)

type Status string

type FilterResult struct {
	Status  Status   `json:"status"`
	Reasons []string `json:"reasons"`
}
