package account_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
)

func (f *totpFixture) tokens() []model.Token {
	f.repo.mu.Lock()
	defer f.repo.mu.Unlock()
	return f.repo.accounts[f.id].Tokens
}

func (f *totpFixture) reauth(purpose, password, otp string) (*model.Token, error) {
	return f.svc.Reauthenticate(context.Background(), f.id, requests.ReauthRequest{Purpose: purpose, CurrentPassword: password, OTP: otp})
}

// The token is of the purpose's type, lives 5 minutes and is stored on the
// account for the operation to consume once.
// specRef: api-endpoint-behaviour.md B15
func TestReauthenticate_IssuesATokenOfThePurpose(t *testing.T) {
	for purpose, tokenType := range reauth.Purposes {
		t.Run(purpose, func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			token, err := f.reauth(purpose, testPassword, "")
			require.NoError(t, err)
			require.Equal(t, tokenType, token.Type)
			require.WithinDuration(t, time.Now().Add(5*time.Minute), token.ExpiresAt, 5*time.Second)
			require.Equal(t, []model.Token{*token}, f.tokens())

			acc, err := f.repo.GetAccountById(context.Background(), f.id)
			require.NoError(t, err)
			require.NoError(t, reauth.VerifyOnAccount(acc, reauth.Params{TokenType: tokenType, ReauthToken: &token.Value}))
		})
	}
}

// specRef: api-endpoint-behaviour.md B15
func TestReauthenticate_Refusals(t *testing.T) {
	t.Run("wrong password", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		_, err := f.reauth("profile_export", "wrong", "")
		require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		require.Empty(t, f.tokens())
	})
	t.Run("account without a password", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		f.repo.mu.Lock()
		acc := f.repo.accounts[f.id]
		acc.Password = nil
		f.repo.accounts[f.id] = acc
		f.repo.mu.Unlock()
		_, err := f.reauth("profile_export", testPassword, "")
		require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		require.Empty(t, f.tokens())
	})
	t.Run("unknown purpose", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		_, err := f.reauth("login", testPassword, "")
		require.ErrorIs(t, err, reauth.ErrUnknownPurpose)
		require.Empty(t, f.tokens())
	})
}

// With 2FA on, a token needs the code too. Without a code the 2FA prompt comes
// first; a code is checked only after the password, so a wrong password uses no
// backup code up and does not count toward the 2FA lockout.
// specRef: api-endpoint-behaviour.md B15, E6, E12
func TestReauthenticate_PasswordIsCheckedBeforeTheCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	secret, codes := f.enable(t)

	_, err := f.reauth("email_change", testPassword, "")
	require.ErrorIs(t, err, account.ErrTOTPRequired)
	_, err = f.reauth("email_change", "wrong", "")
	require.ErrorIs(t, err, account.ErrTOTPRequired, "the password is not tested without the code")

	for range 4 { // below ID_LIMITER_MAX
		_, err = f.reauth("email_change", "wrong", codes[0])
		require.ErrorIs(t, err, reauth.ErrInvalidPassword)
	}
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8, "a wrong password uses no code up")

	_, err = f.reauth("email_change", testPassword, "000000")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	require.Empty(t, f.tokens())

	token, err := f.reauth("email_change", testPassword, codes[0])
	require.NoError(t, err)
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 7)
	require.Equal(t, auth.TokenTypeReauthEmailChange, token.Type)

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = f.reauth("account_deletion", testPassword, code)
	require.NoError(t, err)
}

// 2FA disable and backup-code regeneration ask for the code themselves, so
// their token is issued for the password alone and a code sent here is not
// checked or used up.
// specRef: api-endpoint-behaviour.md B15, E3, E13
func TestReauthenticate_TotpChangesTakeThePasswordAlone(t *testing.T) {
	for _, purpose := range []string{"totp_disable", "backup_codes_regenerate"} {
		t.Run(purpose, func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			_, codes := f.enable(t)
			_, err := f.reauth(purpose, testPassword, "")
			require.NoError(t, err)
			_, err = f.reauth(purpose, testPassword, codes[0])
			require.NoError(t, err)
			require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8)
			_, err = f.reauth(purpose, "wrong", "")
			require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		})
	}
}

// Failed passwords are counted per account before the check: past the limit
// even the correct password is refused and no token is issued. The count is
// separate from the 2FA one, so login 2FA keeps working, and a success
// clears it. specRef: api-endpoint-behaviour.md B15
func TestReauthenticate_FailedPasswordsAreLimitedPerAccount(t *testing.T) {
	t.Run("lockout", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		_, codes := f.enable(t)
		for range 5 { // ID_LIMITER_MAX
			_, err := f.reauth("totp_disable", "wrong", "")
			require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		}
		_, err := f.reauth("totp_disable", testPassword, "")
		require.ErrorIs(t, err, account.ErrReauthRateLimited)
		_, err = f.reauth("email_change", testPassword, codes[0])
		require.ErrorIs(t, err, account.ErrReauthRateLimited, "the limit covers every purpose")
		require.Empty(t, f.tokens())
		require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8, "no code used up while locked")

		_, err = f.svc.VerifyTotp(context.Background(), f.id, codes[1])
		require.NoError(t, err, "login 2FA is not locked")
	})
	t.Run("success clears the count", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		for range 4 {
			_, err := f.reauth("profile_export", "wrong", "")
			require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		}
		_, err := f.reauth("profile_export", testPassword, "")
		require.NoError(t, err)
		for range 5 {
			_, err = f.reauth("profile_export", "wrong", "")
			require.ErrorIs(t, err, reauth.ErrInvalidPassword)
		}
	})
	t.Run("counter error refuses", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		f.cache.incrErr = context.DeadlineExceeded
		_, err := f.reauth("profile_export", testPassword, "")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Empty(t, f.tokens())
	})
}
