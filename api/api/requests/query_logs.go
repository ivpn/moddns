package requests

type LogsSettingsUpdates struct {
	Enabled       *bool   `json:"enabled" validate:"omitempty,boolean"`
	LogClientsIPs *bool   `json:"log_clients_ips" validate:"omitempty,boolean"`
	LogDomains    *bool   `json:"log_domains" validate:"omitempty,boolean"`
	Retention     *string `json:"retention" validate:"omitempty,oneof=1h 6h 1d 1w 1m"`
}

type QueryLogsQueryParams struct {
	Page     int    `json:"page" validate:"required,numeric,min=1"`
	Limit    int    `json:"limit" validate:"required,oneof=10 25 50 100"`
	Timespan string `json:"timespan" validate:"oneof=LAST_1_HOUR LAST_12_HOURS LAST_1_DAY LAST_7_DAYS LAST_MONTH"`
	Status   string `json:"status" validate:"omitempty,oneof=all blocked processed unanswered"`
	DeviceId string `json:"device_id" validate:"omitempty"`
	Search   string `json:"search" validate:"omitempty,max=256"`
	SortBy   string `json:"sort_by" validate:"oneof=created domain client_ip"`
}

type QueryLogsTopQueryParams struct {
	Timespan string `json:"timespan" validate:"oneof=LAST_1_HOUR LAST_3_HOURS LAST_6_HOURS LAST_12_HOURS LAST_1_DAY LAST_7_DAYS LAST_MONTH"`
	Kind     string `json:"kind" validate:"oneof=blocked resolved"`
	Limit    int    `json:"limit" validate:"min=1,max=50"`
}

type QueryLogsClientsQueryParams struct {
	Timespan string `json:"timespan" validate:"oneof=LAST_1_HOUR LAST_3_HOURS LAST_6_HOURS LAST_12_HOURS LAST_1_DAY LAST_7_DAYS LAST_MONTH"`
	Limit    int    `json:"limit" validate:"min=1,max=50"`
}
