package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/rs/zerolog/log"
)

type createProfileBody struct {
	Name string `json:"name" validate:"required,min=1,max=50"`
}

// @Summary Create profile
// @Description Create profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body createProfileBody true "Create profile request"
// @Success 201 {object} model.Profile
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles [post]
func (s *APIServer) createProfile() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(createProfileBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToCreateProfile.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		prof, err := s.Service.CreateProfile(c.Context(), p.Name, accountId)
		if err != nil {
			return HandleError(c, err, ErrFailedToCreateProfile.Error())
		}

		update := model.AccountUpdate{
			Operation: model.UpdateOperationAdd,
			Path:      "/profiles",
			Value:     prof.ProfileId,
		}
		if err = s.Service.UpdateAccount(c.Context(), accountId, []model.AccountUpdate{update}, nil); err != nil {
			return HandleError(c, err, ErrFailedToUpdateAccount.Error())
		}

		return c.Status(201).JSON(prof)
	}
	return handler
}

// @Summary Get profiles data
// @Description Get profiles data
// @Tags Profile
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} []model.Profile
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles [get]
func (s *APIServer) getProfiles() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)
		profiles, err := s.Service.GetProfiles(c.Context(), accountId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get profiles data")
			return HandleError(c, err, dbErrors.ErrProfileNotFound.Error())
		}

		return c.Status(200).JSON(profiles)
	}
	return handler
}

// @Summary Get profile data
// @Description Get profile data
// @Tags Profile
// @Param id path string true "Profile ID"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.Profile
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id} [get]
func (s *APIServer) getProfile() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")
		accountId := auth.GetAccountID(c)
		profile, err := s.Service.GetProfile(c.Context(), accountId, profileId)
		if err != nil {
			return HandleError(c, err, "failed to get profile data")
		}

		return c.Status(200).JSON(profile)
	}
	return handler
}

// @Summary Delete profile
// @Description Delete profile
// @Tags Profile
// @Param id path string true "Profile ID"
// @Produce json
// @Security ApiKeyAuth
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id} [delete]
func (s *APIServer) deleteProfile() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		accountId := auth.GetAccountID(c)
		if err := s.Service.DeleteProfile(c.Context(), accountId, profileId, false); err != nil {
			log.Error().Err(err).Msg("Failed to delete profile")
			if errors.Is(err, profile.ErrLastProfileInAccount) {
				return HandleError(c, err, profile.ErrLastProfileInAccount.Error())
			}
			return HandleError(c, err, dbErrors.ErrProfileNotFound.Error())
		}

		update := model.AccountUpdate{
			Operation: model.UpdateOperationRemove,
			Path:      "/profiles",
			Value:     profileId,
		}
		if err := s.Service.UpdateAccount(c.Context(), accountId, []model.AccountUpdate{update}, nil); err != nil {
			return HandleError(c, err, ErrFailedToUpdateAccount.Error())
		}

		return c.SendStatus(204)
	}
	return handler
}

// @Summary Update profile
// @Description Update profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Profile ID"
// @Param body body requests.ProfileUpdates true "Update profile"
// @Success 200 {object} model.Profile
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/profiles/{id} [patch]
func (s *APIServer) updateProfile() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		profileId := c.Params("id")

		p := new(requests.ProfileUpdates)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToUpdateProfile.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		// TODO: create our context, do not use fiber one
		accountId := auth.GetAccountID(c)
		profile, err := s.Service.UpdateProfile(c.Context(), accountId, profileId, p.Updates)
		if err != nil {
			return HandleError(c, err, ErrFailedToUpdateProfile.Error())
		}

		return c.Status(200).JSON(profile)
	}
	return handler
}
