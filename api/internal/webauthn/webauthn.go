// Package webauthn runs the WebAuthn ceremonies of the API: relying-party settings,
// ceremony state between begin and finish, verification and credential records.
// It knows nothing about accounts, subscriptions, sessions or reauth tokens; the
// service layer supplies the user and decides what a verified ceremony grants.
//
// It is the only package that may import github.com/go-webauthn/webauthn.
package webauthn

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

// CeremonyTimeout lies within the range recommended for ceremonies that verify
// the user (W3C WebAuthn Level 3 §15.1).
const CeremonyTimeout = 5 * time.Minute

// CredentialCreation and CredentialAssertion are the options a begin call returns
// for the browser's navigator.credentials.create() and get().
type (
	CredentialCreation  = protocol.CredentialCreation
	CredentialAssertion = protocol.CredentialAssertion
)

var (
	// ErrCeremonyMismatch is returned when a finish call gets state begun by another
	// ceremony type or, where the caller names one, by another account.
	ErrCeremonyMismatch = errors.New("ceremony state belongs to another ceremony")
	// ErrCloneDetected is returned when the signature counter did not increase
	// (W3C WebAuthn Level 3 §6.1.1, §7.2 step 22).
	ErrCloneDetected = errors.New("credential clone warning: potential security issue")
)

// Config holds the relying-party settings.
type Config struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

// RelyingParty runs the ceremonies and keeps their state in a CeremonyStore.
type RelyingParty struct {
	lib   *gowebauthn.WebAuthn
	store CeremonyStore
}

// New returns a relying party for cfg that keeps ceremony state in store.
func New(cfg Config, store CeremonyStore) (*RelyingParty, error) {
	lib, err := gowebauthn.New(&gowebauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
		Timeouts: gowebauthn.TimeoutsConfig{
			Login:        gowebauthn.TimeoutConfig{Enforce: true, Timeout: CeremonyTimeout, TimeoutUVD: CeremonyTimeout},
			Registration: gowebauthn.TimeoutConfig{Enforce: true, Timeout: CeremonyTimeout, TimeoutUVD: CeremonyTimeout},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize WebAuthn: %w", err)
	}
	return &RelyingParty{lib: lib, store: store}, nil
}
