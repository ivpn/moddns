package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/passkey"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BeginRegistration starts the WebAuthn registration process.
// subID is the external subscription ID from the signup request; it is stored
// in the transient session so FinishRegistration can forward it to the webhook.
func (s *Service) BeginRegistration(ctx context.Context, account *model.Account, subID string) (*protocol.CredentialCreation, string, error) {
	// Create a user object with credentials
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: make([]webauthn.Credential, 0), // potential issue lies here
	}

	// Begin registration
	creation, sessionData, err := s.Webauthn.BeginRegistration(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin registration: %w", err)
	}

	// subID is kept so FinishRegistration can forward it to the webhook
	token, err := s.saveCeremony(ctx, &model.Ceremony{Type: model.CeremonyRegistration, AccountID: account.ID.Hex(), SubID: subID, SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}

	return creation, token, nil
}

// FinishRegistration completes the WebAuthn registration process
func (s *Service) FinishRegistration(ctx context.Context, token string, httpReq *http.Request, paSessionID string) error {
	ceremony, err := s.consumeCeremony(ctx, token, model.CeremonyRegistration, "", passkey.ErrFinishRegistration)
	if err != nil {
		return err
	}

	// Get account
	account, err := s.Store.GetAccount(ctx, ceremony.AccountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Finish registration
	credential, err := s.Webauthn.FinishRegistration(account, ceremony.SessionData, httpReq)
	if err != nil {
		return fmt.Errorf("failed to finish registration: %w", err)
	}

	// Save credential
	err = s.Store.SaveCredential(ctx, *credential, account.ID)
	if err != nil {
		return fmt.Errorf("failed to save credential: %w", err)
	}

	tokenHash := ""
	if sub, subErr := s.GetSubscription(ctx, account.ID.Hex()); subErr == nil && sub != nil {
		tokenHash = sub.TokenHash
	}

	// Use the external subID preserved in the session from BeginRegistration
	if err = s.CompleteRegistration(ctx, account, ceremony.SubID, paSessionID, tokenHash); err != nil {
		return fmt.Errorf("failed to complete registration: %w", err)
	}

	return nil
}

// BeginLogin starts the WebAuthn login process
func (s *Service) BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, string, error) {
	// Get account by email
	account, err := s.Store.GetAccountByEmail(ctx, email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get account: %w", err)
	}

	// Get credentials for the account
	credentials, err := s.GetCredentials(ctx, account.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get credentials: %w", err)
	}

	if len(credentials) == 0 {
		return nil, "", fmt.Errorf("no credentials found for account")
	}

	// Convert to WebAuthn credentials
	webauthnCreds := make([]webauthn.Credential, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
	}

	// Create user object
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: webauthnCreds,
	}

	// Begin login
	assertion, sessionData, err := s.Webauthn.BeginLogin(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin login: %w", err)
	}

	token, err := s.saveCeremony(ctx, &model.Ceremony{Type: model.CeremonyLogin, AccountID: account.ID.Hex(), SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}

	return assertion, token, nil
}

