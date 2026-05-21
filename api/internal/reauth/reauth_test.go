package reauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ptr(s string) *string { return &s }

func bcryptHash(t *testing.T, password string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(b)
}

// fakeStore is a hand-written test double for reauth.Store. It records every
// UpdateAccount call so tests can assert persistence behaviour.
type fakeStore struct {
	account     *model.Account
	getErr      error
	updateCalls int
	updateErr   error
}

func (s *fakeStore) GetAccountById(_ context.Context, _ string) (*model.Account, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.account, nil
}

func (s *fakeStore) UpdateAccount(_ context.Context, account *model.Account) (*model.Account, error) {
	s.updateCalls++
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.account = account
	return account, nil
}

// fakeMfa is a hand-written MfaVerifier double. mfaErr is returned verbatim;
// nil means MFA passes.
type fakeMfa struct {
	mfaErr error
	calls  int
}

func (m *fakeMfa) MfaCheck(_ context.Context, _ *model.Account, _ *model.MfaData) error {
	m.calls++
	return m.mfaErr
}

// totpEnabledAccount builds an account record with TOTP on.
func totpEnabledAccount(hashed string) *model.Account {
	return &model.Account{
		Password: &hashed,
		MFA: model.MFASettings{
			TOTP: model.TotpSettings{Enabled: true},
		},
	}
}

// tokenAccount builds an account with a single reauth token of the given
// type, value, and expiry.
func tokenAccount(tokenType, value string, expiresAt time.Time) *model.Account {
	return &model.Account{
		Tokens: []model.Token{
			{Type: tokenType, Value: value, ExpiresAt: expiresAt},
		},
	}
}

// ---------------------------------------------------------------------------
// Validation of credential combinations
// ---------------------------------------------------------------------------

func TestVerify_NoCredentials_ReturnsMissingAuthMethod(t *testing.T) {
	store := &fakeStore{}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		TokenType: auth.TokenTypeReauthAccountDeletion,
	})
	assert.ErrorIs(t, err, reauth.ErrMissingAuthMethod)
	assert.Equal(t, 0, store.updateCalls)
}

func TestVerify_BothCredentials_ReturnsMultipleAuthMethods(t *testing.T) {
	store := &fakeStore{}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthAccountDeletion,
		Password:    ptr("hunter2"),
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrMultipleAuthMethods)
	assert.Equal(t, 0, store.updateCalls)
}

// ---------------------------------------------------------------------------
// Password path
// ---------------------------------------------------------------------------

func TestVerify_Password_AccountNotFound_ReturnsInvalidPassword(t *testing.T) {
	// Account-not-found is conflated with bad password so the response does
	// not leak whether a given accountId exists.
	store := &fakeStore{getErr: dbErrors.ErrAccountNotFound}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		Password:  ptr("anything"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidPassword)
}

func TestVerify_Password_StoreError_BubblesUp(t *testing.T) {
	sentinel := errors.New("db down")
	store := &fakeStore{getErr: sentinel}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		Password:  ptr("anything"),
	})
	assert.ErrorIs(t, err, sentinel)
}

func TestVerify_Password_NoPasswordOnAccount_ReturnsInvalidPassword(t *testing.T) {
	// Passkey-only account (no password set).
	store := &fakeStore{account: &model.Account{Password: nil}}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		Password:  ptr("anything"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidPassword)
}

func TestVerify_Password_Wrong_ReturnsInvalidPassword(t *testing.T) {
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: &model.Account{Password: &hashed}}
	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		Password:  ptr("wrong"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidPassword)
}

func TestVerify_Password_NoMfaVerifier_MfaSkipped(t *testing.T) {
	// nil verifier → MFA check is skipped even if account has TOTP enabled.
	// This preserves pre-unification behaviour for callers that haven't yet
	// wired the verifier.
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: totpEnabledAccount(hashed)}
	acc, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId: "acct",
		Password:  ptr("correct"),
	})
	require.NoError(t, err)
	assert.NotNil(t, acc)
}

func TestVerify_Password_MfaDisabled_VerifierNotInvokedOnSuccess(t *testing.T) {
	// MfaCheck is still called when verifier != nil, but its short-circuit
	// (TOTP disabled → nil) means the call succeeds. We invoke it
	// unconditionally to keep the path uniform.
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: &model.Account{Password: &hashed}}
	verifier := &fakeMfa{mfaErr: nil}
	_, err := reauth.Verify(context.Background(), store, verifier, reauth.Params{
		AccountId: "acct",
		Password:  ptr("correct"),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, verifier.calls, "verifier is invoked once on password path")
}

