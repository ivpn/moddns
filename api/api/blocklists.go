package api

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/rs/zerolog/log"
)

type BlocklistsUpdates struct {
	BlocklistIds []string `json:"blocklist_ids" validate:"required"`
}

// @Summary Get blocklists data
// @Description Get available blocklists data
// @Tags Blocklists
// @Produce json
// @Security ApiKeyAuth
// @Param        sort_by    query     string  false  "field to sort by" Enums(updated,name,entries) default(updated)
// @Success 200 {object} []model.Blocklist
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/blocklists [get]
func (s *APIServer) getBlocklists() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		queryParams := requests.BlocklistsQueryParams{
			SortBy: c.Query("sort_by", "updated"),
		}
		if err := s.Validator.Validator.Struct(queryParams); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		filter := make(map[string]any)
		defaultBlocklist := c.Query("default")
		if defaultBlocklist != "" {
			log.Debug().Msgf("query param - default: %s", defaultBlocklist)
			boolDefault, err := strconv.ParseBool(defaultBlocklist)
			if err != nil {
				return HandleError(c, err, "Invalid request path param: default")
			}
			filter["default"] = boolDefault
		}

		blocklists, err := s.Service.GetBlocklist(c.Context(), filter, queryParams.SortBy)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get blocklists")
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to get blocklists",
			})
		}

		return c.Status(200).JSON(blocklists)
	}
	return handler
}

// @Summary Enable blocklists
// @Description Enable blocklists for a profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param blocklist_ids body BlocklistsUpdates true "Blocklists to disable"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/blocklists [post]
func (s *APIServer) enableBlocklists() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		profileId := c.Params("id")

		updates := new(BlocklistsUpdates)
		if err := c.BodyParser(updates); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, updates, ErrFailedToEnableBlocklists.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		if len(updates.BlocklistIds) == 0 {
			return HandleError(c, ErrInvalidBlocklistValue, "Blocklist IDs are required")
		}

		if err := s.Service.EnableBlocklists(c.Context(), accountId, profileId, updates.BlocklistIds); err != nil {
			return HandleError(c, err, "Failed to enable blocklists")
		}

		return c.SendStatus(200)
	}
	return handler
}

// @Summary Disable blocklists
// @Description Disable blocklists for a profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param blocklist_ids body BlocklistsUpdates true "Blocklists to disable"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/blocklists [delete]
func (s *APIServer) disableBlocklists() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		profileId := c.Params("id")

		updates := new(BlocklistsUpdates)
		if err := c.BodyParser(updates); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, updates, ErrFailedToDisableBlocklists.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		if len(updates.BlocklistIds) == 0 {
			return HandleError(c, ErrInvalidBlocklistValue, "Blocklist IDs are required")
		}

		if err := s.Service.DisableBlocklists(c.Context(), accountId, profileId, updates.BlocklistIds); err != nil {
			return HandleError(c, err, "Failed to disable blocklists")
		}

		return c.SendStatus(200)
	}
	return handler
}
