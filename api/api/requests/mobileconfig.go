package requests

type MobileConfigReq struct {
	ProfileId string `json:"profile_id" validate:"required"`
	// DeviceId is an optional human-friendly identifier for the device.
	// It will be normalized (allowing only [A-Za-z0-9 -]) and truncated to a max length
	// consistent with the DNS proxy rules (currently 16). When provided, generated
	// mobileconfig profile endpoints (DoH / DoT / DoQ) will embed it so queries can
	// be attributed per-device.
	DeviceId            string `json:"device_id" validate:"omitempty,device_id"`
	*AdvancedOptionsReq `json:"advanced_options"`
}

type AdvancedOptionsReq struct {
	EncryptionType string `json:"encryption_type" validate:"required,oneof=https tls"`
	// ExcludedDomains          string `json:"excluded_domains"`
	ExcludedWifiNetworks string `json:"excluded_wifi_networks"`
	// PayloadRemovalDisallowed *bool  `json:"payload_removal_disallowed"`
	// SignConfigurationProfile *bool `json:"sign_configuration_profile"`
}
