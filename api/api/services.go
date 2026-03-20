package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/libs/servicescatalog"
)

type ServicesUpdates struct {
	ServiceIds []string `json:"service_ids" validate:"required,min=1,max=100,dive,required"`
}

// @Summary Get services catalog
// @Description Get available ASN-based services presets
// @Tags Services
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} servicescatalog.Catalog
// @Failure 500 {object} ErrResponse
// @Router /api/v1/services [get]
func (s *APIServer) getServicesCatalog() fiber.Handler {
	h := func(c *fiber.Ctx) error {
		cat, err := s.ServicesCatalog.Get()
		if err != nil {
			return HandleError(c, err, "Failed to load services catalog")
		}
		if cat == nil {
			cat = &servicescatalog.Catalog{Services: []servicescatalog.Service{}}
		}
		return c.Status(200).JSON(cat)
	}
	return h
}

// @Summary Enable services
// @Description Enable services for a profile (adds to privacy.services)
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param service_ids body ServicesUpdates true "Services to enable"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/services [post]
func (s *APIServer) enableServices() fiber.Handler {
	h := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		profileId := c.Params("id")

		updates := new(ServicesUpdates)
		if err := c.BodyParser(updates); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}
		errMsgs := s.Validator.ValidateRequest(c, updates, ErrFailedToEnableServices.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		if len(updates.ServiceIds) == 0 {
			return HandleError(c, ErrInvalidServiceValue, "Service IDs are required")
		}

		if err := s.Service.EnableServices(c.Context(), accountId, profileId, updates.ServiceIds); err != nil {
			return HandleError(c, err, "Failed to enable services")
		}
		return c.SendStatus(200)
	}
	return h
}

// @Summary Disable services
// @Description Disable services for a profile (removes from privacy.services)
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param service_ids body ServicesUpdates true "Services to disable"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id}/services [delete]
func (s *APIServer) disableServices() fiber.Handler {
	h := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		profileId := c.Params("id")

		updates := new(ServicesUpdates)
		if err := c.BodyParser(updates); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}
		errMsgs := s.Validator.ValidateRequest(c, updates, ErrFailedToDisableServices.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}
		if len(updates.ServiceIds) == 0 {
			return HandleError(c, ErrInvalidServiceValue, "Service IDs are required")
		}

		if err := s.Service.DisableServices(c.Context(), accountId, profileId, updates.ServiceIds); err != nil {
			return HandleError(c, err, "Failed to disable services")
		}
		return c.SendStatus(200)
	}
	return h
}
