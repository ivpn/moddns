package requests

import (
	"errors"
	"fmt"

	"github.com/ivpn/dns/api/model"
)

// ErrInvalidScopeSelection is returned when the scope and profileIds fields are
// mutually inconsistent (spec rows E6, E8).
var ErrInvalidScopeSelection = errors.New("profileIds must be empty when scope=all")

// ExportRequest is the request body for POST /api/v1/profiles/export.
// Exactly one of CurrentPassword or ReauthToken must be provided (spec rows E3, M4).
// specRef: E2, E5–E11
type ExportRequest struct {
	Scope           string   `json:"scope"                        validate:"required,oneof=all selected"`
	ProfileIds      []string `json:"profileIds,omitempty"`
	CurrentPassword *string  `json:"current_password,omitempty"   validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string  `json:"reauth_token,omitempty"       validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}

// Validate enforces the cross-field invariant between Scope and ProfileIds.
// specRef: E6, E8
func (r *ExportRequest) Validate() error {
	if r.Scope == "all" && len(r.ProfileIds) > 0 {
		return fmt.Errorf("scope=all: %w", ErrInvalidScopeSelection)
	}
	if r.Scope == "selected" && len(r.ProfileIds) == 0 {
		return fmt.Errorf("scope=selected requires at least one profileId: %w", ErrInvalidScopeSelection)
	}
	return nil
}

// ImportRequest is the request body for POST /api/v1/profiles/import.
// Exactly one of CurrentPassword or ReauthToken must be provided (spec rows I2, M4).
//
// Payload is the same envelope type the export endpoint emits. Validation tags
// on model.ExportEnvelope and its nested types are enforced recursively by
// s.Validator.ValidateRequest.
//
// specRef: I1, I8–I11, V1–V15
type ImportRequest struct {
	Mode            string                `json:"mode"                         validate:"required,oneof=create_new"`
	Payload         *model.ExportEnvelope `json:"payload"                      validate:"required"`
	CurrentPassword *string               `json:"current_password,omitempty"   validate:"excluded_with=ReauthToken,omitempty,min=1"`
	ReauthToken     *string               `json:"reauth_token,omitempty"       validate:"excluded_with=CurrentPassword,omitempty,min=1"`
}
