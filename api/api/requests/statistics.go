package requests

type StatisticsQueryParams struct {
	Timespan string `json:"timespan" validate:"oneof=LAST_3_HOURS LAST_6_HOURS LAST_1_DAY LAST_7_DAYS LAST_MONTH LAST_3_MONTHS LAST_YEAR"`
}
