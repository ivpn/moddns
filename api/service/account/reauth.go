package account

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// codeCheckedByOperation lists the purposes whose operation asks for the 2FA
// code itself (2FA disable, backup-code regeneration). Their token is issued
// for the password alone, so the user enters one code, as with a passkey.
var codeCheckedByOperation = map[string]bool{
	"totp_disable":            true,
	"backup_codes_regenerate": true,
}

// Reauthenticate checks the current password, then the 2FA code when the
// account has TOTP on, and issues a single-use reauth token for the purpose.
//
// Order: without a code the 2FA prompt comes first, so the client can ask for
// it; a code is checked only after the password, so a wrong password uses no
// backup code up and does not count toward the 2FA lockout. Failed passwords
// are counted per account before the check (NIST SP 800-63B-4 §3.2.2); a
// success clears the count.
func (a *AccountService) Reauthenticate(ctx context.Context, accountId string, req requests.ReauthRequest) (*model.Token, error) {
	if _, ok := reauth.TokenType(req.Purpose); !ok {
		return nil, reauth.ErrUnknownPurpose
	}
	acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, err
	}
	checkCode := !codeCheckedByOperation[req.Purpose]
	mfa := &model.MfaData{OTP: req.OTP}
	if checkCode && mfa.OTP == "" {
		if err := a.MfaCheck(ctx, acc, mfa); err != nil {
			return nil, err
		}
	}

	limiter := a.reauthPasswordLimiter(accountId)
	allowed, err := limiter.Attempt()
	if err != nil {
		return nil, err
	}
	if !allowed {
		log.Ctx(ctx).Warn().Msg("reauth: too many failed passwords")
		return nil, ErrReauthRateLimited
	}
	if acc.Password == nil || !auth.CheckPasswordHash(req.CurrentPassword, *acc.Password) {
		return nil, reauth.ErrInvalidPassword
	}
	_ = limiter.Reset()

	if checkCode && mfa.OTP != "" {
		if err := a.MfaCheck(ctx, acc, mfa); err != nil {
			return nil, err
		}
	}

	token, err := reauth.New(req.Purpose)
	if err != nil {
		return nil, err
	}
	if err := a.AccountRepository.AddReauthToken(ctx, accountId, *token, time.Now()); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *AccountService) reauthPasswordLimiter(accountID string) utils.IDLimiter {
	return utils.IDLimiter{
		ID:    accountID,
		Label: "reauth_password_fails",
		Max:   a.ServiceCfg.IdLimiterMax,
		Exp:   a.ServiceCfg.IdLimiterExpiration,
		Cache: a.Cache,
	}
}
