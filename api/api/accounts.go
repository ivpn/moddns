package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/account"
)

type registerAccountBody struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"password,required"`
	SubID    string `json:"subid" validate:"required,uuid4"`
}

// @Summary Register account
// @Description Register account
// @Tags Account
// @Accept json
// @Produce json
// @Param body body registerAccountBody true "Account request"
// @Success 201 {object} responses.RegistrationSuccessResponse
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts [post]
func (s *APIServer) registerAccount() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(registerAccountBody)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errs := s.Validator.Validate(p)
		if len(errs) > 0 {
			tags := make([]string, 0)
			for _, err := range errs {
				tags = append(tags, err.Tag)
			}
			return HandleError(c, ErrValidationFailed, "validation failed", tags...)
		}

		_, err := s.Service.GetUnfinishedSignupOrPostAccount(c.Context(), p.Email, p.Password, p.SubID)
		if err != nil {
			// Map specific service errors to unified user-facing failure
			if _, ok := err.(*account.ServiceAccountError); ok && err == account.ErrUnableToCreateAccount {
				return c.Status(400).JSON(fiber.Map{"error": account.ErrUnableToCreateAccount.Error()})
			}
			// When subscription already exists (duplicate subid) treat as unable to create account (finished or conflicting state)
			if errors.Is(err, dbErrors.ErrSubscriptionAlreadyExists) {
				return c.Status(400).JSON(fiber.Map{"error": account.ErrUnableToCreateAccount.Error()})
			}
			return HandleError(c, err, ErrFailedToRegisterAccount.Error())
		}

		// Success response (account object suppressed per scenario requirements)
		return c.Status(201).JSON(fiber.Map{"message": "Account created successfully."})
	}
	return handler
}

// @Summary Get account data
// @Description Get account data
// @Tags Account
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.Account
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/current [get]
func (s *APIServer) getAccount() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)

		account, err := s.Service.GetAccount(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, ErrFailedToGetAccount.Error())
		}

		return c.Status(200).JSON(account)
	}
	return handler
}

// @Summary Update account
// @Description Update account
// @Tags Account
// @Accept json
// @Produce json
// @Param body body requests.AccountUpdates true "Update account request"
// @Param x-mfa-code header string false "MFA OTP code"
// @Param x-mfa-methods header []string false "MFA methods"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts [patch]
func (s *APIServer) updateAccount() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		updates := new(requests.AccountUpdates)
		if err := c.BodyParser(updates); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, updates, ErrFailedToUpdateAccount.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		mfa := auth.GetMfaData(c)
		accountId := auth.GetAccountID(c)
		if err := s.Service.UpdateAccount(c.Context(), accountId, updates.Updates, mfa); err != nil {
			return HandleError(c, err, ErrFailedToUpdateAccount.Error())
		}

		return c.SendStatus(204)
	}
	return handler
}

// @Summary Delete account
// @Description Delete account with deletion code
// @Tags Account
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.AccountDeletionRequest true "Account deletion request"
// @Success 204
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/current [delete]
func (s *APIServer) deleteAccount() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)

		payload := new(requests.AccountDeletionRequest)
		if err := c.BodyParser(payload); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, payload, ErrFailedToDeleteAccount.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		mfa := auth.GetMfaData(c)
		err := s.Service.DeleteAccount(c.Context(), accountId, *payload, mfa)
		if err != nil {
			return HandleError(c, err, ErrFailedToDeleteAccount.Error())
		}

		return c.SendStatus(204)
	}
	return handler
}

// @Summary Generate deletion code
// @Description Generate a deletion code for account deletion
// @Tags Account
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} responses.DeletionCodeResponse
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/accounts/current/deletion-code [post]
func (s *APIServer) generateDeletionCode() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)

		response, err := s.Service.GenerateDeletionCode(c.Context(), accountId)
		if err != nil {
			return HandleError(c, err, "Failed to generate deletion code")
		}

		return c.JSON(response)
	}
	return handler
}
