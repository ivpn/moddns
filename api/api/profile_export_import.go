package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// @Summary Export profiles
// @Description Export user's profiles as a downloadable JSON file
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.ExportRequest true "Export request"
// @Success 200 {object} model.ExportEnvelope
// @Header 200 {string} Content-Type "application/vnd.moddns.export+json; charset=utf-8"
// @Header 200 {string} Content-Disposition "attachment with a filename of the form moddns-export-<UTC>.moddns.json (contains no PII)"
// @Header 200 {string} Cache-Control "no-store; the export contains sensitive data"
// @Header 200 {string} Pragma "no-cache"
// @Header 200 {integer} X-modDNS-Export-Truncated "count of profiles whose custom rules were truncated to the 1000 per-profile export limit; omitted when nothing was truncated"
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 404 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Router /api/v1/profiles/export [post]
func (s *APIServer) exportProfiles() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountId := auth.GetAccountID(c)

		req := new(requests.ExportRequest)
		dec := json.NewDecoder(bytes.NewReader(c.Body()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(req); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, req, "Failed to export profiles")
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		if err := req.Validate(); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		if req.CurrentPassword != nil && req.ReauthToken != nil {
			return HandleError(c, ErrInvalidRequestBody, "provide only one of current_password or reauth_token")
		}

		envelope, err := s.Service.Export(c.Context(), accountId, req.Scope, req.ProfileIds, req.CurrentPassword, req.ReauthToken, auth.GetMfaData(c))
		if err != nil {
			return HandleError(c, err, err.Error())
		}

		// Cap each profile's custom rules to the per-profile export limit
		// (oldest-first, preserving insertion order) so the file always re-imports
		// cleanly regardless of how many rules a profile accumulated. Signal how
		// many profiles were trimmed via a header so the UI can warn the user.
		truncatedProfiles := 0
		for i := range envelope.Profiles {
			settings := envelope.Profiles[i].Settings
			if settings != nil && len(settings.CustomRules) > model.ExportedCustomRulesLimit {
				settings.CustomRules = settings.CustomRules[:model.ExportedCustomRulesLimit]
				truncatedProfiles++
			}
		}
		if truncatedProfiles > 0 {
			c.Set("X-modDNS-Export-Truncated", strconv.Itoa(truncatedProfiles))
		}

		// specRef: E12, E13, E14, E15
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"moddns-export-%s.moddns.json\"", time.Now().UTC().Format("20060102T150405Z")))
		c.Set("Cache-Control", "no-store")
		c.Set("Pragma", "no-cache")

		// Pretty-print with 2-space indent + trailing newline. The export is a
		// downloadable file (Content-Disposition: attachment) that users open
		// in a text editor; readability is the whole point. Whitespace is
		// insignificant on the import side (json.Decoder ignores it).
		body, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return HandleError(c, err, err.Error())
		}
		body = append(body, '\n')
		c.Set("Content-Type", "application/vnd.moddns.export+json; charset=utf-8")
		return c.Send(body)
	}
}

// @Summary Import profiles
// @Description Import profiles from a previously exported JSON file
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param X-modDNS-Import header string true "CSRF guard header — must equal \"confirm\""
// @Param body body requests.ImportRequest true "Import request"
// @Success 200 {object} profile.ImportResult
// @Failure 400 {object} ErrResponse
// @Failure 401 {object} ErrResponse
// @Failure 415 {object} ErrResponse
// @Failure 429 {object} ErrResponse
// @Router /api/v1/profiles/import [post]
func (s *APIServer) importProfiles() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// specRef: I6 — reject compressed bodies before reading them.
		if c.Get("Content-Encoding") == "gzip" {
			return c.Status(fiber.StatusUnsupportedMediaType).JSON(ErrResponse{Error: "gzip content-encoding not supported"})
		}

		// specRef: I7 — require JSON content-type; reject other media types with
		// 415 rather than letting them fall through to a malformed-body 400.
		ct := c.Get("Content-Type")
		if ct != "" && !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
			return c.Status(fiber.StatusUnsupportedMediaType).JSON(ErrResponse{Error: "content-type must be application/json"})
		}

		// specRef: I4 — CSRF guard: simple-form cross-site requests cannot set
		// custom headers, so this single check eliminates the CSRF surface.
		if c.Get("X-modDNS-Import") != "confirm" {
			return HandleError(c, ErrInvalidRequestBody, "missing X-modDNS-Import confirm header")
		}

		accountId := auth.GetAccountID(c)

		req := new(requests.ImportRequest)
		dec := json.NewDecoder(bytes.NewReader(c.Body()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(req); err != nil {
			return HandleError(c, ErrInvalidRequestBody, err.Error())
		}

		errMsgs := s.Validator.ValidateRequest(c, req, "Failed to import profiles")
		if len(errMsgs) > 0 {
			return HandleError(c, ErrInvalidRequestBody, strings.Join(errMsgs, " and "))
		}

		if req.CurrentPassword != nil && req.ReauthToken != nil {
			return HandleError(c, ErrInvalidRequestBody, "provide only one of current_password or reauth_token")
		}

		result, err := s.Service.Import(c.Context(), accountId, req.Mode, req.Payload, req.CurrentPassword, req.ReauthToken, auth.GetMfaData(c))
		if err != nil {
			return HandleError(c, err, err.Error())
		}

		return c.JSON(result)
	}
}
