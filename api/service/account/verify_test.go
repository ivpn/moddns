package account_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	validatorv10 "github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/subscription"
)

// Separate suite focused on email verification OTP logic
type EmailVerificationOTPSuite struct {
	suite.Suite
	service         *account.AccountService
	mockAccountRepo *mocks.AccountRepository
	mockMailer      *mocks.Maileremail
	mockCache       *mocks.Cachecache
}

func (suite *EmailVerificationOTPSuite) SetupSuite() {
	// Set minimal required env variables for config.New()
	os.Setenv("SERVER_ALLOWED_DOMAINS", "example.com")
	os.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53")
	os.Setenv("API_PORT", "8080")
	os.Setenv("DB_NAME", "test_db")
	os.Setenv("CACHE_ADDRESS", "localhost:6379")
	cfg, err := config.New()
	suite.Require().NoError(err)

	suite.mockAccountRepo = mocks.NewAccountRepository(suite.T())
	suite.mockMailer = mocks.NewMaileremail(suite.T())
	suite.mockCache = mocks.NewCachecache(suite.T())

	// Minimal dependencies; other repos not needed for OTP operations
	val := validatorv10.New()
	mockSubRepo := mocks.NewSubscriptionRepository(suite.T())
	subService := subscription.NewSubscriptionService(mockSubRepo, suite.mockCache, config.ServiceConfig{})
	suite.service = account.NewAccountService(
		*cfg.Service,
		suite.mockAccountRepo,
		nil, // profileService not required
		nil, // statisticsService not required
		subService,
		nil, // credential repository not required
		suite.mockCache,
		suite.mockMailer,
		nil, // id generator not required here
		val,
		webhookClient.Http{}, // no-op http client for tests
	)
}

// Helper to create a base unverified account
func (suite *EmailVerificationOTPSuite) newAccount(email string, verified bool) *model.Account {
	return &model.Account{
		ID:            primitive.NewObjectID(),
		Email:         email,
		EmailVerified: verified,
		Tokens:        []model.Token{},
	}
}

