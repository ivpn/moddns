package api

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// @Summary Get profile query logs
// @Description Get profile query logs
// @Tags QueryLogs
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param        page    query     int  false  "specify page number" default(1)
// @Param        limit    query     int  false  "specify logs limit by page" default(100)
// @Param        status    query     string  false  "specify status for query" default("all")
// @Param        timespan    query     string  false  "specify timespan for query" default("LAST_1_HOUR")
// @Param        device_id    query     string  false  "specify device ID for filtering"
// @Param        search    query     string  false  "substring (case-insensitive) match against stored domain; free-form (short inputs may scan more)"
// @Param        sort_by    query     string  false  "field to sort by" Enums(created,domain,client_ip) default(created)
// @Success 200 {object} []model.QueryLog
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs [get]
func (s *APIServer) getProfileQueryLogs() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		queryParams := requests.QueryLogsQueryParams{
			Page:     c.QueryInt("page", 1),
			Limit:    c.QueryInt("limit", 25),
			Timespan: c.Query("timespan", model.LAST_1_HOUR),
			Status:   c.Query("status", "all"),
			DeviceId: c.Query("device_id", ""),
			Search:   c.Query("search", ""),
			SortBy:   c.Query("sort_by", "created"),
		}
		err := s.Validator.Validator.Struct(queryParams)
		if err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		queryLogs, err := s.Service.GetProfileQueryLogs(c.Context(), accountId, profileId, queryParams.Status, queryParams.Timespan, queryParams.DeviceId, queryParams.Search, queryParams.SortBy, queryParams.Page, queryParams.Limit)
		if err != nil {
			log.Error().Err(err).Msg(ErrFailedToGetQueryLogs.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogs.Error())
		}

		return c.Status(200).JSON(queryLogs)
	}
	return handler
}

// @Summary Download profile query logs
// @Description Download profile query logs
// @Tags QueryLogs
// @Produce application/json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Success 200 {object} []model.QueryLog
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs/download [get]
func (s *APIServer) downloadProfileQueryLogs() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		accountId := auth.GetAccountID(c)
		queryLogs, err := s.Service.DownloadProfileQueryLogs(c.Context(), accountId, profileId, 0, 0)
		if err != nil {
			log.Error().Err(err).Msg(ErrFailedToGetQueryLogs.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogs.Error())
		}

		jsonFile, err := json.Marshal(queryLogs)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal query logs")
			return HandleError(c, err, "Failed to marshal query logs")
		}
		reader := bytes.NewReader(jsonFile)

		c.Attachment("dns-query-logs.json")
		return c.SendStream(reader)
	}
	return handler
}

// @Summary Delete profile query logs
// @Description Delete profile query logs
// @Tags QueryLogs
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs [delete]
func (s *APIServer) deleteProfileQueryLogs() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		accountId := auth.GetAccountID(c)
		err := s.Service.DeleteProfileQueryLogs(c.Context(), accountId, profileId)
		if err != nil {
			log.Error().Err(err).Msg(ErrFailedToDeleteQueryLogs.Error())
			return HandleError(c, err, ErrFailedToDeleteQueryLogs.Error())
		}

		return c.SendStatus(204)
	}
	return handler
}
