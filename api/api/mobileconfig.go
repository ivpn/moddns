package api

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/apple"
)

// @Summary Generate configuration profile for Apple devices
// @Description Generate configuration profile for Apple devices
// @Tags Apple mobileconfig
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.MobileConfigReq true "Generate .mobileconfig request"
// @Success 201 {object} string
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/mobileconfig [post]
func (s *APIServer) generateMobileConfig() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.MobileConfigReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToGenerateMobileConfig.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		_, err := s.Service.GetProfile(c.Context(), accountId, p.ProfileId)
		if err != nil {
			return HandleError(c, err, ErrFailedToGenerateMobileConfig.Error())
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mobileconfig, _, err := s.Service.GenerateMobileConfig(ctx, *p, accountId, false)
		if err != nil {
			return HandleError(c, err, ErrFailedToGenerateMobileConfig.Error())
		}

		c.Set("Content-Type", "application/x-apple-aspen-config")
		// Use inline disposition - frontend handles download via blob for desktop browsers,
		// while direct navigation triggers iOS profile installation on mobile devices
		// Set filename with profile_id for proper identification
		filename := fmt.Sprintf("inline; filename=modDNS-%s.mobileconfig", p.ProfileId)
		c.Set("Content-Disposition", filename)
		return c.Status(201).SendString(string(mobileconfig))
	}
	return handler
}

// @Summary Generate short link for configuration profile (Apple devices)
// @Description Generate short link for configuration profile (Apple devices)
// @Tags Apple mobileconfig
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.MobileConfigReq true "Generate .mobileconfig request"
// @Success 200 {object} responses.ShortLinkResponse
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/mobileconfig/short [post]
func (s *APIServer) generateMobileConfigShortLink() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		p := new(requests.MobileConfigReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToGenerateMobileConfig.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		accountId := auth.GetAccountID(c)
		_, err := s.Service.GetProfile(c.Context(), accountId, p.ProfileId)
		if err != nil {
			return HandleError(c, err, ErrFailedToGenerateMobileConfig.Error())
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, link, err := s.Service.GenerateMobileConfig(ctx, *p, accountId, true)
		if err != nil {
			return HandleError(c, err, ErrFailedToGenerateMobileConfig.Error())
		}
		res := new(responses.ShortLinkResponse)
		res.Link = link
		c.Set("Content-Type", "application/json")
		return c.Status(200).JSON(res)
	}
	return handler
}

// @Summary Download configuration profile for Apple devices from short link
// @Description Download configuration profile for Apple devices from short link
// @Tags Apple mobileconfig
// @Produce json
// @Param code path string true "short code"
// @Success 200 {object} string
// @Failure 400 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/short/{code} [get]
func (s *APIServer) downloadMobileConfigFromLink() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		code := c.Params("code")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dataStr, err := s.Cache.Get(ctx, apple.MobileConfigCacheKey(code))
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Configuration not found or expired")
		}
		data := []byte(dataStr)

		// Extract profile_id from the data (format: profile_id|mobileconfig_data)
		profileId := "profile"
		mobileConfigData := data
		if delimiterIndex := bytes.IndexByte(data, '|'); delimiterIndex > 0 {
			profileId = string(data[:delimiterIndex])
			mobileConfigData = data[delimiterIndex+1:]
		}

		// Set the correct content type for .mobileconfig files
		c.Set("Content-Type", "application/x-apple-aspen-config")

		// Use inline disposition with proper filename including profile_id
		filename := fmt.Sprintf("inline; filename=modDNS-%s.mobileconfig", profileId)
		c.Set("Content-Disposition", filename)

		return c.SendString(string(mobileConfigData))
	}
	return handler
}
