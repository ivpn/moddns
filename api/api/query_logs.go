package api

import (
	"bytes"
	"encoding/json"
	"strconv"

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
// @Param        status    query     string  false  "specify status for query" Enums(all,blocked,processed,unanswered) default("all")
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
			Status:   c.Query("status", model.QueryLogStatusAll),
			DeviceId: c.Query("device_id", ""),
			Search:   c.Query("search", ""),
			SortBy:   c.Query("sort_by", "created"),
		}
		err := s.Validator.Validator.Struct(queryParams)
		if err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		queryLogs, err := s.Service.GetProfileQueryLogs(c.UserContext(), accountId, profileId, queryParams.Status, queryParams.Timespan, queryParams.DeviceId, queryParams.Search, queryParams.SortBy, queryParams.Page, queryParams.Limit)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogs.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogs.Error())
		}

		return c.Status(200).JSON(queryLogs)
	}
	return handler
}

// @Summary Get profile query log devices
// @Description List distinct device IDs seen in the profile's query logs (current retention window), each with its last-seen timestamp, sorted by device ID
// @Tags QueryLogs
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Success 200 {object} []model.QueryLogDevice
// @Failure 404 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs/devices [get]
func (s *APIServer) getProfileQueryLogDevices() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		accountId := auth.GetAccountID(c)
		devices, err := s.Service.GetProfileQueryLogDevices(c.UserContext(), accountId, profileId)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogDevices.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogDevices.Error())
		}

		return c.Status(200).JSON(devices)
	}
	return handler
}

// topLimitParam returns the "limit" query value, 10 when absent. A present but
// non-integer value maps to 0, which the request validation rejects (min=1).
func topLimitParam(c *fiber.Ctx) int {
	if !c.Context().QueryArgs().Has("limit") {
		return 10
	}
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		return 0
	}
	return n
}

// @Summary Get profile top domains
// @Description Most frequent blocked or resolved domains in the profile's query logs (current retention window). Returns enabled=false with no items unless logs and domain logging are on. Counts only.
// @Tags QueryLogs
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param timespan query string false "specify timespan for query" Enums(LAST_1_HOUR,LAST_3_HOURS,LAST_6_HOURS,LAST_12_HOURS,LAST_1_DAY,LAST_7_DAYS,LAST_MONTH) default(LAST_1_DAY)
// @Param kind query string true "which domains to rank" Enums(blocked,resolved)
// @Param limit query int false "number of items" minimum(1) maximum(50) default(10)
// @Success 200 {object} model.QueryLogTopDomains
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs/top [get]
func (s *APIServer) getProfileQueryLogTop() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		params := requests.QueryLogsTopQueryParams{
			Timespan: c.Query("timespan", model.LAST_1_DAY),
			Kind:     c.Query("kind", ""),
			Limit:    topLimitParam(c),
		}
		if err := s.Validator.Validator.Struct(params); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		top, err := s.Service.GetProfileQueryLogTop(c.UserContext(), accountId, profileId, params.Timespan, params.Kind, params.Limit)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogTop.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogTop.Error())
		}

		return c.Status(200).JSON(top)
	}
	return handler
}

// @Summary Get profile top clients
// @Description Most frequent client IPs in the profile's query logs (current retention window), enriched with ASN, AS organisation and country (null when unknown). Returns enabled=false with no items unless logs and client IP logging are on.
// @Tags QueryLogs
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param timespan query string false "specify timespan for query" Enums(LAST_1_HOUR,LAST_3_HOURS,LAST_6_HOURS,LAST_12_HOURS,LAST_1_DAY,LAST_7_DAYS,LAST_MONTH) default(LAST_1_DAY)
// @Param limit query int false "number of items" minimum(1) maximum(50) default(10)
// @Success 200 {object} model.QueryLogTopClients
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs/clients [get]
func (s *APIServer) getProfileQueryLogClients() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		params := requests.QueryLogsClientsQueryParams{
			Timespan: c.Query("timespan", model.LAST_1_DAY),
			Limit:    topLimitParam(c),
		}
		if err := s.Validator.Validator.Struct(params); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		clients, err := s.Service.GetProfileQueryLogClients(c.UserContext(), accountId, profileId, params.Timespan, params.Limit)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogClients.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogClients.Error())
		}

		return c.Status(200).JSON(clients)
	}
	return handler
}

// @Summary Get profile top blocklists
// @Description Blocklists that blocked the most queries in the profile's query logs (current retention window), by blocklist id. A query matched by several blocklists counts once for each, so counts can sum to more than the blocked total. Returns enabled=false with no items unless logs are on. Counts only.
// @Tags QueryLogs
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param timespan query string false "specify timespan for query" Enums(LAST_1_HOUR,LAST_3_HOURS,LAST_6_HOURS,LAST_12_HOURS,LAST_1_DAY,LAST_7_DAYS,LAST_MONTH) default(LAST_1_DAY)
// @Param limit query int false "number of items" minimum(1) maximum(50) default(10)
// @Success 200 {object} model.QueryLogTopBlocklists
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/logs/blocklists [get]
func (s *APIServer) getProfileQueryLogBlocklists() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		params := requests.QueryLogsBlocklistsQueryParams{
			Timespan: c.Query("timespan", model.LAST_1_DAY),
			Limit:    topLimitParam(c),
		}
		if err := s.Validator.Validator.Struct(params); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		accountId := auth.GetAccountID(c)
		blocklists, err := s.Service.GetProfileQueryLogBlocklists(c.UserContext(), accountId, profileId, params.Timespan, params.Limit)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogBlocklists.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogBlocklists.Error())
		}

		return c.Status(200).JSON(blocklists)
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
		queryLogs, err := s.Service.DownloadProfileQueryLogs(c.UserContext(), accountId, profileId, 0, 0)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToGetQueryLogs.Error())
			return HandleError(c, err, ErrFailedToGetQueryLogs.Error())
		}

		jsonFile, err := json.Marshal(queryLogs)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg("Failed to marshal query logs")
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
		err := s.Service.DeleteProfileQueryLogs(c.UserContext(), accountId, profileId)
		if err != nil {
			log.Ctx(c.UserContext()).Error().Err(err).Msg(ErrFailedToDeleteQueryLogs.Error())
			return HandleError(c, err, ErrFailedToDeleteQueryLogs.Error())
		}

		return c.SendStatus(204)
	}
	return handler
}
