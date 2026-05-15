package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/internal/auth"
)

// @Summary Generate DNS Stamps for a modDNS profile
// @Description Returns DoH, DoT, and DoQ sdns:// strings for the given profile,
// @Description optionally scoped to a specific device label. Stamps are
// @Description consumed by clients that don't expose separate hostname/path
// @Description fields (UniFi Network, dnscrypt-proxy, AdGuard Home upstreams, etc.).
// @Tags DNS Stamps
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.DNSStampReq true "Generate DNS stamp request"
// @Success 200 {object} responses.DNSStampResponse
// @Failure 400 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 500 {object} ErrResponse
// @Router /api/v1/dnsstamp [post]
func (s *APIServer) generateDNSStamps() fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := new(requests.DNSStampReq)
		if err := c.BodyParser(p); err != nil {
			return HandleError(c, err, ErrInvalidRequestBody.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, p, ErrFailedToGenerateDNSStamp.Error())
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		// Ownership check — identical pattern to mobileconfig.go.
		accountId := auth.GetAccountID(c)
		if _, err := s.Service.GetProfile(c.Context(), accountId, p.ProfileId); err != nil {
			return HandleError(c, err, ErrFailedToGenerateDNSStamp.Error())
		}

		resp, err := s.Service.GenerateStamps(c.Context(), *p)
		if err != nil {
			return HandleError(c, err, ErrFailedToGenerateDNSStamp.Error())
		}

		c.Set("Content-Type", "application/json")
		return c.Status(fiber.StatusOK).JSON(responses.DNSStampResponse(resp))
	}
}
