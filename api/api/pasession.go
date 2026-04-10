package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/model"
)

// PASessionCookie is the cookie name for the pre-auth session ID.
const PASessionCookie = "pa_session"

// @Summary Add pre-auth session
// @Description Add a pre-auth session to cache (called by preauth service)
// @Tags PASession
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.PASessionReq true "Pre-auth session request"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Router /api/v1/pasession/add [post]
func (s *APIServer) addPASession() fiber.Handler {
	return func(c *fiber.Ctx) error {
		req := new(requests.PASessionReq)
		if err := c.BodyParser(req); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, req, ErrInvalidRequestBody.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		session := &model.PASession{
			ID:        req.ID,
			Token:     req.Token,
			PreauthID: req.PreauthID,
		}

		if err := s.Service.AddPASession(c.Context(), session); err != nil {
			return HandleError(c, err, "failed to add pre-auth session")
		}

		return c.Status(200).JSON(fiber.Map{"message": "pre-auth session added"})
	}
}

// @Summary Rotate pre-auth session ID
// @Description Rotate pre-auth session ID and set new ID as cookie
// @Tags PASession
// @Accept json
// @Produce json
// @Param body body requests.RotatePASessionReq true "Rotate pre-auth session request"
// @Success 200
// @Failure 400 {object} ErrResponse
// @Router /api/v1/pasession/rotate [put]
func (s *APIServer) rotatePASession() fiber.Handler {
	return func(c *fiber.Ctx) error {
		req := new(requests.RotatePASessionReq)
		if err := c.BodyParser(req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "This signup link has expired."})
		}

		errMsgs := s.Validator.ValidateRequest(c, req, "")
		if len(errMsgs) > 0 {
			return c.Status(400).JSON(fiber.Map{"error": "This signup link has expired."})
		}

		newID, err := s.Service.RotatePASessionID(c.Context(), req.SessionID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "This signup link has expired."})
		}

		c.Cookie(&fiber.Cookie{
			Name:     PASessionCookie,
			Value:    newID,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteLaxMode,
			MaxAge:   900,
			Expires:  time.Now().Add(15 * time.Minute),
		})

		return c.SendStatus(fiber.StatusOK)
	}
}
