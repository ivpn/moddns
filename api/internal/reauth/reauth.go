// Package reauth provides the unified reauthentication primitive used by
// every privileged operation in the API (account deletion, email change,
// profile export, profile import).
//
// A privileged operation must prove fresh control of the account beyond the
// session cookie. Two credential paths are accepted:
//
//  1. current_password — the user's plaintext password. If the account has
//     TOTP enabled, a valid OTP code (delivered via the x-mfa-code header)
//     is also required.
//  2. reauth_token — a short-lived (5 min) token issued by the WebAuthn
//     passkey reauth ceremony in api/service/webauthn.go:FinishReauth. The
//     ceremony already proved possession of a passkey, so the token path
//     bypasses MFA. Tokens are single-use: a successful match removes the
//     token from the account.
//
// Exactly one of the two credentials must be supplied; supplying neither or
// both is a validation error.
//
// This package depends only on api/model and api/internal/auth, keeping it
// a leaf in the dependency graph. Both AccountService and ProfileService
// import it without creating a cycle.
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

// MfaVerifier is the narrow MFA surface the helper needs on the password
// path. It is satisfied by *account.AccountService (which already exposes
// MfaCheck). May be nil — in which case MFA is skipped, mirroring the
// pre-unification behaviour of ProfileService.verifyReauth for callers that
// do not yet wire the verifier.
type MfaVerifier interface {
	MfaCheck(ctx context.Context, acc *model.Account, mfa *model.MfaData) error
}

// Params are the inputs to Verify.
type Params struct {
	// AccountId identifies the account whose reauth we are verifying.
	AccountId string

	// TokenType is the auth.TokenTypeReauth* constant a reauth_token must
	// match. Ignored on the password path.
	TokenType string

	// Password is the user-supplied plaintext password (the "current_password"
	// field of the request body). May be nil.
	Password *string

	// ReauthToken is the user-supplied short-lived token issued by the
	// passkey reauth ceremony. May be nil.
	ReauthToken *string

	// Mfa carries the x-mfa-code header value. May be nil — Verify substitutes
	// an empty struct to keep MfaCheck panic-free.
	Mfa *model.MfaData

	// PersistOnConsume controls whether Verify persists the account after
	// consuming a reauth_token. Callers whose outer flow already persists the
	// account (e.g. DeleteAccount, UpdateAccount handling email change) pass
	// false; callers that do not persist anything else on this code path
	// (profile export/import) pass true.
	PersistOnConsume bool
}

// Reauth error sentinels. HandleError maps all five to HTTP 400 — the
// caller's session is still valid; what failed is part of the request body
// (wrong password, wrong/expired reauth_token, or a missing OTP). Returning
// 401 would be misread as session expiry by the global axios interceptor on
// the frontend and would trigger an unwanted logout.
var (
	// ErrMissingAuthMethod is returned when neither password nor token is
	// supplied.
	ErrMissingAuthMethod = errors.New("missing current_password or reauth_token")

	// ErrMultipleAuthMethods is returned when both password and token are
	// supplied — defense in depth; the DTO validator already enforces this.
	ErrMultipleAuthMethods = errors.New("provide only one of current_password or reauth_token")

	// ErrInvalidPassword is returned when the supplied password does not match
	// the account, OR when the account cannot be loaded. The two cases are
	// deliberately conflated so the response does not leak whether the
	// accountId exists.
	ErrInvalidPassword = errors.New("invalid current password")

	// ErrInvalidReauthToken is returned when the supplied token does not match
	// any reauth_token of the requested TokenType on the account.
	ErrInvalidReauthToken = errors.New("invalid reauth token")

	// ErrReauthTokenExpired is returned when the supplied token matches but
	// is past its ExpiresAt.
	ErrReauthTokenExpired = errors.New("reauth token expired")
)

