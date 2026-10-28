package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/internal/webauthn"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/passkey"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// passkeyUser describes acc and the credentials it holds to the relying party.
func passkeyUser(acc *model.Account, credentials []model.Credential) webauthn.User {
	return webauthn.User{
		AccountID:   acc.ID,
		Handle:      acc.WebAuthnID(),
		Name:        acc.WebAuthnName(),
		DisplayName: acc.WebAuthnDisplayName(),
		Credentials: credentials,
	}
}

// accountLoader returns a UserLoader that reads the ceremony's account, into *acc
// when acc is set, and, when getCredentials is set, the credentials it holds.
func (s *Service) accountLoader(acc **model.Account, getCredentials func(context.Context, primitive.ObjectID) ([]model.Credential, error)) webauthn.UserLoader {
	return func(ctx context.Context, accountID string) (webauthn.User, error) {
		account, err := s.Store.GetAccount(ctx, accountID)
		if err != nil {
			return webauthn.User{}, fmt.Errorf("failed to get account: %w", err)
		}
		var credentials []model.Credential
		if getCredentials != nil {
			credentials, err = getCredentials(ctx, account.ID)
			if err != nil {
				return webauthn.User{}, fmt.Errorf("failed to get credentials: %w", err)
			}
		}
		if acc != nil {
			*acc = account
		}
		return passkeyUser(account, credentials), nil
	}
}

// ceremonyError returns mismatchErr for state begun by another ceremony or account.
func ceremonyError(err, mismatchErr error) error {
	if errors.Is(err, webauthn.ErrCeremonyMismatch) {
		return mismatchErr
	}
	return err
}

// BeginRegistration starts the WebAuthn registration process.
// subID is the external subscription ID from the signup request; it is kept with
// the ceremony so FinishRegistration can forward it to the webhook.
func (s *Service) BeginRegistration(ctx context.Context, account *model.Account, subID string) (*webauthn.CredentialCreation, string, error) {
	return s.Webauthn.BeginRegistration(ctx, passkeyUser(account, nil), subID)
}

// FinishRegistration completes the WebAuthn registration process
func (s *Service) FinishRegistration(ctx context.Context, token string, httpReq *http.Request, paSessionID string) error {
	var account *model.Account
	result, err := s.Webauthn.FinishRegistration(ctx, token, s.accountLoader(&account, nil), httpReq)
	if err != nil {
		return ceremonyError(err, passkey.ErrFinishRegistration)
	}

	// Save credential
	err = s.Store.SaveCredential(ctx, result.Credential)
	if err != nil {
		return fmt.Errorf("failed to save credential: %w", err)
	}

	tokenHash := ""
	if sub, subErr := s.GetSubscription(ctx, account.ID.Hex()); subErr == nil && sub != nil {
		tokenHash = sub.TokenHash
	}

	// Use the external subID preserved in the ceremony from BeginRegistration
	if err = s.CompleteRegistration(ctx, account, result.SubID, paSessionID, tokenHash); err != nil {
		return fmt.Errorf("failed to complete registration: %w", err)
	}

	return nil
}

// BeginLogin starts the WebAuthn login process
func (s *Service) BeginLogin(ctx context.Context, email string) (*webauthn.CredentialAssertion, string, error) {
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

	return s.Webauthn.BeginLogin(ctx, passkeyUser(account, credentials))
}

func (s *Service) BeginReauth(ctx context.Context, purpose, accountId string) (*webauthn.CredentialAssertion, string, error) {
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

	return s.Webauthn.BeginReauth(ctx, passkeyUser(acc, credentials), purpose)
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
	var acc *model.Account
	result, err := s.Webauthn.FinishReauth(ctx, tmpToken, accountID, s.accountLoader(&acc, s.Store.GetCredentials), httpReq)
	if err = s.recordAssertion(ctx, acc, result, ceremonyError(err, passkey.ErrFinishReauth)); err != nil {
		return nil, err
	}

	tokenType, ok := reauthTokenType(result.Purpose)
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
	var account *model.Account
	result, err := s.Webauthn.FinishLogin(ctx, tmpToken, s.accountLoader(&account, s.Store.GetCredentials), httpReq)
	if err = s.recordAssertion(ctx, account, result, ceremonyError(err, passkey.ErrFinishLogin)); err != nil {
		return nil, "", "", err
	}
	purpose := result.Purpose

	// Save the session
	sessionData := gowebauthn.SessionData{
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

// recordAssertion stores the new signature counter of the credential a verified
// login or reauth asserted. It returns err unchanged when the finish call failed.
func (s *Service) recordAssertion(ctx context.Context, account *model.Account, result *webauthn.Result, err error) error {
	if err != nil {
		return err
	}
	if err := s.Store.UpdateCredential(ctx, result.CredentialID, account.ID, result.SignCount); err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}
	return nil
}

// BeginAddPasskey starts the process of adding a new passkey to an existing account
// The account owner must first verify with the current password (plus TOTP when enabled)
// or a passkey reauth token issued for passkey_add, so a session alone cannot add a credential.
func (s *Service) BeginAddPasskey(ctx context.Context, accountID string, currentPassword, reauthToken *string, mfa *model.MfaData) (*webauthn.CredentialCreation, string, error) {
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

	return s.Webauthn.BeginAddPasskey(ctx, passkeyUser(account, credentials))
}

// FinishAddPasskey completes the process of adding a new passkey to an existing account
func (s *Service) FinishAddPasskey(ctx context.Context, accountID, token string, httpReq *http.Request) error {
	result, err := s.Webauthn.FinishAddPasskey(ctx, token, accountID, s.accountLoader(nil, s.GetCredentials), httpReq)
	if err != nil {
		return ceremonyError(err, passkey.ErrFinishAddPasskey)
	}

	// Save the new credential
	return s.SaveCredential(ctx, result.Credential)
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
