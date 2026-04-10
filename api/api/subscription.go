package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// reference model.Subscription to satisfy import for swagger annotations
var _ model.Subscription

// @Summary Get subscription data
// @Description Get subscription data for the authenticated account
// @Tags Subscription
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.Subscription
// @Failure 401 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/sub [get]
func (s *APIServer) getSubscription() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)

		subscription, err := s.Service.GetSubscription(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, ErrFailedToGetSubscription.Error())
		}
		return c.Status(200).JSON(subscription)
	}
}

// @Summary Update subscription via PASession
// @Description Resync subscription using a pre-auth session
// @Tags Subscription
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.SubscriptionUpdateReq true "Subscription update request"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Router /api/v1/sub/update [put]
func (s *APIServer) updateSubscription() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(PASessionCookie)
		accountId := auth.GetAccountID(c)

		req := new(requests.SubscriptionUpdateReq)
		if err := c.BodyParser(req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, req, ErrInvalidRequestBody.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		sub, err := s.Service.GetSubscription(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, ErrFailedToGetSubscription.Error())
		}

		if err := s.Service.UpdateSubscriptionFromPASession(c.Context(), sub, req.SubID, sessionID); err != nil {
			return HandleError(c, err, "failed to update subscription")
		}

		return c.Status(200).JSON(fiber.Map{"message": "Subscription updated successfully."})
	}
}
