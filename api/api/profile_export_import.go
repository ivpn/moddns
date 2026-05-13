package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/service/profile"
)

// @Summary Export profiles
// @Description Export user's profiles as a downloadable JSON file
// @Tags Profile
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body requests.ExportRequest true "Export request"
// @Success 200 {object} profile.ExportEnvelope
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

		envelope, err := s.Service.Export(c.Context(), accountId, req.Scope, req.ProfileIds, req.CurrentPassword, req.ReauthToken)
		if err != nil {
			return handleExportImportError(c, err)
		}

		// specRef: E12, E13, E14, E15
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"moddns-export-%s.moddns.json\"", time.Now().UTC().Format("20060102T150405Z")))
		c.Set("Cache-Control", "no-store")
		c.Set("Pragma", "no-cache")

		return c.JSON(envelope, "application/vnd.moddns.export+json; charset=utf-8")
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

		envelope := mapImportPayload(req.Payload)

		result, err := s.Service.Import(c.Context(), accountId, req.Mode, envelope, req.CurrentPassword, req.ReauthToken)
		if err != nil {
			return handleExportImportError(c, err)
		}

		return c.JSON(result)
	}
}

// mapImportPayload converts the validated import DTO into the ExportEnvelope
// shape that ProfileService.Import() expects.  The two struct trees are
// intentionally parallel; this function is a pure field copy with no
// filtering logic — business rules live in the service layer.
//
// specRef: I1, V1-V6
func mapImportPayload(p *requests.ImportPayload) *profile.ExportEnvelope {
	if p == nil {
		return nil
	}

	env := &profile.ExportEnvelope{
		SchemaVersion: p.SchemaVersion,
		Kind:          p.Kind,
		ExportedAt:    p.ExportedAt,
	}

	if p.ExportedFrom != nil {
		env.ExportedFrom = &profile.ExportedFromInfo{
			Service:    p.ExportedFrom.Service,
			AppVersion: p.ExportedFrom.AppVersion,
		}
	}

	env.Profiles = make([]profile.ExportedProfile, 0, len(p.Profiles))
	for _, ip := range p.Profiles {
		ep := profile.ExportedProfile{
			Name:    ip.Name,
			Comment: ip.Comment,
		}
		if ip.Settings != nil {
			ep.Settings = mapImportSettings(ip.Settings)
		}
		env.Profiles = append(env.Profiles, ep)
	}

	return env
}

// mapImportSettings converts the per-profile settings section of the import
// DTO into the ExportedSettings shape the service expects.
func mapImportSettings(s *requests.ImportSettings) *profile.ExportedSettings {
	if s == nil {
		return nil
	}

	es := &profile.ExportedSettings{}

	if s.Privacy != nil {
		ep := &profile.ExportedPrivacy{
			DefaultRule:               s.Privacy.DefaultRule,
			BlocklistsSubdomainsRule:  s.Privacy.BlocklistsSubdomainsRule,
			CustomRulesSubdomainsRule: s.Privacy.CustomRulesSubdomainsRule,
			Blocklists:                s.Privacy.Blocklists,
			Services:                  s.Privacy.Services,
		}
		es.Privacy = ep
	}

	if s.Security != nil && s.Security.DNSSEC != nil {
		es.Security = &profile.ExportedSecurity{
			DNSSEC: &profile.ExportedDNSSEC{
				Enabled:   s.Security.DNSSEC.Enabled,
				SendDoBit: s.Security.DNSSEC.SendDoBit,
			},
		}
	}

	for _, cr := range s.CustomRules {
		es.CustomRules = append(es.CustomRules, profile.ExportedCustomRule{
			Action:  cr.Action,
			Value:   cr.Value,
			Comment: cr.Comment,
		})
	}

	if s.Logs != nil {
		es.Logs = &profile.ExportedLogs{
			Enabled:       s.Logs.Enabled,
			LogClientsIPs: s.Logs.LogClientsIPs,
			LogDomains:    s.Logs.LogDomains,
			Retention:     s.Logs.Retention,
		}
	}

	if s.Statistics != nil {
		es.Statistics = &profile.ExportedStatistics{
			Enabled: s.Statistics.Enabled,
		}
	}

	if s.Advanced != nil {
		es.Advanced = &profile.ExportedAdvanced{
			Recursor: s.Advanced.Recursor,
		}
	}

	return es
}

// handleExportImportError maps profile service errors to HTTP status codes for
// the export and import endpoints.  Reauth errors become 401; all other errors
// fall through to the shared HandleError helper.
//
// specRef: M5, M6
func handleExportImportError(c *fiber.Ctx, err error) error {
	switch err {
	case profile.ErrReauthRequired, profile.ErrReauthInvalid:
		return c.Status(fiber.StatusUnauthorized).JSON(ErrResponse{Error: err.Error()})
	default:
		return HandleError(c, err, err.Error())
	}
}
