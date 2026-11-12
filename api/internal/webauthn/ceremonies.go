package webauthn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"

	"github.com/ivpn/dns/api/model"
)

// UserLoader returns the user a finish call verifies against, given the account
// the ceremony was begun for.
type UserLoader func(ctx context.Context, accountID string) (User, error)

// Result is a verified ceremony.
type Result struct {
	AccountID string
	Purpose   string
	SubID     string
	// Credential is the new credential to store after registration and add passkey.
	Credential *model.Credential
	// CredentialID and SignCount identify the credential a login or reauth asserted
	// and its new signature counter.
	CredentialID []byte
	SignCount    uint32
}

// requireUserVerification makes a new credential verify the user, so a passkey sign-in
// counts as two factors. The relying party sets this per operation and the library
// rejects a response without the UV flag (W3C WebAuthn L3 §5.8.6, §7.1 step 16).
// Login and reauth keep the library default.
var requireUserVerification = gowebauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
	UserVerification: protocol.VerificationRequired,
})

// BeginRegistration starts the signup ceremony. subID is returned by FinishRegistration.
func (rp *RelyingParty) BeginRegistration(ctx context.Context, user User, subID string) (*CredentialCreation, string, error) {
	creation, sessionData, err := rp.lib.BeginRegistration(newLibUser(user), requireUserVerification)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin registration: %w", err)
	}
	token, err := rp.save(ctx, &ceremony{Type: CeremonyRegistration, AccountID: user.AccountID.Hex(), SubID: subID, SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}
	return creation, token, nil
}

// BeginAddPasskey starts the ceremony that adds a credential to user.
func (rp *RelyingParty) BeginAddPasskey(ctx context.Context, user User) (*CredentialCreation, string, error) {
	libUser := newLibUser(user)
	excluded := make([]protocol.CredentialDescriptor, len(libUser.credentials))
	for i, cred := range libUser.credentials {
		excluded[i] = cred.Descriptor()
	}

	// An authenticator keeps one discoverable credential per (RP ID, user handle) and
	// replaces it on a new registration (W3C WebAuthn L3 §6.3.2 step 7.4.4). Excluding the
	// account's credentials makes it refuse instead (§6.3.2 step 3, §5.4).
	creation, sessionData, err := rp.lib.BeginRegistration(libUser, gowebauthn.WithExclusions(excluded), requireUserVerification)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin add passkey: %w", err)
	}
	token, err := rp.save(ctx, &ceremony{Type: CeremonyAddPasskey, AccountID: user.AccountID.Hex(), SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}
	return creation, token, nil
}

// BeginLogin starts the login ceremony for user.
func (rp *RelyingParty) BeginLogin(ctx context.Context, user User) (*CredentialAssertion, string, error) {
	assertion, sessionData, err := rp.lib.BeginLogin(newLibUser(user))
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin login: %w", err)
	}
	token, err := rp.save(ctx, &ceremony{Type: CeremonyLogin, AccountID: user.AccountID.Hex(), SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}
	return assertion, token, nil
}

// BeginReauth starts a reauthentication ceremony for user. purpose is returned by FinishReauth.
func (rp *RelyingParty) BeginReauth(ctx context.Context, user User, purpose string) (*CredentialAssertion, string, error) {
	assertion, sessionData, err := rp.lib.BeginLogin(newLibUser(user))
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin reauth login: %w", err)
	}
	token, err := rp.save(ctx, &ceremony{Type: CeremonyReauth, AccountID: user.AccountID.Hex(), Purpose: purpose, SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}
	return assertion, token, nil
}

// FinishRegistration verifies the browser's response to BeginRegistration.
func (rp *RelyingParty) FinishRegistration(ctx context.Context, token string, load UserLoader, req *http.Request) (*Result, error) {
	c, err := rp.consume(ctx, token, CeremonyRegistration, nil)
	if err != nil {
		return nil, err
	}
	user, err := load(ctx, c.AccountID)
	if err != nil {
		return nil, err
	}
	cred, err := rp.lib.FinishRegistration(newLibUser(user), c.SessionData, req)
	if err != nil {
		return nil, verificationError("failed to finish registration", err)
	}
	return &Result{AccountID: c.AccountID, SubID: c.SubID, Credential: toRecord(cred, user.AccountID)}, nil
}

// FinishAddPasskey verifies the browser's response to BeginAddPasskey begun by accountID.
func (rp *RelyingParty) FinishAddPasskey(ctx context.Context, token, accountID string, load UserLoader, req *http.Request) (*Result, error) {
	c, err := rp.consume(ctx, token, CeremonyAddPasskey, &accountID)
	if err != nil {
		return nil, err
	}
	user, err := load(ctx, c.AccountID)
	if err != nil {
		return nil, err
	}
	cred, err := rp.lib.FinishRegistration(newLibUser(user), c.SessionData, req)
	if err != nil {
		return nil, verificationError("failed to finish add passkey", err)
	}
	return &Result{AccountID: c.AccountID, Credential: toRecord(cred, user.AccountID)}, nil
}

// FinishLogin verifies the browser's response to BeginLogin.
func (rp *RelyingParty) FinishLogin(ctx context.Context, token string, load UserLoader, req *http.Request) (*Result, error) {
	return rp.finishAssertion(ctx, token, CeremonyLogin, nil, load, req)
}

// FinishReauth verifies the browser's response to BeginReauth begun by accountID.
func (rp *RelyingParty) FinishReauth(ctx context.Context, token, accountID string, load UserLoader, req *http.Request) (*Result, error) {
	return rp.finishAssertion(ctx, token, CeremonyReauth, &accountID, load, req)
}

func (rp *RelyingParty) finishAssertion(ctx context.Context, token string, ceremonyType CeremonyType, accountID *string, load UserLoader, req *http.Request) (*Result, error) {
	c, err := rp.consume(ctx, token, ceremonyType, accountID)
	if err != nil {
		return nil, err
	}
	user, err := load(ctx, c.AccountID)
	if err != nil {
		return nil, err
	}
	cred, err := rp.lib.FinishLogin(newLibUser(user), c.SessionData, req)
	if err != nil {
		return nil, verificationError("failed to finish login", err)
	}
	if cred.Authenticator.CloneWarning {
		return nil, ErrCloneDetected
	}
	return &Result{AccountID: c.AccountID, Purpose: c.Purpose, CredentialID: cred.ID, SignCount: cred.Authenticator.SignCount}, nil
}

// verificationError marks a refusal by the WebAuthn library with ErrVerificationFailed.
// The library reports a failed relying-party check as a *protocol.Error, except a
// clientDataJSON in an assertion that is not JSON, which it returns as the decoder's error.
func verificationError(msg string, err error) error {
	var (
		protoErr  *protocol.Error
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)
	if errors.As(err, &protoErr) || errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return fmt.Errorf("%s: %w: %w", msg, ErrVerificationFailed, err)
	}
	return fmt.Errorf("%s: %w", msg, err)
}
