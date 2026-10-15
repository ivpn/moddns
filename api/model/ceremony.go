package model

import (
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// CeremonyTimeout lies within the range recommended for ceremonies that verify
// the user (W3C WebAuthn Level 3 §15.1).
const CeremonyTimeout = 5 * time.Minute

// Ceremony is the server-side state of a WebAuthn ceremony between its begin and
// finish calls. It is kept in the cache, apart from login sessions, so a ceremony
// token can never be presented as a login session.
type Ceremony struct {
	AccountID   string               `json:"account_id"`
	Purpose     string               `json:"purpose,omitempty"`
	SubID       string               `json:"sub_id,omitempty"`
	SessionData webauthn.SessionData `json:"session_data"`
}