func (s *Service) BeginReauth(ctx context.Context, purpose, accountId string) (*protocol.CredentialAssertion, string, error) {
	// Rate limiting
	limiter := utils.IDLimiter{Cache: s.Cache, ID: accountId, Label: "reauth_" + purpose + "_begin", Max: s.Cfg.Service.IdLimiterMax, Exp: s.Cfg.Service.IdLimiterExpiration}
	if err := limiter.Tick(); err != nil {
		return nil, "", err
	}
	if !limiter.IsAllowed() {
		return nil, "", account.ErrReauthRateLimited
	}

	// Get account
	acc, err := s.GetAccount(ctx, accountId)
	if err != nil {
		return nil, "", err
	}

	if _, ok := reauthTokenType(purpose); !ok {
		return nil, "", fmt.Errorf("unsupported reauth purpose: %s", purpose)
	}

	credentials, err := s.GetCredentials(ctx, acc.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get credentials: %w", err)
	}
	if len(credentials) == 0 {
		return nil, "", fmt.Errorf("no credentials found for account")
	}

	webauthnCreds := make([]webauthn.Credential, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
	}

	user := &passkey.WebAuthnUser{
		Account:     acc,
		Credentials: webauthnCreds,
	}

	options, sessionData, err := s.Webauthn.BeginLogin(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin reauth login: %w", err)
	}

	token, err := s.saveCeremony(ctx, &model.Ceremony{Type: model.CeremonyReauth, AccountID: acc.ID.Hex(), Purpose: purpose, SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}

	return options, token, nil
}

// reauthTokenTypes maps each passkey reauth purpose to the token type that the
// operation it protects consumes.
var reauthTokenTypes = map[string]string{
	"email_change":     auth.TokenTypeReauthEmailChange,
	"account_deletion": auth.TokenTypeReauthAccountDeletion,
	"profile_export":   auth.TokenTypeReauthProfileExport,
	"profile_import":   auth.TokenTypeReauthProfileImport,
	"password_set":     auth.TokenTypeReauthPasswordSet,
	"passkey_add":      auth.TokenTypeReauthPasskeyAdd,
	"passkey_delete":   auth.TokenTypeReauthPasskeyDelete,
}

func reauthTokenType(purpose string) (string, bool) {
	tokenType, ok := reauthTokenTypes[purpose]
	return tokenType, ok
}

// FinishReauth completes the WebAuthn reauthentication process and issues a reauth token
func (s *Service) FinishReauth(ctx context.Context, accountID, tmpToken string, httpReq *http.Request) (*model.Token, error) {
	acc, purpose, err := s.finishAssertion(ctx, tmpToken, model.CeremonyReauth, accountID, passkey.ErrFinishReauth, httpReq)
	if err != nil {
		return nil, err
	}

	tokenType, ok := reauthTokenType(purpose)
	if !ok {
		return nil, account.ErrInvalidReauthToken
	}

	reauthTok, err := auth.NewToken(tokenType)
	if err != nil {
		return nil, err
	}
	acc.Tokens = append(acc.Tokens, *reauthTok)
	if _, err = s.Store.UpdateAccount(ctx, acc); err != nil {
		return nil, err
	}
	return reauthTok, nil
}

// FinishLogin completes the WebAuthn login process
func (s *Service) FinishLogin(ctx context.Context, tmpToken string, httpReq *http.Request, saveSession bool) (*model.Account, string, string, error) {
	account, purpose, err := s.finishAssertion(ctx, tmpToken, model.CeremonyLogin, "", passkey.ErrFinishLogin, httpReq)
	if err != nil {
		return nil, "", purpose, err
	}

	// Save the session
	sessionData := webauthn.SessionData{
		UserID:  account.WebAuthnID(),
		Expires: time.Now().Add(s.Cfg.API.SessionExpirationTime), // TODO: improve
	}

	if saveSession {
		token, err := model.GenSessionToken()
		if err != nil {
			return nil, "", purpose, fmt.Errorf("failed to generate session token: %w", err)
		}

		err = s.SaveSession(ctx, sessionData, token, account.ID.Hex())
		if err != nil {
			return nil, "", purpose, fmt.Errorf("failed to save session: %w", err)
		}

		return account, token, "", nil
	}
	return account, "", purpose, nil
}

// finishAssertion verifies a WebAuthn assertion against the state of the given
// ceremony and returns the account it authenticates.
func (s *Service) finishAssertion(ctx context.Context, tmpToken, ceremonyType, accountID string, mismatchErr error, httpReq *http.Request) (*model.Account, string, error) {
	ceremony, err := s.consumeCeremony(ctx, tmpToken, ceremonyType, accountID, mismatchErr)
	if err != nil {
		return nil, "", err
	}
	purpose := ceremony.Purpose

	// Get account
	account, err := s.Store.GetAccount(ctx, ceremony.AccountID)
	if err != nil {
		return nil, purpose, fmt.Errorf("failed to get account: %w", err)
	}

	// Get credentials
	credentials, err := s.Store.GetCredentials(ctx, account.ID)
	if err != nil {
		return nil, purpose, fmt.Errorf("failed to get credentials: %w", err)
	}

	// Convert to WebAuthn credentials
	webauthnCreds := make([]webauthn.Credential, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
	}

	// Create user object
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: webauthnCreds,
	}

	// Finish login
	credential, err := s.Webauthn.FinishLogin(user, ceremony.SessionData, httpReq)
	if err != nil {
		return nil, purpose, fmt.Errorf("failed to finish login: %w", err)
	}

	// Check for clone warning
	if credential.Authenticator.CloneWarning {
		return nil, purpose, fmt.Errorf("credential clone warning: potential security issue")
	}

	// Update credential with new sign count
	err = s.Store.UpdateCredential(ctx, *credential, account.ID)
	if err != nil {
		return nil, purpose, fmt.Errorf("failed to update credential: %w", err)
	}

	return account, purpose, nil
}

// BeginAddPasskey starts the process of adding a new passkey to an existing account
// The account owner must first verify with the current password (plus TOTP when enabled)
// or a passkey reauth token issued for passkey_add, so a session alone cannot add a credential.
func (s *Service) BeginAddPasskey(ctx context.Context, accountID string, currentPassword, reauthToken *string, mfa *model.MfaData) (*protocol.CredentialCreation, string, error) {
	account, err := reauth.Verify(ctx, s.Store, s.AccountServicer, reauth.Params{
		AccountId:        accountID,
		TokenType:        auth.TokenTypeReauthPasskeyAdd,
		Password:         currentPassword,
		ReauthToken:      reauthToken,
		Mfa:              mfa,
		PersistOnConsume: true,
	})
	if err != nil {
		return nil, "", err
	}

	// Get existing credentials for the account
	credentials, err := s.GetCredentials(ctx, account.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get credentials: %w", err)
	}

	// Convert to WebAuthn credentials
	webauthnCreds := make([]webauthn.Credential, len(credentials))
	excluded := make([]protocol.CredentialDescriptor, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
		excluded[i] = webauthnCreds[i].Descriptor()
	}

	// Create user object with existing credentials
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: webauthnCreds,
	}

	// An authenticator keeps one discoverable credential per (RP ID, user handle) and
	// replaces it on a new registration (W3C WebAuthn L3 §6.3.2 step 7.4.4). Excluding the
	// account's credentials makes it refuse instead (§6.3.2 step 3, §5.4).
	creation, sessionData, err := s.Webauthn.BeginRegistration(user, webauthn.WithExclusions(excluded))
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin add passkey: %w", err)
	}

	token, err := s.saveCeremony(ctx, &model.Ceremony{Type: model.CeremonyAddPasskey, AccountID: account.ID.Hex(), SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}

	return creation, token, nil
}

