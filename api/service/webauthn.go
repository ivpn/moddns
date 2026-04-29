package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/passkey"
	"github.com/rs/zerolog/log"
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

	// Generate session token
	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Save session (includes subID so FinishRegistration can forward it to the webhook)
	err = s.SaveSession(ctx, *sessionData, token, account.ID.Hex(), "", subID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to save session: %w", err)
	}

	return creation, token, nil
}

// FinishRegistration completes the WebAuthn registration process
func (s *Service) FinishRegistration(ctx context.Context, token string, httpReq *http.Request, paSessionID string) error {
	// Get session
	session, exists, err := s.GetSession(ctx, token)
	if err != nil || !exists {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Get account
	account, err := s.Store.GetAccount(ctx, session.AccountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Finish registration
	credential, err := s.Webauthn.FinishRegistration(account, session.SessionData, httpReq)
	if err != nil {
		return fmt.Errorf("failed to finish registration: %w", err)
	}

	// Save credential
	err = s.Store.SaveCredential(ctx, *credential, account.ID)
	if err != nil {
		return fmt.Errorf("failed to save credential: %w", err)
	}

	// Use the external subID preserved in the session from BeginRegistration
	if err = s.CompleteRegistration(ctx, account, session.SubID, paSessionID); err != nil {
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

	// Generate session token
	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Save session
	err = s.SaveSession(ctx, *sessionData, token, account.ID.Hex(), "", "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to save session: %w", err)
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

	if purpose != "email_change" && purpose != "account_deletion" {
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

	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}

	if err = s.SaveSession(ctx, *sessionData, token, acc.ID.Hex(), purpose, ""); err != nil {
		return nil, "", fmt.Errorf("failed to save reauth session: %w", err)
	}

	return options, token, nil
}

// FinishReauth completes the WebAuthn reauthentication process and issues a reauth token
func (s *Service) FinishReauth(ctx context.Context, tmpToken string, httpReq *http.Request) (*model.Token, error) {
	acc, _, purpose, err := s.FinishLogin(ctx, tmpToken, httpReq, false)
	if err != nil {
		return nil, err
	}

	var tokenType string
	switch purpose {
	case "email_change":
		tokenType = auth.TokenTypeReauthEmailChange
	case "account_deletion":
		tokenType = auth.TokenTypeReauthAccountDeletion
	default:
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
	// Get session
	session, exists, err := s.GetSession(ctx, tmpToken)
	if err != nil || !exists {
		return nil, "", "", fmt.Errorf("failed to get session: %w", err)
	}
	purpose := session.Purpose

	// Delete the temporary session (log error if deletion fails)
	defer func() {
		if err := s.DeleteSession(ctx, tmpToken); err != nil {
			// best-effort cleanup; log but ignore
			log.Err(err).Str("token", tmpToken).Msg("failed to delete temporary webauthn session")
		}
	}()

	// Get account
	account, err := s.Store.GetAccount(ctx, session.AccountID)
	if err != nil {
		return nil, "", purpose, fmt.Errorf("failed to get account: %w", err)
	}

	// Get credentials
	credentials, err := s.Store.GetCredentials(ctx, account.ID)
	if err != nil {
		return nil, "", purpose, fmt.Errorf("failed to get credentials: %w", err)
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
	credential, err := s.Webauthn.FinishLogin(user, session.SessionData, httpReq)
	if err != nil {
		return nil, "", purpose, fmt.Errorf("failed to finish login: %w", err)
	}

	// Check for clone warning
	if credential.Authenticator.CloneWarning {
		return nil, "", purpose, fmt.Errorf("credential clone warning: potential security issue")
	}

	// Update credential with new sign count
	err = s.Store.UpdateCredential(ctx, *credential, account.ID)
	if err != nil {
		return nil, "", purpose, fmt.Errorf("failed to update credential: %w", err)
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

		err = s.SaveSession(ctx, sessionData, token, account.ID.Hex(), "", "")
		if err != nil {
			return nil, "", purpose, fmt.Errorf("failed to save session: %w", err)
		}

		return account, token, "", nil
	}
	return account, "", purpose, nil
}

// BeginAddPasskey starts the process of adding a new passkey to an existing account
func (s *Service) BeginAddPasskey(ctx context.Context, account *model.Account) (*protocol.CredentialCreation, string, error) {
	// Get existing credentials for the account
	credentials, err := s.GetCredentials(ctx, account.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get credentials: %w", err)
	}

	// Convert to WebAuthn credentials
	webauthnCreds := make([]webauthn.Credential, len(credentials))
	for i, cred := range credentials {
		webauthnCreds[i] = cred.ToWebAuthnCredential()
	}

	// Create user object with existing credentials
	user := &passkey.WebAuthnUser{
		Account:     account,
		Credentials: webauthnCreds,
	}

	// Begin registration for additional credential
	creation, sessionData, err := s.Webauthn.BeginRegistration(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to begin add passkey: %w", err)
	}

	// Generate session token
	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Save session
	err = s.SaveSession(ctx, *sessionData, token, account.ID.Hex(), "", "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to save session: %w", err)
	}

	return creation, token, nil
}

// FinishAddPasskey completes the process of adding a new passkey to an existing account
func (s *Service) FinishAddPasskey(ctx context.Context, token string, httpReq *http.Request) error {
	// Get session
	session, exists, err := s.GetSession(ctx, token)
	if err != nil || !exists {
		return fmt.Errorf("failed to get session: %w", err)
	}
	// Delete temporary session (log error if deletion fails)
	defer func() {
		if err := s.DeleteSession(ctx, token); err != nil {
			log.Err(err).Str("token", token).Msg("failed to delete add-passkey session")
		}
	}()

	// Get account
	account, err := s.Store.GetAccount(ctx, session.AccountID)
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
	credential, err := s.Webauthn.FinishRegistration(user, session.SessionData, httpReq)
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
func (s *Service) DeletePasskeyByID(ctx context.Context, credentialID primitive.ObjectID, accountID primitive.ObjectID) error {
	// Verify the credential belongs to the account before deletion
	credential, err := s.Store.GetCredentialByID(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("failed to get credential: %w", err)
	}

	if credential.AccountID != accountID {
		return fmt.Errorf("credential does not belong to account")
	}

	// Delete the credential
	err = s.DeleteCredentialByID(ctx, credentialID, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
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