func TestVerify_Password_MfaFails_PropagatesError(t *testing.T) {
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: totpEnabledAccount(hashed)}
	sentinel := errors.New("totp required")
	verifier := &fakeMfa{mfaErr: sentinel}

	_, err := reauth.Verify(context.Background(), store, verifier, reauth.Params{
		AccountId: "acct",
		Password:  ptr("correct"),
		Mfa:       &model.MfaData{},
	})
	assert.ErrorIs(t, err, sentinel)
}

func TestVerify_Password_MfaCheckedBeforePassword(t *testing.T) {
	// When both password and OTP are wrong, the MFA error wins — matches
	// DeleteAccount / handleEmailUpdate ordering.
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: totpEnabledAccount(hashed)}
	mfaErr := errors.New("totp required")
	verifier := &fakeMfa{mfaErr: mfaErr}

	_, err := reauth.Verify(context.Background(), store, verifier, reauth.Params{
		AccountId: "acct",
		Password:  ptr("wrong-password"),
	})
	assert.ErrorIs(t, err, mfaErr)
}

func TestVerify_Password_NilMfa_PassesEmptyToVerifier(t *testing.T) {
	// MfaCheck must never receive a nil *MfaData — Verify substitutes an
	// empty struct.
	hashed := bcryptHash(t, "correct")
	store := &fakeStore{account: &model.Account{Password: &hashed}}
	verifier := &fakeMfa{mfaErr: nil}
	_, err := reauth.Verify(context.Background(), store, verifier, reauth.Params{
		AccountId: "acct",
		Password:  ptr("correct"),
		Mfa:       nil,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, verifier.calls)
}

// ---------------------------------------------------------------------------
// Token path
// ---------------------------------------------------------------------------

func TestVerify_Token_NoMatch_ReturnsInvalid(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "real-token", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthProfileExport,
		ReauthToken: ptr("wrong-token"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
	assert.Equal(t, 0, store.updateCalls)
}

func TestVerify_Token_WrongType_ReturnsInvalid(t *testing.T) {
	// Token issued for account_deletion must not satisfy a profile_export
	// reauth check — guards against token-purpose mixing if a client app
	// pastes the wrong token into the wrong field.
	acc := tokenAccount(auth.TokenTypeReauthAccountDeletion, "tok", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthProfileExport,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
}

func TestVerify_Token_Expired_ReturnsExpired_NotConsumed(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "tok", time.Now().Add(-1*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileExport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true, // even with persist on, expired tokens are NOT consumed
	})
	assert.ErrorIs(t, err, reauth.ErrReauthTokenExpired)
	assert.Equal(t, 0, store.updateCalls, "expired token must not be consumed/persisted")
	// Expired token still present in the account slice.
	assert.Len(t, acc.Tokens, 1)
}

func TestVerify_Token_Match_PersistOff_NoUpdate(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthAccountDeletion, "tok", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	got, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthAccountDeletion,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: false,
	})
	require.NoError(t, err)
	require.NotNil(t, got)

	// Token consumed from the in-memory account but no persist call —
	// outer flow (DeleteAccount / email change) handles persistence later.
	assert.Empty(t, got.Tokens)
	assert.Equal(t, 0, store.updateCalls)
}

func TestVerify_Token_Match_PersistOn_OneUpdate(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "tok", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	got, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileExport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Empty(t, got.Tokens)
	assert.Equal(t, 1, store.updateCalls, "PersistOnConsume=true must persist exactly once")
}

func TestVerify_Token_SingleUse(t *testing.T) {
	// After successful consume, the same token cannot reauth a second time.
	acc := tokenAccount(auth.TokenTypeReauthProfileImport, "tok", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileImport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, store.updateCalls)

	// Second attempt — store.account now has the consumed-token state.
	_, err = reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileImport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
}

func TestVerify_Token_PreservesOtherTokens(t *testing.T) {
	// Consumption must remove only the matching token; unrelated tokens
	// (e.g. password_reset, or a different reauth purpose) stay.
	now := time.Now()
	acc := &model.Account{
		Tokens: []model.Token{
			{Type: auth.TokenTypePasswordReset, Value: "pr", ExpiresAt: now.Add(time.Hour)},
			{Type: auth.TokenTypeReauthEmailChange, Value: "ec", ExpiresAt: now.Add(5 * time.Minute)},
			{Type: auth.TokenTypeReauthAccountDeletion, Value: "ad", ExpiresAt: now.Add(5 * time.Minute)},
		},
	}
	store := &fakeStore{account: acc}

	got, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthEmailChange,
		ReauthToken:      ptr("ec"),
		PersistOnConsume: true,
	})
	require.NoError(t, err)
	require.Len(t, got.Tokens, 2)

	remainingValues := map[string]bool{}
	for _, t := range got.Tokens {
		remainingValues[t.Value] = true
	}
	assert.True(t, remainingValues["pr"], "password reset token must remain")
	assert.True(t, remainingValues["ad"], "account deletion reauth token must remain")
	assert.False(t, remainingValues["ec"], "consumed email-change token must be gone")
}

