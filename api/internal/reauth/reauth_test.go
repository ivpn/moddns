package reauth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/model"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ptr(s string) *string { return &s }

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
// Missing token and account lookup
// ---------------------------------------------------------------------------

func TestVerify_NoToken_ReturnsMissingAuthMethod(t *testing.T) {
	for name, tok := range map[string]*string{"nil": nil, "empty": ptr("")} {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			_, err := reauth.Verify(context.Background(), store, reauth.Params{
				AccountId:   "acct",
				TokenType:   auth.TokenTypeReauthAccountDeletion,
				ReauthToken: tok,
			})
			assert.ErrorIs(t, err, reauth.ErrMissingAuthMethod)
			assert.Equal(t, 0, store.updateCalls)
		})
	}
}

func TestVerify_AccountNotFound_ReturnsInvalidToken(t *testing.T) {
	// Conflated with a wrong token so the response does not reveal whether the account exists.
	store := &fakeStore{getErr: dbErrors.ErrAccountNotFound}
	_, err := reauth.Verify(context.Background(), store, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthProfileExport,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
}

func TestVerify_StoreError_BubblesUp(t *testing.T) {
	sentinel := errors.New("db down")
	store := &fakeStore{getErr: sentinel}
	_, err := reauth.Verify(context.Background(), store, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthProfileExport,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, sentinel)
}

// ---------------------------------------------------------------------------
// Token path
// ---------------------------------------------------------------------------

func TestVerify_Token_NoMatch_ReturnsInvalid(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "real-token", time.Now().Add(5*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, reauth.Params{
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

	_, err := reauth.Verify(context.Background(), store, reauth.Params{
		AccountId:   "acct",
		TokenType:   auth.TokenTypeReauthProfileExport,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
}

func TestVerify_Token_Expired_ReturnsExpired_NotConsumed(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "tok", time.Now().Add(-1*time.Minute))
	store := &fakeStore{account: acc}

	_, err := reauth.Verify(context.Background(), store, reauth.Params{
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

	got, err := reauth.Verify(context.Background(), store, reauth.Params{
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

	got, err := reauth.Verify(context.Background(), store, reauth.Params{
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

	_, err := reauth.Verify(context.Background(), store, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileImport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, store.updateCalls)

	// Second attempt — store.account now has the consumed-token state.
	_, err = reauth.Verify(context.Background(), store, reauth.Params{
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

	got, err := reauth.Verify(context.Background(), store, reauth.Params{
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

func TestVerify_Token_PersistFailure_PropagatesError(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthProfileExport, "tok", time.Now().Add(5*time.Minute))
	sentinel := errors.New("update failed")
	store := &fakeStore{account: acc, updateErr: sentinel}

	_, err := reauth.Verify(context.Background(), store, reauth.Params{
		AccountId:        "acct",
		TokenType:        auth.TokenTypeReauthProfileExport,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true,
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 1, store.updateCalls)
}

// ---------------------------------------------------------------------------
// VerifyOnAccount - operates on a pre-loaded account, never touches the store
// ---------------------------------------------------------------------------

func TestVerifyOnAccount_NilAccount_ReturnsInvalidToken(t *testing.T) {
	err := reauth.VerifyOnAccount(nil, reauth.Params{ReauthToken: ptr("tok")})
	assert.ErrorIs(t, err, reauth.ErrInvalidReauthToken)
}

func TestVerifyOnAccount_NoToken_ReturnsMissingAuthMethod(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthEmailChange, "tok", time.Now().Add(5*time.Minute))
	err := reauth.VerifyOnAccount(acc, reauth.Params{TokenType: auth.TokenTypeReauthEmailChange})
	assert.ErrorIs(t, err, reauth.ErrMissingAuthMethod)
	assert.Len(t, acc.Tokens, 1)
}

func TestVerifyOnAccount_Token_Match_ConsumesInPlace_NoPersist(t *testing.T) {
	// The whole point of VerifyOnAccount: caller owns persistence.
	acc := tokenAccount(auth.TokenTypeReauthEmailChange, "tok", time.Now().Add(5*time.Minute))
	err := reauth.VerifyOnAccount(acc, reauth.Params{
		TokenType:        auth.TokenTypeReauthEmailChange,
		ReauthToken:      ptr("tok"),
		PersistOnConsume: true, // must be ignored
	})
	require.NoError(t, err)
	assert.Empty(t, acc.Tokens, "matched token must be consumed in place")
}

func TestVerifyOnAccount_Token_Expired_ReturnsExpired(t *testing.T) {
	acc := tokenAccount(auth.TokenTypeReauthAccountDeletion, "tok", time.Now().Add(-1*time.Minute))
	err := reauth.VerifyOnAccount(acc, reauth.Params{
		TokenType:   auth.TokenTypeReauthAccountDeletion,
		ReauthToken: ptr("tok"),
	})
	assert.ErrorIs(t, err, reauth.ErrReauthTokenExpired)
	assert.Len(t, acc.Tokens, 1, "expired token must not be consumed")
}

// ---------------------------------------------------------------------------
// Purposes and issuance
// ---------------------------------------------------------------------------

func TestTokenType_EveryPurposeHasAReauthTokenType(t *testing.T) {
	for purpose, tokenType := range reauth.Purposes {
		got, ok := reauth.TokenType(purpose)
		require.True(t, ok, purpose)
		assert.Equal(t, tokenType, got)
		_, err := auth.NewToken(got)
		assert.NoError(t, err, "auth.NewToken must know %s", got)
	}
	_, ok := reauth.TokenType("login")
	assert.False(t, ok)
}

func TestNew_TokenOfThePurposeThatVerifyConsumes(t *testing.T) {
	tok, err := reauth.New("profile_export")
	require.NoError(t, err)
	assert.Equal(t, auth.TokenTypeReauthProfileExport, tok.Type)
	assert.NotEmpty(t, tok.Value)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), tok.ExpiresAt, 5*time.Second)

	acc := &model.Account{Tokens: []model.Token{{Type: auth.TokenTypePasswordReset, Value: "pr", ExpiresAt: time.Now().Add(time.Hour)}, *tok}}
	require.NoError(t, reauth.VerifyOnAccount(acc, reauth.Params{TokenType: auth.TokenTypeReauthProfileExport, ReauthToken: &tok.Value}))
	require.Len(t, acc.Tokens, 1)
	assert.Equal(t, "pr", acc.Tokens[0].Value)
}

// AccountRepository.AddReauthToken prunes expired tokens by this type prefix.
func TestNew_EveryReauthTokenTypeHasTheReauthPrefix(t *testing.T) {
	for purpose, tokenType := range reauth.Purposes {
		assert.True(t, strings.HasPrefix(tokenType, "reauth_"), purpose)
	}
	assert.False(t, strings.HasPrefix(auth.TokenTypePasswordReset, "reauth_"))
}

func TestNew_UnknownPurpose(t *testing.T) {
	_, err := reauth.New("login")
	assert.ErrorIs(t, err, reauth.ErrUnknownPurpose)
}
