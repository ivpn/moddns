package profile_test

// verify_reauth_test.go — unit tests for ProfileService.verifyReauth.
// specRef: M4, M5, M6

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
)

// ptr is a convenience helper that returns a pointer to a string literal.
func ptr(s string) *string { return &s }

// newTestService builds a minimal ProfileService wired to the given
// account-repository mock for verifyReauth testing.
func newTestService(t *testing.T, accountRepo *mocks.AccountRepository) *profile.ProfileService {
	t.Helper()
	profileRepo := mocks.NewProfileRepository(t)
	return &profile.ProfileService{
		ProfileRepository: profileRepo,
		AccountRepository: accountRepo,
	}
}

// bcryptHash creates a bcrypt hash of password for use in test Account records.
func bcryptHash(t *testing.T, password string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(b)
}

// TestVerifyReauth_BothNil — both credentials nil returns ErrReauthRequired.
// specRef: M5
func TestVerifyReauth_BothNil(t *testing.T) {
	accountRepo := mocks.NewAccountRepository(t)
	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, nil, nil)
	assert.ErrorIs(t, err, profile.ErrReauthRequired)
}

// TestVerifyReauth_WrongPassword — wrong password returns ErrReauthInvalid.
// specRef: M6
func TestVerifyReauth_WrongPassword(t *testing.T) {
	hash := bcryptHash(t, "correctPassword")
	acc := &model.Account{
		Password: &hash,
	}

	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(acc, nil)

	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, ptr("wrongPassword"), nil)
	assert.ErrorIs(t, err, profile.ErrReauthInvalid)
}

// TestVerifyReauth_CorrectPassword — correct password returns no error.
// specRef: M6
func TestVerifyReauth_CorrectPassword(t *testing.T) {
	hash := bcryptHash(t, "correctPassword")
	acc := &model.Account{
		Password: &hash,
	}

	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(acc, nil)

	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, ptr("correctPassword"), nil)
	assert.NoError(t, err)
}

// TestVerifyReauth_WrongReauthToken — token that does not match returns ErrReauthInvalid.
// specRef: M6
func TestVerifyReauth_WrongReauthToken(t *testing.T) {
	acc := &model.Account{
		Tokens: []model.Token{
			{
				Type:      auth.TokenTypeReauthProfileExport,
				Value:     "real-token-value",
				ExpiresAt: time.Now().Add(5 * time.Minute),
			},
		},
	}

	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(acc, nil)

	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, nil, ptr("wrong-token"))
	assert.ErrorIs(t, err, profile.ErrReauthInvalid)
}

// TestVerifyReauth_CorrectReauthToken — correct token succeeds and is consumed
// (single-use: second call with same token fails).
// specRef: M6
func TestVerifyReauth_CorrectReauthToken_SingleUse(t *testing.T) {
	const tokenValue = "valid-token-abc"

	acc := &model.Account{
		Tokens: []model.Token{
			{
				Type:      auth.TokenTypeReauthProfileExport,
				Value:     tokenValue,
				ExpiresAt: time.Now().Add(5 * time.Minute),
			},
		},
	}

	// After consuming the token, UpdateAccount is called with an empty Tokens slice.
	accountAfterConsume := &model.Account{Tokens: []model.Token{}}

	accountRepo := mocks.NewAccountRepository(t)
	// First call (consumes token)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(acc, nil).Once()
	accountRepo.On("UpdateAccount", context.Background(), &model.Account{Tokens: []model.Token{}}).Return(accountAfterConsume, nil).Once()
	// Second call (token already consumed — no more tokens in account)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(accountAfterConsume, nil).Once()

	svc := newTestService(t, accountRepo)

	// First use — must succeed.
	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, nil, ptr(tokenValue))
	require.NoError(t, err)

	// Second use — same token; must fail because it was consumed.
	err = svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, nil, ptr(tokenValue))
	assert.ErrorIs(t, err, profile.ErrReauthInvalid)
}

// TestVerifyReauth_ExpiredReauthToken — expired token returns ErrReauthInvalid.
// specRef: M6
func TestVerifyReauth_ExpiredReauthToken(t *testing.T) {
	const tokenValue = "expired-token"

	acc := &model.Account{
		Tokens: []model.Token{
			{
				Type:      auth.TokenTypeReauthProfileExport,
				Value:     tokenValue,
				ExpiresAt: time.Now().Add(-1 * time.Minute), // in the past
			},
		},
	}

	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(acc, nil)

	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, nil, ptr(tokenValue))
	assert.ErrorIs(t, err, profile.ErrReauthInvalid)
}

// TestVerifyReauth_AccountNotFound — account lookup failure returns ErrReauthInvalid.
// specRef: M6
func TestVerifyReauth_AccountNotFound(t *testing.T) {
	accountRepo := mocks.NewAccountRepository(t)
	accountRepo.On("GetAccountById", context.Background(), "acct1").Return(nil, dbErrors.ErrAccountNotFound)

	svc := newTestService(t, accountRepo)

	err := svc.ExportVerifyReauthForTest(context.Background(), "acct1", auth.TokenTypeReauthProfileExport, ptr("any-password"), nil)
	assert.ErrorIs(t, err, profile.ErrReauthInvalid)
}