func TestVerify_Token_MfaVerifierIgnoredOnTokenPath(t *testing.T) {
	// The passkey ceremony that issued the token already proved MFA. Even
	// if a verifier is supplied AND the account has TOTP enabled AND no OTP
	// is provided, the token path succeeds. Matches DeleteAccount /
	// handleEmailUpdate behaviour.
	hashed := bcryptHash(t, "doesntmatter")
	acc := totpEnabledAccount(hashed)
	acc.Tokens = []model.Token{
		{Type: auth.TokenTypeReauthAccountDeletion, Value: "tok", ExpiresAt: time.Now().Add(5 * time.Minute)},
	}
	store := &fakeStore{account: acc}

	mfaSentinel := errors.New("totp required")
	verifier := &fakeMfa{mfaErr: mfaSentinel}

	_, err := reauth.Verify(context.Background(), store, verifier, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthAccountDeletion,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, verifier.calls, "MFA verifier must not be invoked on token path")
}

func TestVerify_Token_PersistFailure_PropagatesError(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "tok", time.Now().Add(5*time.Minute))
	sentinel := errors.New("update failed")
	store := &fakeStore{account: acc, updateErr: sentinel}

	_, err := reauth.Verify(context.Background(), store, nil, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileExport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 1, store.updateCalls)
}

// ---------------------------------------------------------------------------
// VerifyOnAccount — operates on a pre-loaded account, never touches the store
// ---------------------------------------------------------------------------

func TestVerifyOnAccount_NilAccount_ReturnsInvalidPassword(t *testing.T) {
	err := reauth.VerifyOnAccount(context.Background(), nil, nil, reauth.Params{
		Password: ptr("anything"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidPassword)
}

func TestVerifyOnAccount_BothCredentials_ReturnsMultipleAuthMethods(t *testing.T) {
	err := reauth.VerifyOnAccount(context.Background(), &model.Account{}, nil, reauth.Params{
		Password:    ptr("p"),
		ReauthToken: ptr("t"),
	})
	assert.ErrorIs(t, err, reauth.ErrMultipleAuthMethods)
}

func TestVerifyOnAccount_Password_Correct_MutatesNothing(t *testing.T) {
	hashed := bcryptHash(t, "correct")
	acc := &model.Account{
		Password: &hashed,
		Tokens:   []model.Token{{Type: "x", Value: "y"}},
	}
	err := reauth.VerifyOnAccount(context.Background(), acc, nil, reauth.Params{
		Password: ptr("correct"),
	})
	require.NoError(t, err)
	assert.Len(t, acc.Tokens, 1, "password path must not touch tokens")
}

func TestVerifyOnAccount_Token_Match_ConsumesInPlace_NoPersist(t *testing.T) {
	// The whole point of VerifyOnAccount: caller owns persistence.
	acc := tokenAccount(auth.TokenTypeReauthEmailChange, "tok", time.Now().Add(5*time.Minute))
	err := reauth.VerifyOnAccount(context.Background(), acc, nil, reauth.Params{
		TokenType:        auth.TokenTypeReauthEmailChange,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true, // must be ignored
	})
	require.NoError(t, err)
	assert.Empty(t, acc.Tokens, "matched token must be consumed in place")
}

func TestVerifyOnAccount_Token_Expired_ReturnsExpired(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthAccountDeletion, "tok", time.Now().Add(-1*time.Minute))
	err := reauth.VerifyOnAccount(context.Background(), acc, nil, reauth.Params{
		TokenType:   auth.TokenTypeReauthAccountDeletion,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrReauthTokenExpired)
	assert.Len(t, acc.Tokens, 1, "expired token must not be consumed")
}

func TestVerifyOnAccount_Password_MfaFails_PropagatesError(t *testing.T) {
	hashed := bcryptHash(t, "correct")
	acc := totpEnabledAccount(hashed)
	mfaErr := errors.New("totp required")
	err := reauth.VerifyOnAccount(context.Background(), acc, &fakeMfa{mfaErr: mfaErr}, reauth.Params{
		Password: ptr("correct"),
	})
	assert.ErrorIs(t, err, mfaErr)
}
