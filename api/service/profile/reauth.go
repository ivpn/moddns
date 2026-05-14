package profile

import (
	"context"
	"time"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/model"
)

// verifyReauth validates the caller's reauth credential (password or reauth token)
// against the account record identified by accountId.
//
// Exactly one of currentPassword or reauthToken must be non-nil; if both are nil
// the function returns ErrReauthRequired.  The caller is responsible for ensuring
// that at most one is set (the handler layer enforces this via DTO validation).
//
// Behaviour mirrors api/service/account/account.go:592-628 (DeleteAccount).
// specRef: M4, M5, M6
func (p *ProfileService) verifyReauth(
	ctx context.Context,
	accountId string,
	tokenType string,
	currentPassword, reauthToken *string,
) error {
	if currentPassword == nil && reauthToken == nil {
		// specRef: M5
		return ErrReauthRequired
	}

	account, err := p.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		if err == dbErrors.ErrAccountNotFound {
			return ErrReauthInvalid
		}
		return err
	}

	if currentPassword != nil {
		// specRef: M6 — password path
		if account.Password == nil || !auth.CheckPasswordHash(*currentPassword, *account.Password) {
			return ErrReauthInvalid
		}
		return nil
	}

	// specRef: M6 — reauth token path; single-use semantics match DeleteAccount
	// (api/service/account/account.go:608-627).
	tokenValue := *reauthToken
	matched := false
	kept := make([]model.Token, 0, len(account.Tokens))
	for _, t := range account.Tokens {
		if t.Type == tokenType && t.Value == tokenValue {
			if time.Now().After(t.ExpiresAt) {
				// Expired token — treat as invalid; do not consume.
				return ErrReauthInvalid
			}
			matched = true
			// Single-use: skip appending — token is consumed.
			continue
		}
		kept = append(kept, t)
	}
	if !matched {
		return ErrReauthInvalid
	}

	// Persist the token consumption so the same token cannot be replayed.
	account.Tokens = kept
	if _, err := p.AccountRepository.UpdateAccount(ctx, account); err != nil {
		return err
	}

	return nil
}