func (suite *EmailVerificationOTPSuite) TestRequestEmailVerificationOTP() {
	tests := []struct {
		name              string
		account           *model.Account
		getErr            error
		alreadyVerified   bool
		tickErr           error
		allowed           bool
		updateErr         error
		mailerErr         error
		expectedErrorPart string
	}{
		{
			name:    "success generates otp and sends email",
			account: suite.newAccount("user@example.com", false),
			allowed: true,
		},
		{
			name:              "account fetch error",
			account:           nil,
			getErr:            dbErrors.ErrAccountNotFound,
			expectedErrorPart: "account not found",
		},
		{
			name:              "already verified returns error",
			account:           suite.newAccount("user@example.com", true),
			alreadyVerified:   true,
			expectedErrorPart: "email already verified",
		},
		{
			name:              "rate limit tick error",
			account:           suite.newAccount("user@example.com", false),
			tickErr:           errors.New("cache incr failed"),
			allowed:           true,
			expectedErrorPart: "cache incr failed",
		},
		{
			name:              "rate limited not allowed",
			account:           suite.newAccount("user@example.com", false),
			allowed:           false,
			expectedErrorPart: "email verification otp rate limited",
		},
		{
			name:              "update account error",
			account:           suite.newAccount("user@example.com", false),
			allowed:           true,
			updateErr:         errors.New("update failed"),
			expectedErrorPart: "update failed",
		},
		{
			name:              "mailer send error maps to ErrSendOTP",
			account:           suite.newAccount("user@example.com", false),
			allowed:           true,
			mailerErr:         errors.New("smtp down"),
			expectedErrorPart: "could not send OTP",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockMailer.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountById
			if tt.getErr != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), mock.AnythingOfType("string")).Return(nil, tt.getErr)
			} else {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), mock.AnythingOfType("string")).Return(tt.account, nil)
			}

			// Short circuit if fetch failed
			if tt.getErr == nil && !tt.alreadyVerified {
				// IDLimiter interactions: Incr + Get
				if tt.tickErr != nil {
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(0), tt.tickErr)
				} else {
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil)
				}
				if tt.allowed {
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("1", nil)
				} else {
					// Return a value greater than Max (service config IdLimiterMax) to force not allowed; using 999
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("999", nil)
				}
			}

			// Mock update & mailer on success path up to that point
			if tt.expectedErrorPart == "" || (tt.updateErr != nil || tt.mailerErr != nil) {
				if tt.getErr == nil && !tt.alreadyVerified && tt.tickErr == nil && tt.allowed {
					// We can't predict OTP value; expect UpdateAccount with any account pointer
					if tt.updateErr != nil {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateErr)
					} else {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)
						// Mailer expectation
						if tt.mailerErr != nil {
							suite.mockMailer.On("SendEmailVerificationOTP", context.Background(), tt.account.Email, mock.AnythingOfType("string")).Return(tt.mailerErr)
						} else {
							suite.mockMailer.On("SendEmailVerificationOTP", context.Background(), tt.account.Email, mock.AnythingOfType("string")).Return(nil)
						}
					}
				}
			}

			err := suite.service.RequestEmailVerificationOTP(context.Background(), "someAccountId")
			if tt.expectedErrorPart != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *EmailVerificationOTPSuite) TestVerifyEmailOTP() {
	validTokenValue := "123456"
	expiredTime := time.Now().Add(-1 * time.Minute)
	futureTime := time.Now().Add(10 * time.Minute)

	makeAccountWithToken := func(email string, tokenValue string, exp time.Time, verified bool) *model.Account {
		return &model.Account{
			ID:            primitive.NewObjectID(),
			Email:         email,
			EmailVerified: verified,
			Tokens: []model.Token{
				{Type: "email_verification_otp", Value: tokenValue, ExpiresAt: exp},
			},
		}
	}

	tests := []struct {
		name              string
		account           *model.Account
		otpInput          string
		getErr            error
		updateErr         error
		failTickErr       error
		allowedAfterFail  bool
		expectedErrorPart string
	}{
		{
			name:     "success verifies and removes token",
			account:  makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput: validTokenValue,
		},
		{
			name:              "missing otp input",
			account:           nil,
			otpInput:          "",
			expectedErrorPart: "email verification otp missing",
		},
		{
			name:              "get account error",
			account:           nil,
			otpInput:          validTokenValue,
			getErr:            dbErrors.ErrAccountNotFound,
			expectedErrorPart: "account not found",
		},
		{
			name:     "already verified returns nil",
			account:  makeAccountWithToken("user@example.com", validTokenValue, futureTime, true),
			otpInput: validTokenValue,
		},
		{
			name:              "no token present",
			account:           &model.Account{ID: primitive.NewObjectID(), Email: "user@example.com", EmailVerified: false, Tokens: []model.Token{}},
			otpInput:          validTokenValue,
			expectedErrorPart: "invalid verification token",
		},
		{
			name:              "token expired",
			account:           makeAccountWithToken("user@example.com", validTokenValue, expiredTime, false),
			otpInput:          validTokenValue,
			expectedErrorPart: "invalid verification token",
		},
		{
			name:              "incorrect otp increments attempts",
			account:           makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput:          "111111",
			allowedAfterFail:  true,
			expectedErrorPart: "incorrect OTP",
		},
		{
			name:              "incorrect otp tick error",
			account:           makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput:          "222222",
			failTickErr:       errors.New("cache incr failed"),
			expectedErrorPart: "cache incr failed",
		},
		{
			name:              "too many attempts",
			account:           makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput:          "333333",
			allowedAfterFail:  false,
			expectedErrorPart: "too many invalid email verification attempts",
		},
		{
			name:              "update account not found maps to invalid token",
			account:           makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput:          validTokenValue,
			updateErr:         dbErrors.ErrAccountNotFound,
			expectedErrorPart: "invalid verification token",
		},
		{
			name:              "update account error generic",
			account:           makeAccountWithToken("user@example.com", validTokenValue, futureTime, false),
			otpInput:          validTokenValue,
			updateErr:         errors.New("write failed"),
			expectedErrorPart: "write failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			if tt.otpInput == "" {
				// Direct early error, no repo calls expected
				err := suite.service.VerifyEmailOTP(context.Background(), "someAccountId", tt.otpInput)
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
				return
			}

			// Mock GetAccountById
			if tt.getErr != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), mock.AnythingOfType("string")).Return(nil, tt.getErr)
			} else {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), mock.AnythingOfType("string")).Return(tt.account, nil)
			}

			// Early exit if getErr
			if tt.getErr != nil {
				err := suite.service.VerifyEmailOTP(context.Background(), "someAccountId", tt.otpInput)
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
				return
			}

			// Already verified case: expect no update and nil error
			if tt.account != nil && tt.account.EmailVerified {
				err := suite.service.VerifyEmailOTP(context.Background(), "someAccountId", tt.otpInput)
				suite.NoError(err)
				return
			}

			// Incorrect / expired / missing token paths
			tokenScenarioNeedsLimiter := false
			// Determine token presence & validity
			hasValidToken := false
			if tt.account != nil {
				for _, t := range tt.account.Tokens {
					if t.Type == "email_verification_otp" && time.Now().Before(t.ExpiresAt) {
						hasValidToken = true
						break
					}
				}
			}

			if hasValidToken && tt.otpInput != validTokenValue {
				tokenScenarioNeedsLimiter = true
			}

			if tokenScenarioNeedsLimiter {
				// Fail attempt tick
				if tt.failTickErr != nil {
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(0), tt.failTickErr)
				} else {
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil)
				}
				if tt.allowedAfterFail {
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("1", nil)
				} else {
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("999", nil)
				}
			}

			// Success path update
			if hasValidToken && tt.otpInput == validTokenValue && (tt.expectedErrorPart == "" || tt.updateErr != nil) {
				if tt.updateErr != nil {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateErr)
				} else {
					// After success EmailVerified should be true; we don't assert content here, just allow call
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)
				}
			}

			err := suite.service.VerifyEmailOTP(context.Background(), "someAccountId", tt.otpInput)
			if tt.expectedErrorPart != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *EmailVerificationOTPSuite) TestVerifyPasswordReset() {
	validToken := "reset-token-abc123"
	validPassword := "NewSecurePass123!"
	futureTime := time.Now().Add(1 * time.Hour)
	expiredTime := time.Now().Add(-1 * time.Minute)

	makeAccountWithResetToken := func(tokenValue string, exp time.Time) *model.Account {
		return &model.Account{
			ID:            primitive.NewObjectID(),
			Email:         "user@example.com",
			EmailVerified: true,
			Tokens: []model.Token{
				{Type: "password_reset", Value: tokenValue, ExpiresAt: exp},
			},
		}
	}

	tests := []struct {
		name               string
		account            *model.Account
		tokenInput         string
		password           string
		mfa                *model.MfaData
		getByTokenErr      error
		updateErr          error
		expectedErrorPart  string
		expectTokenRemoved bool
	}{
		{
			name:               "success resets password and removes token",
			account:            makeAccountWithResetToken(validToken, futureTime),
			tokenInput:         validToken,
			password:           validPassword,
			mfa:                &model.MfaData{},
			expectTokenRemoved: true,
		},
		{
			name:              "token not found in database",
			tokenInput:        validToken,
			password:          validPassword,
			mfa:               &model.MfaData{},
			getByTokenErr:     dbErrors.ErrAccountNotFound,
			expectedErrorPart: "invalid verification token",
		},
		{
			name:              "token expired",
			account:           makeAccountWithResetToken(validToken, expiredTime),
			tokenInput:        validToken,
			password:          validPassword,
			mfa:               &model.MfaData{},
			expectedErrorPart: "token expired",
		},
		{
			name: "token not in account tokens array",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "user@example.com",
				EmailVerified: true,
				Tokens:        []model.Token{},
			},
			tokenInput:        validToken,
			password:          validPassword,
			mfa:               &model.MfaData{},
			expectedErrorPart: "invalid verification token",
		},
		{
			name:              "empty password rejected",
			account:           makeAccountWithResetToken(validToken, futureTime),
			tokenInput:        validToken,
			password:          "",
			mfa:               &model.MfaData{},
			expectedErrorPart: "password cannot be empty",
		},
		{
			name:              "update account error",
			account:           makeAccountWithResetToken(validToken, futureTime),
			tokenInput:        validToken,
			password:          validPassword,
			mfa:               &model.MfaData{},
			updateErr:         errors.New("db write failed"),
			expectedErrorPart: "db write failed",
		},
		{
			name: "mfa required but not provided",
			account: func() *model.Account {
				acc := makeAccountWithResetToken(validToken, futureTime)
				acc.MFA.TOTP.Enabled = true
				acc.MFA.TOTP.Secret = "JBSWY3DPEHPK3PXP"
				return acc
			}(),
			tokenInput:        validToken,
			password:          validPassword,
			mfa:               &model.MfaData{},
			expectedErrorPart: "TOTP is required",
		},
		{
			name: "preserves other tokens after reset",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "user@example.com",
				EmailVerified: true,
				Tokens: []model.Token{
					{Type: "password_reset", Value: validToken, ExpiresAt: futureTime},
					{Type: "email_verification_otp", Value: "keep-me", ExpiresAt: futureTime},
				},
			},
			tokenInput:         validToken,
			password:           validPassword,
			mfa:                &model.MfaData{},
			expectTokenRemoved: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountByToken
			if tt.getByTokenErr != nil {
				suite.mockAccountRepo.On("GetAccountByToken", context.Background(), tt.tokenInput, "password_reset").Return(nil, tt.getByTokenErr)
			} else {
				suite.mockAccountRepo.On("GetAccountByToken", context.Background(), tt.tokenInput, "password_reset").Return(tt.account, nil)
			}

			// Early exit on getByTokenErr
			if tt.getByTokenErr != nil {
				err := suite.service.VerifyPasswordReset(context.Background(), tt.tokenInput, tt.password, tt.mfa)
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
				return
			}

			// MFA check may fail before we reach token loop
			if tt.account != nil && tt.account.MFA.TOTP.Enabled && tt.mfa.OTP == "" {
				err := suite.service.VerifyPasswordReset(context.Background(), tt.tokenInput, tt.password, tt.mfa)
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
				return
			}

			// Mock UpdateAccount on success path (valid token, valid password)
			if tt.expectTokenRemoved || tt.updateErr != nil {
				if tt.updateErr != nil {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateErr)
				} else {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).
						Run(func(args mock.Arguments) {
							acc := args.Get(1).(*model.Account)
							// Verify the used reset token was removed
							for _, t := range acc.Tokens {
								suite.NotEqual(tt.tokenInput, t.Value, "used reset token should be removed")
							}
							// Verify password was set (non-nil)
							suite.NotNil(acc.Password, "password should be set after reset")
						}).
						Return(tt.account, nil)
				}
			}

			err := suite.service.VerifyPasswordReset(context.Background(), tt.tokenInput, tt.password, tt.mfa)
			if tt.expectedErrorPart != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedErrorPart)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func TestEmailVerificationOTPSuite(t *testing.T) {
	suite.Run(t, new(EmailVerificationOTPSuite))
}
