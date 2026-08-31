package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// @Summary Get statistics data for a profile
// @Description Get statistics data for a profile
// @Tags Statistics
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param        timespan		   query     string  false  "specify timespan for query" default("LAST_MONTH")
// @Success 200 {object} []model.StatisticsAggregated
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/statistics [get]
func (s *APIServer) getStatistics() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		queryParams := requests.StatisticsQueryParams{
			Timespan: c.Query("timespan", model.LAST_MONTH),
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