// FinishAddPasskey completes the process of adding a new passkey to an existing account
func (s *Service) FinishAddPasskey(ctx context.Context, accountID, token string, httpReq *http.Request) error {
	ceremony, err := s.consumeCeremony(ctx, token, model.CeremonyAddPasskey, accountID, passkey.ErrFinishAddPasskey)
	if err != nil {
		return err
	}

	// Get account
	account, err := s.Store.GetAccount(ctx, ceremony.AccountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Get existing credentials
	credentials, err := s.GetCredentials(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Convert to WebAuthn credentials
	webauthnCreds := make([]webauthn.Credential, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
	}

	// Create user object
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: webauthnCreds,
	}

	// Finish registration
	credential, err := s.Webauthn.FinishRegistration(user, ceremony.SessionData, httpReq)
	if err != nil {
		return err
	}

	// Save the new credential
	err = s.SaveCredential(ctx, *credential, account.ID)
	if err != nil {
		return err
	}
	return nil
}

// GetPasskeysForAccount retrieves passkeys for an account (excluding sensitive data)
func (s *Service) GetPasskeysForAccount(ctx context.Context, accountID primitive.ObjectID) ([]model.Credential, error) {
	// Get credentials from repository
	credentials, err := s.GetCredentials(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	return credentials, nil
}

// DeletePasskeyByID deletes a passkey by its ID, ensuring it belongs to the account
// The account owner must verify with the current password (plus TOTP when enabled) or a
// passkey reauth token issued for passkey_delete. Ownership is checked first so a token is
// not consumed by a request that cannot succeed.
func (s *Service) DeletePasskeyByID(ctx context.Context, credentialID primitive.ObjectID, accountID primitive.ObjectID, currentPassword, reauthToken *string, mfa *model.MfaData) error {
	// Verify the credential belongs to the account before deletion
	credential, err := s.Store.GetCredentialByID(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("failed to get credential: %w", err)
	}

	if credential.AccountID != accountID {
		return fmt.Errorf("credential does not belong to account")
	}

	if err := s.ensureAnotherSignInMethod(ctx, accountID); err != nil {
		return err
	}

	if _, err := reauth.Verify(ctx, s.Store, s.AccountServicer, reauth.Params{
		AccountId:        accountID.Hex(),
		TokenType:        auth.TokenTypeReauthPasskeyDelete,
		Password:         currentPassword,
		ReauthToken:      reauthToken,
		Mfa:              mfa,
		PersistOnConsume: true,
	}); err != nil {
		return err
	}

	// Delete the credential
	err = s.DeleteCredentialByID(ctx, credentialID, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
}

// ensureAnotherSignInMethod refuses removing a passkey that is the account's only way to
// sign in: the last credential on an account without a password.
func (s *Service) ensureAnotherSignInMethod(ctx context.Context, accountID primitive.ObjectID) error {
	count, err := s.Store.GetCredentialsCount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to count credentials: %w", err)
	}
	if count > 1 {
		return nil
	}
	acc, err := s.Store.GetAccountById(ctx, accountID.Hex())
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	if acc.Password == nil {
		return passkey.ErrLastSignInMethod
	}
	return nil
}

// saveCeremony stores the state of a ceremony started by a begin call and returns
// the token that identifies it.
func (s *Service) saveCeremony(ctx context.Context, ceremony *model.Ceremony) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate ceremony token: %w", err)
	}
	if err := s.Cache.SaveCeremony(ctx, token, ceremony, model.CeremonyTimeout); err != nil {
		return "", fmt.Errorf("failed to save ceremony: %w", err)
	}
	return token, nil
}

// consumeCeremony returns the state saved by a ceremony's begin call. The state is
// removed as it is read, whether or not the finish call succeeds. State left by a
// different ceremony, or by another account when accountID is set, yields mismatchErr.
func (s *Service) consumeCeremony(ctx context.Context, token, ceremonyType, accountID string, mismatchErr error) (*model.Ceremony, error) {
	if token == "" {
		return nil, errors.New("missing ceremony token")
	}
	ceremony, err := s.Cache.ConsumeCeremony(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get ceremony: %w", err)
	}
	if ceremony.Type != ceremonyType || (accountID != "" && ceremony.AccountID != accountID) {
		return nil, mismatchErr
	}
	return ceremony, nil
}

// generateToken generates a secure random token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
