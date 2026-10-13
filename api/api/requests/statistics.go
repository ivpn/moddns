package requests

type StatisticsQueryParams struct {
	Timespan string `json:"timespan" validate:"statistics_timespan"`
}
