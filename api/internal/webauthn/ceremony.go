package webauthn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

// CeremonyType names a ceremony; a finish call only accepts state from its own begin call.
type CeremonyType string

const (
	CeremonyRegistration CeremonyType = "registration"
	CeremonyAddPasskey   CeremonyType = "add_passkey"
	CeremonyLogin        CeremonyType = "login"
	CeremonyReauth       CeremonyType = "reauth"
)

// CeremonyStore keeps ceremony state between the begin and finish calls. Consume
// must read and delete in one step, so each challenge is used at most once
// (W3C WebAuthn Level 3 §13.4.3).
type CeremonyStore interface {
	SaveCeremony(ctx context.Context, token string, state []byte, expiresIn time.Duration) error
	ConsumeCeremony(ctx context.Context, token string) ([]byte, error)
}

// ceremony is the stored state. Instances of different builds share it during a
// rolling deploy, so its JSON shape must stay stable.
type ceremony struct {
	Type        CeremonyType           `json:"type"`
	AccountID   string                 `json:"account_id"`
	Purpose     string                 `json:"purpose,omitempty"`
	SubID       string                 `json:"sub_id,omitempty"`
	SessionData gowebauthn.SessionData `json:"session_data"`
}

// save stores c and returns the token that identifies it.
func (rp *RelyingParty) save(ctx context.Context, c *ceremony) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate ceremony token: %w", err)
	}
	state, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to encode ceremony: %w", err)
	}
	if err := rp.store.SaveCeremony(ctx, token, state, CeremonyTimeout); err != nil {
		return "", fmt.Errorf("failed to save ceremony: %w", err)
	}
	return token, nil
}

// consume returns the state saved under token by a begin call of ceremonyType. The
// state is removed as it is read, whether or not the finish call succeeds. State
// from another ceremony type, or, for a ceremony bound to accountID, from another
// account, yields ErrCeremonyMismatch. An empty accountID matches no account.
func (rp *RelyingParty) consume(ctx context.Context, token string, ceremonyType CeremonyType, accountID *string) (*ceremony, error) {
	if token == "" {
		return nil, errors.New("missing ceremony token")
	}
	state, err := rp.store.ConsumeCeremony(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get ceremony: %w", err)
	}
	var c ceremony
	if err := json.Unmarshal(state, &c); err != nil {
		return nil, fmt.Errorf("failed to decode ceremony: %w", err)
	}
	if c.Type != ceremonyType || (accountID != nil && (*accountID == "" || c.AccountID != *accountID)) {
		return nil, ErrCeremonyMismatch
	}
	return &c, nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
