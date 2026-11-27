// Package reauth provides the reauthentication token every privileged operation
// in the API consumes (account deletion, email change, password change and set,
// passkey add and delete, profile export and import, backup-code regeneration,
// turning 2FA on and off, signing out the other sessions).
//
// A privileged operation must prove fresh control of the account beyond the
// session cookie. It accepts only a reauth_token: a short-lived (5 min),
// single-use token bound to one purpose. Two ceremonies issue it:
//
//  1. POST /accounts/reauth: the current password, plus a 2FA code when the
//     account has TOTP on (api/service/account/reauth.go:Reauthenticate).
//  2. The WebAuthn passkey reauth ceremony (api/service/webauthn.go:FinishReauth).
//
// This follows the re-authentication pattern of RFC 9470 §3 (max_age): the
// credential is checked once, by the issuer, and the operation sees only the
// resulting token.
//
// This package depends only on api/model and api/internal/auth, keeping it a
// leaf in the dependency graph.
package reauth

import (
	"context"
	"errors"
	"time"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// Store is the narrow account-repository surface the helper needs. It is
// satisfied by repository.AccountRepository.
type Store interface {
	GetAccountById(ctx context.Context, accountId string) (*model.Account, error)
	UpdateAccount(ctx context.Context, account *model.Account) (*model.Account, error)
}

// Purposes maps each reauth purpose a client may ask for to the token type
// the operation it protects consumes.
var Purposes = map[string]string{
	"email_change":            auth.TokenTypeReauthEmailChange,
	"account_deletion":        auth.TokenTypeReauthAccountDeletion,
	"profile_export":          auth.TokenTypeReauthProfileExport,
	"profile_import":          auth.TokenTypeReauthProfileImport,
	"password_set":            auth.TokenTypeReauthPasswordSet,
	"passkey_add":             auth.TokenTypeReauthPasskeyAdd,
	"passkey_delete":          auth.TokenTypeReauthPasskeyDelete,
	"backup_codes_regenerate": auth.TokenTypeReauthBackupCodesRegen,
	"totp_disable":            auth.TokenTypeReauthTotpDisable,
	"totp_enable":             auth.TokenTypeReauthTotpEnable,
	"sessions_delete":         auth.TokenTypeReauthSessionsDelete,
}

// TokenType returns the token type issued for purpose.
func TokenType(purpose string) (string, bool) {
	tokenType, ok := Purposes[purpose]
	return tokenType, ok
}

// Params are the inputs to Verify.
type Params struct {
	// AccountId identifies the account whose reauth we are verifying.
	AccountId string

	// TokenType is the auth.TokenTypeReauth* constant the reauth_token must match.
	TokenType string

	// ReauthToken is the user-supplied token. May be nil.
	ReauthToken *string

	// PersistOnConsume controls whether Verify persists the account after
	// consuming the token. Callers whose outer flow already persists the
	// account pass false.
	PersistOnConsume bool
}

// Reauth error sentinels. HandleError maps all of them to HTTP 400: the
// caller's session is still valid; what failed is part of the request body.
// Returning 401 would be misread as session expiry by the frontend.
var (
	// ErrMissingAuthMethod is returned when no reauth_token is supplied.
	ErrMissingAuthMethod = errors.New("missing reauth_token")

	// ErrInvalidPassword is returned by the password ceremony when the password
	// does not match the account, or the account has none.
	ErrInvalidPassword = errors.New("invalid current password")

	// ErrInvalidReauthToken is returned when the supplied token does not match
	// any reauth_token of the requested TokenType on the account, or when the
	// account cannot be loaded (conflated to avoid account enumeration).
	ErrInvalidReauthToken = errors.New("invalid reauth token")

	// ErrReauthTokenExpired is returned when the supplied token matches but
	// is past its ExpiresAt.
	ErrReauthTokenExpired = errors.New("reauth token expired")

	// ErrUnknownPurpose is returned when a token is asked for a purpose no
	// operation consumes.
	ErrUnknownPurpose = errors.New("unknown reauth purpose")
)

// Verify loads the account and consumes its reauth token.
//
// On success it returns the loaded account so callers can continue with the
// same instance. Verify never returns a nil error with a nil account, and never
// returns a non-nil account on error.
func Verify(ctx context.Context, store Store, p Params) (*model.Account, error) {
	if p.ReauthToken == nil || *p.ReauthToken == "" {
		return nil, ErrMissingAuthMethod
	}

	account, err := store.GetAccountById(ctx, p.AccountId)
	if err != nil {
		if errors.Is(err, dbErrors.ErrAccountNotFound) {
			return nil, ErrInvalidReauthToken
		}
		return nil, err
	}

	if err := consume(account, p); err != nil {
		return nil, err
	}

	if p.PersistOnConsume {
		if _, err := store.UpdateAccount(ctx, account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

// VerifyOnAccount consumes the reauth token on an already-loaded account. It
// removes the token from account.Tokens in place; the caller persists the
// account afterwards. PersistOnConsume is ignored.
func VerifyOnAccount(account *model.Account, p Params) error {
	if account == nil {
		return ErrInvalidReauthToken
	}
	if p.ReauthToken == nil || *p.ReauthToken == "" {
		return ErrMissingAuthMethod
	}
	return consume(account, p)
}

func consume(account *model.Account, p Params) error {
	tokenValue := *p.ReauthToken
	kept := make([]model.Token, 0, len(account.Tokens))
	matched := false
	for _, t := range account.Tokens {
		if t.Type == p.TokenType && t.Value == tokenValue {
			if time.Now().After(t.ExpiresAt) {
				// An expired token is not consumed.
				return ErrReauthTokenExpired
			}
			matched = true
			continue
		}
		kept = append(kept, t)
	}
	if !matched {
		return ErrInvalidReauthToken
	}
	account.Tokens = kept
	return nil
}

// New creates a reauth token for purpose. The caller stores it with
// AccountRepository.AddReauthToken, which also removes expired reauth tokens.
func New(purpose string) (*model.Token, error) {
	tokenType, ok := TokenType(purpose)
	if !ok {
		return nil, ErrUnknownPurpose
	}
	return auth.NewToken(tokenType)
}
