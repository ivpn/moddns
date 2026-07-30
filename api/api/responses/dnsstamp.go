package responses

// DNSStampResponse is the response body for POST /api/v1/dnsstamp.
//
// Each field is an sdns:// string ready to paste into a stamp-consuming
// client (UniFi Network, dnscrypt-proxy, AdGuard Home, etc.). All three
// stamps target the same modDNS profile; the user picks whichever protocol
// their client expects.
type DNSStampResponse struct {
	DoH string `json:"doh"`
	DoT string `json:"dot"`
	DoQ string `json:"doq"`
}
