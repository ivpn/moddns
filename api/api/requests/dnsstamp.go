package requests

// DNSStampReq is the request payload for POST /api/v1/dnsstamp.
//
// ProfileId is required and must match the same shape used elsewhere in the
// API: alphanumeric, length 10–64. DeviceId is optional and, when present,
// scopes the generated stamps to a specific device label for per-device
// query log attribution.
type DNSStampReq struct {
	ProfileId string `json:"profile_id" validate:"required,alphanum,min=10,max=64"`
	// DeviceId is an optional human-friendly identifier for the device.
	// It is normalized via libs/deviceid.Normalize (allowing only [A-Za-z0-9 -])
	// before being embedded in the stamps. Empty means "profile-only stamp".
	DeviceId string `json:"device_id" validate:"omitempty,device_id"`
}
