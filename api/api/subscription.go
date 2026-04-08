package api

import (
	"github.com/gofiber/fiber/v2"
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
