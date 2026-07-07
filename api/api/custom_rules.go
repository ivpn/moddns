package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/rs/zerolog/log"
)

// @Summary Create profile custom rule
// @Description Create profile custom rule
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.CreateProfileCustomRuleBody true "Create custom rule request"
// @Success 201
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rules [post]
func (s *APIServer) createProfileCustomRule() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.CreateProfileCustomRuleBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToCreateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidCustomRuleSyntax, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		if err := s.Service.CreateCustomRule(c.Context(), accountId, profileId, p.Action, p.Value); err != nil {
			return HandleError(c, err, ErrFailedToCreateCustomRule.Error())
		}

		return c.SendStatus(201)
	}
	return handler
}

// @Summary Create profile custom rules (batch)
// @Description Create up to 20 custom rules for a profile in a single request
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.CreateProfileCustomRulesBatchBody true "Create custom rules batch request"
// @Success 200 {object} responses.CreateProfileCustomRulesBatchResponse
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rules/batch [post]
func (s *APIServer) createProfileCustomRulesBatch() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.CreateProfileCustomRulesBatchBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToCreateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidCustomRuleSyntax, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		result, err := s.Service.CreateCustomRulesBulk(c.Context(), accountId, profileId, p.Action, p.Values)
		if err != nil {
			return HandleError(c, err, ErrFailedToCreateCustomRule.Error())
		}

		response := responses.CreateProfileCustomRulesBatchResponse{
			Action:         string(result.Action),
			TotalRequested: result.TotalRequested,
			Created:        make([]responses.CustomRuleBatchCreated, len(result.Created)),
			Skipped:        make([]responses.CustomRuleBatchSkipped, len(result.Skipped)),
		}

		for i, rule := range result.Created {
			response.Created[i] = responses.CustomRuleBatchCreated{
				ID:    rule.ID.Hex(),
				Value: rule.Value,
			}
		}

		for i, skipped := range result.Skipped {
			response.Skipped[i] = responses.CustomRuleBatchSkipped{
				Value:   skipped.Value,
				Reason:  string(skipped.Reason),
				Message: skipped.Message,
			}
		}

		return c.Status(fiber.StatusOK).JSON(response)
	}
	return handler
}

// @Summary Update profile custom rule
// @Description Partially update a single custom rule in place (value, action, note, group, order)
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param profile_id path string true "Profile ID"
// @Param custom_rule_id path string true "Custom rule ID"
// @Param body body requests.UpdateProfileCustomRuleBody true "Update custom rule request"
// @Success 200 {object} model.CustomRule
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{profile_id}/custom_rules/{custom_rule_id} [patch]
func (s *APIServer) updateProfileCustomRule() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("profile_id")
		customRuleId := c.Params("custom_rule_id")

		p := new(requests.UpdateProfileCustomRuleBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToUpdateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidCustomRuleSyntax, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		rule, err := s.Service.UpdateCustomRule(c.Context(), accountId, profileId, customRuleId, profile.CustomRulePatch{
			Action: p.Action,
			Value:  p.Value,
			Note:   p.Note,
			Group:  p.Group,
			Order:  p.Order,
		})
		if err != nil {
			return HandleError(c, err, ErrFailedToUpdateCustomRule.Error())
		}

		return c.Status(fiber.StatusOK).JSON(rule)
	}
	return handler
}

// @Summary Reorder profile custom rules
// @Description Set the display order of a profile's custom rules. Order is organizational only and does not affect filtering precedence.
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.ReorderProfileCustomRulesBody true "Ordered rule IDs"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rules/order [patch]
func (s *APIServer) reorderProfileCustomRules() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.ReorderProfileCustomRulesBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToUpdateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		if err := s.Service.ReorderCustomRules(c.Context(), accountId, profileId, p.Order); err != nil {
			return HandleError(c, err, ErrFailedToUpdateCustomRule.Error())
		}

		return c.SendStatus(200)
	}
	return handler
}

// @Summary Reorder profile custom rule groups
// @Description Set the display order of a list's custom-rule group registry. action scopes it to one list (block=denylist, allow=allowlist); groups are per-list. Order is organizational only; unknown names are ignored and omitted groups keep their order, so a stale client list cannot drop a group.
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.ReorderProfileCustomRuleGroupsBody true "List and ordered group names"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rule_groups/order [patch]
func (s *APIServer) reorderProfileCustomRuleGroups() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.ReorderProfileCustomRuleGroupsBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToUpdateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		if err := s.Service.ReorderCustomRuleGroups(c.Context(), accountId, profileId, p.Action, p.Order); err != nil {
			return HandleError(c, err, ErrFailedToUpdateCustomRule.Error())
		}

		return c.SendStatus(200)
	}
	return handler
}

// decodeJSONPointerSegment turns a single-segment RFC6901 JSON Pointer ("/<name>")
// into the plain group name, unescaping ~1 -> "/" and ~0 -> "~". Returns "" for an
// empty pointer or "/".
func decodeJSONPointerSegment(pointer string) string {
	seg := strings.TrimPrefix(pointer, "/")
	// Unescape in the RFC6901-mandated order: ~1 before ~0.
	seg = strings.ReplaceAll(seg, "~1", "/")
	seg = strings.ReplaceAll(seg, "~0", "~")
	return seg
}

// @Summary Update profile custom rule groups
// @Description Apply JSON-Patch-style operations to the custom-rule group registry. Group names travel in the JSON-Pointer path/from (never the URL). operation=add|replace sets a group's note (creating it); remove deletes a group (its rules move to Ungrouped, not deleted); move renames from->path.
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.CustomRuleGroupUpdates true "Group operations"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rule_groups [patch]
func (s *APIServer) updateProfileCustomRuleGroups() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.CustomRuleGroupUpdates)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToUpdateCustomRule.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		ops := make([]profile.CustomRuleGroupOp, 0, len(p.Updates))
		for _, u := range p.Updates {
			ops = append(ops, profile.CustomRuleGroupOp{
				Operation: u.Operation,
				Action:    u.Action,
				Group:     decodeJSONPointerSegment(u.Path),
				From:      decodeJSONPointerSegment(u.From),
				Note:      u.Value,
			})
		}

		accountId := auth.GetAccountID(c)
		if err := s.Service.ApplyCustomRuleGroupOps(c.Context(), accountId, profileId, ops); err != nil {
			return HandleError(c, err, ErrFailedToUpdateCustomRule.Error())
		}

		return c.SendStatus(200)
	}
	return handler
}

// @Summary Delete profile custom rule
// @Description Delete profile custom rule
// @Tags Profile
// @Param id path string true "Profile ID"
// @Param custom_rule_id path string true "Custom rule ID"
// @Produce json
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/custom_rules/{custom_rule_id} [delete]
func (s *APIServer) deleteProfileCustomRule() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("profile_id")
		customRuleId := c.Params("custom_rule_id")
		accountId := auth.GetAccountID(c)

		if err := s.Service.DeleteCustomRule(c.Context(), accountId, profileId, customRuleId); err != nil {
			log.Error().Err(err).Msg(ErrFailedToDeleteCustomRule.Error())
			return HandleError(c, err, ErrFailedToDeleteCustomRule.Error())
		}

		return c.SendStatus(200)
	}
	return handler
}
