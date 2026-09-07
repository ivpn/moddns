package dns

const (
	StatusConfigured   = "ok"
	StatusUnconfigured = "unconfigured"
)

// DNSLogRecord is what the DNS side stores for the HTTP side to read. The
// client IP and ASN only ever feed the status decision and are not kept.
type DNSLogRecord struct {
	Status    string `json:"status"`
	ProfileId string `json:"profile_id"`
}

// DNSCheckResponse is the minimal HTTP response payload.
// Only status and profile_id are needed by the frontend.
type DNSCheckResponse struct {
	Status    string `json:"status"`
	ProfileId string `json:"profile_id"`
}
