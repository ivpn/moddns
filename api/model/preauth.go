package model

import "time"

// Preauth represents an entry returned by the external preauth service.
// It contains subscription entitlement data validated via ZLA token hash.
type Preauth struct {
	ID          string    `json:"id"`
	TokenHash   string    `json:"token_hash"`
	IsActive    bool      `json:"is_active"`
	ActiveUntil time.Time `json:"active_until"`
	Tier        string    `json:"tier"`
}
