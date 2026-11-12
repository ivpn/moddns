package responses

import (
	"time"

	"github.com/ivpn/dns/api/model"
)

type WebAuthnReauthFinishResponse struct {
	ReauthToken string    `json:"reauth_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Passkey is a credential as the account owner sees it in the passkey list.
type Passkey struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	// UserVerified tells whether the latest ceremony with this passkey verified the user.
	UserVerified bool `json:"user_verified"`
}

// NewPasskeys lists creds as passkeys, keeping their order.
func NewPasskeys(creds []model.Credential) []Passkey {
	passkeys := make([]Passkey, len(creds))
	for i, c := range creds {
		passkeys[i] = Passkey{ID: c.ID.Hex(), CreatedAt: c.CreatedAt, UserVerified: c.Flags.UserVerified}
	}
	return passkeys
}
