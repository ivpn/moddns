package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// @Summary Get statistics data for a profile
// @Description Get the profile's DNS statistics for a timespan: totals, a zero-filled time series, blocking reasons, transport protocols and per-device counts. Always answers 200 for an owned profile; `enabled` is false (with empty data) when statistics are off.
// @Tags Statistics
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param        timespan		   query     string  false  "specify timespan for query" Enums(LAST_3_HOURS,LAST_6_HOURS,LAST_1_DAY,LAST_7_DAYS,LAST_MONTH,LAST_3_MONTHS,LAST_YEAR) default(LAST_7_DAYS)
// @Success 200 {object} model.StatisticsResponse
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/statistics [get]
func (s *APIServer) getStatistics() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		queryParams := requests.StatisticsQueryParams{
			Timespan: c.Query("timespan", model.StatisticsDefaultTimespan),
		}
		err := s.Validator.Validator.Struct(queryParams)
		if err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		stats, err := s.Service.GetStatistics(c.UserContext(), accountId, profileId, queryParams.Timespan)
		if err != nil {
			return HandleError(c, err, ErrFailedToGetStatistics.Error())
		}

		return c.Status(200).JSON(stats)
	}
	return handler
}

// @Summary Delete a profile's statistics history
// @Description Delete every stored statistic of the profile, including the current hour and day; statistics stay on with the same retention.
// @Tags Statistics
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/statistics [delete]
func (s *APIServer) deleteStatisticsHistory() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		if err := s.Service.DeleteStatisticsHistory(c.UserContext(), accountId, c.Params("id")); err != nil {
			return HandleError(c, err, ErrFailedToDeleteStatistics.Error())
		}
		return c.SendStatus(204)
	}
}