// Verify executes the unified reauth flow described in the package doc.
//
// On success it returns the loaded account so callers can continue with the
// same instance they would otherwise have fetched themselves. The returned
// account reflects any in-place mutation Verify performed (token
// consumption); when PersistOnConsume is false the caller is responsible for
// the eventual UpdateAccount.
//
// Verify never returns a nil error with a nil account, and never returns a
// non-nil account on error.
//
// Use VerifyOnAccount instead when the caller already has the account loaded
// (e.g. AccountService.UpdateAccount has just fetched it for unrelated
// purposes); VerifyOnAccount skips the redundant store round-trip and lets
// the caller own persistence.
func Verify(ctx context.Context, store Store, verifier MfaVerifier, p Params) (*model.Account, error) {
	if p.Password == nil && p.ReauthToken == nil {
		return nil, ErrMissingAuthMethod
	}
	if p.Password != nil && p.ReauthToken != nil {
		return nil, ErrMultipleAuthMethods
	}

	account, err := store.GetAccountById(ctx, p.AccountId)
	if err != nil {
		if errors.Is(err, dbErrors.ErrAccountNotFound) {
			// Conflate with bad-password to avoid account enumeration.
			return nil, ErrInvalidPassword
		}
		return nil, err
	}

	if err := verifyOnLoadedAccount(ctx, account, verifier, p); err != nil {
		return nil, err
	}

	if p.PersistOnConsume && p.ReauthToken != nil {
		if _, err := store.UpdateAccount(ctx, account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

// VerifyOnAccount runs the reauth check against an already-loaded account.
// It mutates account.Tokens in place when a reauth_token is consumed; the
// caller is responsible for persisting the account afterwards (typically as
// part of an outer UpdateAccount that batches other mutations).
//
// PersistOnConsume is ignored — VerifyOnAccount never calls the store.
func VerifyOnAccount(ctx context.Context, account *model.Account, verifier MfaVerifier, p Params) error {
	if account == nil {
		// Defensive: an unauthenticated caller should never reach here, but
		// returning ErrInvalidPassword (rather than panicking) keeps the
		// failure mode aligned with the not-found branch in Verify.
		return ErrInvalidPassword
	}
	if p.Password == nil && p.ReauthToken == nil {
		return ErrMissingAuthMethod
	}
	if p.Password != nil && p.ReauthToken != nil {
		return ErrMultipleAuthMethods
	}
	return verifyOnLoadedAccount(ctx, account, verifier, p)
}

// verifyOnLoadedAccount is the shared post-fetch check used by both Verify
// and VerifyOnAccount. The credential-mutual-exclusivity guard runs in the
// public entry points so this helper can assume exactly one of Password /
// ReauthToken is set.
func verifyOnLoadedAccount(ctx context.Context, account *model.Account, verifier MfaVerifier, p Params) error {
	if p.Password != nil {
		// MFA check runs BEFORE password verification so a user with the
		// wrong password but no OTP code receives ErrTOTPRequired rather than
		// ErrInvalidPassword — matches the established order in
		// AccountService.DeleteAccount and handleEmailUpdate.
		if verifier != nil {
			mfa := p.Mfa
			if mfa == nil {
				mfa = &model.MfaData{}
			}
			if err := verifier.MfaCheck(ctx, account, mfa); err != nil {
				return err
			}
		}
		if account.Password == nil || !auth.CheckPasswordHash(*p.Password, *account.Password) {
			return ErrInvalidPassword
		}
		return nil
	}

	// Token path.
	tokenValue := *p.ReauthToken
	kept := make([]model.Token, 0, len(account.Tokens))
	matched := false
	for _, t := range account.Tokens {
		if t.Type == p.TokenType && t.Value == tokenValue {
			if time.Now().After(t.ExpiresAt) {
				// Expired tokens are NOT consumed — caller may still see them
				// in the slice for diagnostics, but Verify returns expired.
				return ErrReauthTokenExpired
			}
			matched = true
			// Skip appending — this is the single-use consumption.
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
