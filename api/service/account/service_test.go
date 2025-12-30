package account_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/auth"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/blocklist"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/pquerna/otp/totp"
)

type AccountTestSuite struct {
	suite.Suite
	service              *account.AccountService
	mockAccountRepo      *mocks.AccountRepository
	mockProfileRepo      *mocks.ProfileRepository
	mockBlocklistRepo    *mocks.BlocklistRepository
	mockQueryLogsRepo    *mocks.QueryLogsRepository
	mockStatsRepo        *mocks.StatisticsRepository
	mockSubscriptionRepo *mocks.SubscriptionRepository
	mockCache            *mocks.Cachecache
	mockMailer           *mocks.Maileremail
	mockIDGenerator      *mocks.Generatoridgen
	profileService       *profile.ProfileService
	blocklistService     *blocklist.BlocklistService
	queryLogsService     *querylogs.QueryLogsService
	statisticsService    *statistics.StatisticsService
	subscriptionService  *subscription.SubscriptionService
	validator            *validator.Validate
	serviceConfig        config.ServiceConfig
	serverConfig         config.ServerConfig
}

func (suite *AccountTestSuite) SetupSuite() {
	// Set required environment variables
	os.Setenv("SERVER_ALLOWED_DOMAINS", "test.com,allowed.com")
	os.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53,1.1.1.1:53")
	os.Setenv("API_PORT", "8080")
	os.Setenv("DB_NAME", "test_db")
	os.Setenv("CACHE_ADDRESS", "localhost:6379")

	cfg, err := config.New()
	suite.Require().NoError(err, "Failed to create config")

	// Initialize mocks
	suite.mockAccountRepo = mocks.NewAccountRepository(suite.T())
	suite.mockProfileRepo = mocks.NewProfileRepository(suite.T())
	suite.mockBlocklistRepo = mocks.NewBlocklistRepository(suite.T())
	suite.mockQueryLogsRepo = mocks.NewQueryLogsRepository(suite.T())
	suite.mockStatsRepo = mocks.NewStatisticsRepository(suite.T())
	suite.mockCache = mocks.NewCachecache(suite.T())
	suite.mockMailer = mocks.NewMaileremail(suite.T())
	suite.mockIDGenerator = mocks.NewGeneratoridgen(suite.T())
	suite.validator = validator.New()

	// Extract configs
	suite.serviceConfig = *cfg.Service
	suite.serverConfig = *cfg.Server

	// Create dependency services with mocks
	suite.blocklistService = blocklist.NewBlocklistService(suite.mockBlocklistRepo, suite.mockCache)
	suite.queryLogsService = querylogs.NewQueryLogsService(suite.mockQueryLogsRepo)
	suite.statisticsService = statistics.NewStatisticsService(suite.mockStatsRepo)
	suite.mockSubscriptionRepo = mocks.NewSubscriptionRepository(suite.T())
	suite.subscriptionService = subscription.NewSubscriptionService(suite.mockSubscriptionRepo, suite.mockCache, suite.serviceConfig)

	// Create the profile service with mocks
	suite.profileService = profile.NewProfileService(
		suite.serverConfig,
		suite.serviceConfig,
		suite.mockProfileRepo,
		suite.blocklistService,
		suite.queryLogsService,
		suite.statisticsService,
		suite.mockCache,
		suite.mockIDGenerator,
		suite.validator,
	)

	// Create the AccountService with mocks
	suite.service = account.NewAccountService(
		suite.serviceConfig,
		suite.mockAccountRepo,
		suite.profileService,
		suite.statisticsService,
		suite.subscriptionService,
		nil, // credential repository not required for these tests
		suite.mockCache,
		suite.mockMailer,
		suite.mockIDGenerator,
		suite.validator,
		webhookClient.Http{}, // no-op http client for tests
	)
}

// TestRegisterAccount tests the RegisterAccount method using table-driven tests
func (suite *AccountTestSuite) TestRegisterAccount() {
	subID := "550e8400-e29b-41d4-a716-446655440000" // test subscription UUID
	activeUntilStr := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	tests := []struct {
		name               string
		email              string
		password           string
		existingAccount    *model.Account
		getAccountError    error
		profileCreateError error
		createAccountError error
		expectedError      string
		expectSuccess      bool
	}{
		{
			name:          "Successful registration",
			email:         "test@example.com",
			password:      "StrongPass123!",
			expectSuccess: true,
		},
		{
			name:            "Account already exists",
			email:           "existing@example.com",
			password:        "StrongPass123!",
			existingAccount: &model.Account{Email: "existing@example.com"},
			expectedError:   "account with this email already exists",
		},
		{
			name:            "Database error on check",
			email:           "test@example.com",
			password:        "StrongPass123!",
			getAccountError: errors.New("database error"),
			expectedError:   "database error",
		},
		{
			name:               "Profile creation fails",
			email:              "test@example.com",
			password:           "StrongPass123!",
			profileCreateError: errors.New("profile creation failed"),
			expectedError:      "profile creation failed",
		},
		{
			name:               "Account creation fails",
			email:              "test@example.com",
			password:           "StrongPass123!",
			createAccountError: errors.New("account creation failed"),
			expectedError:      "account creation failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockBlocklistRepo.ExpectedCalls = nil
			suite.mockMailer.ExpectedCalls = nil
			suite.mockIDGenerator.ExpectedCalls = nil

			// Mock GetAccountByEmail (switch to satisfy lint ifElseChain)
			switch {
			case tt.getAccountError != nil:
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.email).Return(nil, tt.getAccountError)
			case tt.existingAccount != nil:
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.email).Return(tt.existingAccount, nil)
			default:
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.email).Return(nil, dbErrors.ErrAccountNotFound)
			}

			if tt.expectSuccess || tt.profileCreateError != nil || tt.createAccountError != nil {
				// Mock profile creation dependencies
				if tt.profileCreateError != nil {
					suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), mock.AnythingOfType("string")).Return([]model.Profile{}, nil)
					suite.mockIDGenerator.On("Generate").Return("", tt.profileCreateError)
				} else if tt.expectSuccess || tt.createAccountError != nil {
					suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), mock.AnythingOfType("string")).Return([]model.Profile{}, nil)
					suite.mockIDGenerator.On("Generate").Return("profile123", nil)

					// Mock default blocklists for profile creation
					defaultBlocklists := []*model.Blocklist{
						{
							Name:    "Default Blocklist",
							Default: true,
						},
					}
					suite.mockBlocklistRepo.On("Get", context.Background(), map[string]any{"default": true}, "updated").Return(defaultBlocklists, nil)

					// Mock profile creation in repository
					suite.mockProfileRepo.On("CreateProfile", context.Background(), mock.AnythingOfType("*model.Profile")).Return(nil)

					// Mock cache settings creation
					suite.mockCache.On("CreateOrUpdateProfileSettings", context.Background(), mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil)

					// Mock account creation
					if tt.createAccountError != nil {
						suite.mockAccountRepo.On("CreateAccount", context.Background(), tt.email, mock.AnythingOfType("string"), mock.AnythingOfType("string"), "profile123", mock.Anything).Return(nil, tt.createAccountError)
					} else {
						expectedAccount := &model.Account{
							ID:    primitive.NewObjectID(),
							Email: tt.email,
						}
						suite.mockAccountRepo.On("CreateAccount", context.Background(), tt.email, mock.AnythingOfType("string"), mock.AnythingOfType("string"), "profile123", mock.Anything).Return(expectedAccount, nil)

						// Single insert path; no UpdateAccount expected when password provided (pre-hashed before CreateAccount)

						// Mock verify + welcome email
						suite.mockMailer.On("Verify", tt.email).Return(nil)
						suite.mockMailer.On("SendWelcomeEmail", context.Background(), tt.email, mock.AnythingOfType("string")).Return(nil)
					}
				}
			}

			// Execute the method
			// Common expectation: cache provides subscription activeUntil
			suite.mockCache.On("GetSubscription", context.Background(), subID).Return(activeUntilStr, nil)
			// Expect subscription creation only on success path (no earlier errors)
			if tt.expectSuccess && tt.profileCreateError == nil && tt.createAccountError == nil && tt.getAccountError == nil && tt.existingAccount == nil {
				suite.mockSubscriptionRepo.On("Create", context.Background(), mock.AnythingOfType("model.Subscription")).Return(nil)
				// Expect removal of subscription cache marker
				suite.mockCache.On("RemoveSubscription", context.Background(), subID).Return(nil)
			}
			result, err := suite.service.RegisterAccount(context.Background(), tt.email, tt.password, subID)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				suite.Equal(tt.email, result.Email)
			}
		})
	}
}

// TestGetUnfinishedSignupOrPostAccount covers registration reuse & creation scenarios
func (suite *AccountTestSuite) TestGetUnfinishedSignupOrPostAccount() {
	subID := "550e8400-e29b-41d4-a716-446655449999"
	activeUntil := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)

	password := "StrongPass123!" // valid password

	type mocksConfig struct {
		cacheErr                         error
		existingAccount                  *model.Account
		subscriptionDuplicate            bool // subscription UUID already exists (GetSubscriptionById returns object)
		subscriptionMissingForUnfinished bool // unfinished account has no subscription; will be created
	}

	tests := []struct {
		name        string
		cfg         mocksConfig
		expectError string
	}{
		{
			name:        "Cache key missing -> error",
			cfg:         mocksConfig{cacheErr: errors.New("redis: nil")},
			expectError: account.ErrUnableToCreateAccount.Error(),
		},
		{
			name:        "Subscription duplicate pre-create -> error",
			cfg:         mocksConfig{subscriptionDuplicate: true},
			expectError: account.ErrUnableToCreateAccount.Error(),
		},
		{
			name: "New account creation success",
			cfg:  mocksConfig{},
		},
		{
			name: "Reuse unfinished account creates subscription and sets password",
			cfg:  mocksConfig{existingAccount: &model.Account{ID: primitive.NewObjectID(), Email: "unfinished@example.com"}, subscriptionMissingForUnfinished: true},
		},
		{
			name: "Finished account reuse attempt (password present) -> error",
			cfg: func() mocksConfig {
				pw := "hash"
				return mocksConfig{existingAccount: &model.Account{ID: primitive.NewObjectID(), Email: "fin@example.com", Password: &pw}}
			}(),
			expectError: account.ErrUnableToCreateAccount.Error(),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset expectations
			suite.mockCache.ExpectedCalls = nil
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockSubscriptionRepo.ExpectedCalls = nil
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockBlocklistRepo.ExpectedCalls = nil
			suite.mockMailer.ExpectedCalls = nil
			suite.mockIDGenerator.ExpectedCalls = nil

			// Cache GetSubscription behavior
			if tt.cfg.cacheErr != nil {
				suite.mockCache.On("GetSubscription", context.Background(), subID).Return("", tt.cfg.cacheErr)
			} else {
				suite.mockCache.On("GetSubscription", context.Background(), subID).Return(activeUntil, nil)
			}

			email := "user@example.com"
			if tt.cfg.existingAccount != nil {
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.cfg.existingAccount.Email).Return(tt.cfg.existingAccount, nil)
			} else if tt.cfg.subscriptionDuplicate {
				// New account path; email not found
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), email).Return(nil, dbErrors.ErrAccountNotFound)
			} else if tt.cfg.cacheErr == nil { // normal new account path
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), email).Return(nil, dbErrors.ErrAccountNotFound)
			}

			// Duplicate subscription detection (pre-create) only for new account path
			if tt.cfg.existingAccount == nil {
				if tt.cfg.subscriptionDuplicate {
					// GetSubscriptionById returns existing subscription object
					suite.mockSubscriptionRepo.On("GetSubscriptionById", context.Background(), subID).Return(&model.Subscription{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")}, nil)
				} else if tt.cfg.cacheErr == nil { // default non-duplicate
					suite.mockSubscriptionRepo.On("GetSubscriptionById", context.Background(), subID).Return(nil, dbErrors.ErrSubscriptionNotFound)
				}
			}

			// Unfinished reuse path: ensure subscription creation if missing (finished accounts return early; no subscription calls)
			if tt.cfg.existingAccount != nil && tt.cfg.existingAccount.Password == nil {
				suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.MatchedBy(func(a *model.Account) bool {
					if a.Email != tt.cfg.existingAccount.Email || a.Password == nil {
						return false
					}
					return bcrypt.CompareHashAndPassword([]byte(*a.Password), []byte(password)) == nil
				})).Return(tt.cfg.existingAccount, nil).Once()
				if tt.cfg.subscriptionMissingForUnfinished {
					suite.mockSubscriptionRepo.On("GetSubscriptionByAccountId", context.Background(), tt.cfg.existingAccount.ID.Hex()).Return(nil, dbErrors.ErrSubscriptionNotFound)
					suite.mockSubscriptionRepo.On("Create", context.Background(), mock.AnythingOfType("model.Subscription")).Return(nil)
				} else {
					suite.mockSubscriptionRepo.On("GetSubscriptionByAccountId", context.Background(), tt.cfg.existingAccount.ID.Hex()).Return(&model.Subscription{}, nil)
				}
				suite.mockMailer.On("Verify", tt.cfg.existingAccount.Email).Return(nil)
				suite.mockMailer.On("SendWelcomeEmail", context.Background(), tt.cfg.existingAccount.Email, mock.AnythingOfType("string")).Return(nil)
			}

			// New account creation path expectations
			if tt.cfg.existingAccount == nil && tt.cfg.cacheErr == nil && !tt.cfg.subscriptionDuplicate {
				// Profile service path: list existing profiles returns empty
				suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), mock.AnythingOfType("string")).Return([]model.Profile{}, nil)
				// ID generator for profile
				suite.mockIDGenerator.On("Generate").Return("profile123", nil)
				suite.mockBlocklistRepo.On("Get", context.Background(), map[string]any{"default": true}, "updated").Return([]*model.Blocklist{{Name: "Default Blocklist", Default: true}}, nil)
				suite.mockProfileRepo.On("CreateProfile", context.Background(), mock.AnythingOfType("*model.Profile")).Return(nil)
				suite.mockCache.On("CreateOrUpdateProfileSettings", context.Background(), mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil)
				suite.mockAccountRepo.On("CreateAccount", context.Background(), email, password, mock.AnythingOfType("string"), "profile123", mock.Anything).Return(&model.Account{ID: primitive.NewObjectID(), Email: email, Password: &password}, nil)
				suite.mockSubscriptionRepo.On("Create", context.Background(), mock.AnythingOfType("model.Subscription")).Return(nil)
				suite.mockCache.On("RemoveSubscription", context.Background(), subID).Return(nil)
				suite.mockMailer.On("Verify", email).Return(nil)
				suite.mockMailer.On("SendWelcomeEmail", context.Background(), email, mock.AnythingOfType("string")).Return(nil)
			}

			// For reuse unfinished: cache removal
			if tt.cfg.existingAccount != nil && tt.cfg.cacheErr == nil {
				suite.mockCache.On("RemoveSubscription", context.Background(), subID).Return(nil)
			}

			// Execute
			var targetEmail string
			if tt.cfg.existingAccount != nil {
				targetEmail = tt.cfg.existingAccount.Email
			} else {
				targetEmail = email
			}
			result, err := suite.service.GetUnfinishedSignupOrPostAccount(context.Background(), targetEmail, password, subID)

			if tt.expectError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				suite.Equal(targetEmail, result.Email)
			}

			// Cleanup expectations to avoid leaking into other suite tests
			suite.mockSubscriptionRepo.AssertExpectations(suite.T())
			suite.mockSubscriptionRepo.ExpectedCalls = nil
			suite.mockSubscriptionRepo.Calls = nil
			suite.mockAccountRepo.AssertExpectations(suite.T())
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockAccountRepo.Calls = nil
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockProfileRepo.Calls = nil
			suite.mockBlocklistRepo.ExpectedCalls = nil
			suite.mockBlocklistRepo.Calls = nil
			suite.mockMailer.ExpectedCalls = nil
			suite.mockMailer.Calls = nil
		})
	}
}

// TestGetUnfinishedSignupOrPostAccount_PasskeyFlows adds coverage for passkey-only signup scenarios
// ensuring side-effects (webhook/email/cache removal) only occur when a password is provided.

// TestGetAccount tests the GetAccount method using table-driven tests
func (suite *AccountTestSuite) TestGetAccount() {
	tests := []struct {
		name          string
		accountID     string
		account       *model.Account
		repoError     error
		expectedError string
		expectSuccess bool
	}{
		{
			name:      "Successful get",
			accountID: "507f1f77bcf86cd799439011",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
			},
			expectSuccess: true,
		},
		{
			name:          "Account not found",
			accountID:     "507f1f77bcf86cd799439011",
			repoError:     dbErrors.ErrAccountNotFound,
			expectedError: "account not found",
		},
		{
			name:          "Database error",
			accountID:     "507f1f77bcf86cd799439011",
			repoError:     errors.New("database error"),
			expectedError: "database error",
		},
		{
			name:          "Invalid account ID",
			accountID:     "invalid-id",
			repoError:     errors.New("invalid ObjectId"),
			expectedError: "invalid ObjectId",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil

			// Mock repository call for all IDs
			if tt.repoError != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.repoError)
			} else {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil)
				// Mock GetAccountMetrics dependencies
				for _, profileID := range tt.account.Profiles {
					stats := []model.StatisticsAggregated{
						{
							Total: 100,
						},
					}
					suite.mockStatsRepo.On("GetProfileStatistics", context.Background(), profileID, 720).Return(stats, nil)
				}
			}

			// Execute the method
			result, err := suite.service.GetAccount(context.Background(), tt.accountID)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				suite.Equal(tt.account.Email, result.Email)
			}
		})
	}
}

// TestGetAccountMetrics tests the GetAccountMetrics method
func (suite *AccountTestSuite) TestGetAccountMetrics() {
	tests := []struct {
		name          string
		account       *model.Account
		timespan      string
		statsError    error
		expectedError string
		expectSuccess bool
	}{
		{
			name: "Successful metrics retrieval",
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Profiles: []string{"profile1", "profile2"},
			},
			timespan:      "LAST_1_DAY",
			expectSuccess: true,
		},
		{
			name: "Statistics error",
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Profiles: []string{"profile1"},
			},
			timespan:      "LAST_1_DAY",
			statsError:    errors.New("stats error"),
			expectedError: "stats error",
		},
		{
			name: "No profiles",
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Profiles: []string{},
			},
			timespan:      "LAST_1_DAY",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockStatsRepo.ExpectedCalls = nil
			suite.mockProfileRepo.ExpectedCalls = nil

			// Mock statistics calls for each profile
			for _, profileID := range tt.account.Profiles {
				stats := []model.StatisticsAggregated{
					{
						Total: 100,
					},
				}

				// Mock profile validation in profile service (always successful)
				profile := &model.Profile{
					ProfileId: profileID,
					AccountId: tt.account.ID.Hex(),
				}
				suite.mockProfileRepo.On("GetProfileById", context.Background(), profileID).Return(profile, nil)

				if tt.statsError != nil {
					suite.mockStatsRepo.On("GetProfileStatistics", context.Background(), profileID, 24).Return(nil, tt.statsError)
					break // Only need one error to trigger the test condition
				} else {
					suite.mockStatsRepo.On("GetProfileStatistics", context.Background(), profileID, 24).Return(stats, nil)
				}
			}

			// Execute the method
			result, err := suite.service.GetAccountMetrics(context.Background(), tt.account, tt.timespan)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
			}
		})
	}
}

// TestSendResetPasswordEmail tests the SendResetPasswordEmail method
func (suite *AccountTestSuite) TestSendResetPasswordEmail() {
	tests := []struct {
		name               string
		email              string
		account            *model.Account
		getAccountError    error
		updateAccountError error
		mailerVerifyError  error
		mailerSendError    error
		expectedError      string
		expectSuccess      bool
	}{
		{
			name:  "Successful reset email",
			email: "test@example.com",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "test@example.com",
				EmailVerified: true,
			},
			expectSuccess: true,
		},
		{
			name:            "Account not found",
			email:           "notfound@example.com",
			getAccountError: dbErrors.ErrAccountNotFound,
			expectedError:   "account not found",
		},
		{
			name:            "Database error on get account",
			email:           "test@example.com",
			getAccountError: errors.New("database error"),
			expectedError:   "database error",
		},
		{
			name:  "Email not verified (suppressed)",
			email: "unverified@example.com",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "unverified@example.com",
				EmailVerified: false,
			},
			expectedError: "Email delivery is not possible until your address is verified.",
		},
		// TODO: data race
		// {
		// 	name:  "Update account fails",
		// 	email: "test@example.com",
		// 	account: &model.Account{
		// 		ID:            primitive.NewObjectID(),
		// 		Email:         "test@example.com",
		// 		EmailVerified: true,
		// 	},
		// 	updateAccountError: errors.New("update failed"),
		// 	expectedError:      "update failed",
		// },
		{
			name:  "Mailer verify fails",
			email: "test@example.com",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "test@example.com",
				EmailVerified: true,
			},
			mailerVerifyError: errors.New("verify failed"),
			expectedError:     "verify failed",
		},
		{
			name:  "Mailer send fails",
			email: "test@example.com",
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "test@example.com",
				EmailVerified: true,
			},
			mailerSendError: errors.New("send failed"),
			expectedError:   "send failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockMailer.ExpectedCalls = nil

			// Mock GetAccountByEmail
			if tt.getAccountError != nil {
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.email).Return(nil, tt.getAccountError)
			} else {
				suite.mockAccountRepo.On("GetAccountByEmail", context.Background(), tt.email).Return(tt.account, nil)

				// Mock UpdateAccount if account is found
				if tt.updateAccountError != nil {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateAccountError)
				} else {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)

					// Always expect Verify() call
					if tt.mailerVerifyError != nil {
						suite.mockMailer.On("Verify", tt.email).Return(tt.mailerVerifyError)
					} else {
						suite.mockMailer.On("Verify", tt.email).Return(nil)
					}

					// Only expect SendPasswordResetEmail when not suppressed
					if tt.account.EmailVerified {
						if tt.mailerSendError != nil {
							suite.mockMailer.On("SendPasswordResetEmail", context.Background(), tt.email, mock.AnythingOfType("string")).Return(tt.mailerSendError)
						} else {
							suite.mockMailer.On("SendPasswordResetEmail", context.Background(), tt.email, mock.AnythingOfType("string")).Return(nil)
						}
					}
				}
			}

			// Execute the method
			err := suite.service.SendResetPasswordEmail(context.Background(), tt.email)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}
		})
	}
}

// TestUpdateAccount tests the UpdateAccount method
func (suite *AccountTestSuite) TestUpdateAccount() {
	dbErr := errors.New("db error")
	updateFailedErr := errors.New("update failed")
	accountNotFoundErr := errors.New("account not found")
	mfaCheckFailedErr := errors.New("mfa check failed")
	profileUpdateFailedErr := errors.New("profile update failed")

	tests := []struct {
		name                string
		accountID           string
		updates             []model.AccountUpdate
		mfa                 *model.MfaData
		account             *model.Account
		getAccountError     error
		updateAccountError  error
		mfaError            error
		profileUpdateError  error
		expectedErr         error
		expectValidationErr bool
		skipReason          string
	}{
		{
			name:      "Successful error reports consent update",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{
					Operation: "replace",
					Path:      "/error_reports_consent",
					Value:     true,
				},
			},
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
			},
		},
		{
			name:      "Successful password update",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!@#"},
			},
			mfa: &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "test@example.com", Password: &h}
			}(),
		},
		{
			name:      "Successful password set when account has no current password",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!@#"},
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "nopass@example.com",
			},
		},
		{
			name:      "Password update without test fails when current password exists",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/password", Value: "AnotherStrongPassword123!"},
			},
			mfa: &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "test@example.com", Password: &h}
			}(),
			expectedErr: account.ErrPasswordTestRequired,
		},
		{
			name:      "Profile update only",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{
					Operation: "add",
					Path:      "/profiles",
					Value:     "newprofile123",
				},
			},
		},
		{
			name:      "No updates",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{},
		},
		{
			name:      "Successful email update resets verification and sends OTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
		},
		{
			name:      "Email update wrong current password",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "wrong", "new_email": "new@example.com"}}},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			expectedErr: account.ErrInvalidCurrentPassword,
		},
		{
			name:      "Email update missing fields",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "missing@example.com"}}},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			expectedErr: account.ErrMissingAuthMethod,
		},
		{
			name:      "Email update invalid format",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "invalid"}}},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			expectValidationErr: true,
		},
		{
			name:      "Email update with same email address",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "old@example.com"}}},
			mfa:       &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			expectedErr: account.ErrSameEmailAddress,
		},
		{
			name:      "Email update with same email address case insensitive",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "OLD@EXAMPLE.COM"}}},
			mfa:       &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			expectedErr: account.ErrSameEmailAddress,
		},
		{
			name:      "Email update requires TOTP when enabled and OTP missing",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}}},
			mfa:       &model.MfaData{OTP: ""},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true, MFA: model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}}
			}(),
			expectedErr: account.ErrTOTPRequired,
		},
		{
			name:      "Email update wrong TOTP code",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}}},
			mfa:       &model.MfaData{OTP: "000000"},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true, MFA: model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}}
			}(),
			expectedErr: account.ErrInvalidTOTPCode,
		},
		{
			name:            "Email update account fetch error",
			accountID:       "507f1f77bcf86cd799439011",
			updates:         []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}}},
			getAccountError: dbErr,
			expectedErr:     dbErr,
		},
		{
			name:      "Email update persistence error",
			accountID: "507f1f77bcf86cd799439011",
			updates:   []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}}},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "old@example.com", Password: &h, EmailVerified: true}
			}(),
			updateAccountError: updateFailedErr,
			expectedErr:        updateFailedErr,
		},
		{
			name:      "Get account fails",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{
					Operation: "replace",
					Path:      "/error_reports_consent",
					Value:     true,
				},
			},
			getAccountError: accountNotFoundErr,
			expectedErr:     accountNotFoundErr,
		},
		{
			name:      "MFA check fails for password update",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "newpassword123"},
			},
			mfa: &model.MfaData{},
			account: func() *model.Account {
				b, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
				h := string(b)
				return &model.Account{ID: primitive.NewObjectID(), Email: "test@example.com", Password: &h}
			}(),
			mfaError:   mfaCheckFailedErr,
			skipReason: "MFA check testing requires more complex mocking",
		},
		{
			name:      "Update account fails",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{
					Operation: "replace",
					Path:      "/error_reports_consent",
					Value:     true,
				},
			},
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
			},
			updateAccountError: updateFailedErr,
			expectedErr:        updateFailedErr,
		},
		{
			name:      "Profile update fails",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{
					Operation: "add",
					Path:      "/profiles",
					Value:     "newprofile123",
				},
			},
			profileUpdateError: profileUpdateFailedErr,
			expectedErr:        profileUpdateFailedErr,
		},
	}

	for _, tt := range tests {
		tt := tt
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil

			if tt.account != nil && tt.account.MFA.TOTP.Enabled && tt.accountID != "" {
				if parsedID, err := primitive.ObjectIDFromHex(tt.accountID); err == nil {
					tt.account.ID = parsedID
				}
			}

			// Check if this test case has profile updates
			hasProfileUpdates := false
			hasOtherUpdates := false
			for _, update := range tt.updates {
				if update.Path == "/profiles" {
					hasProfileUpdates = true
				} else {
					hasOtherUpdates = true
				}
			}

			// Mock profile updates if needed
			if hasProfileUpdates {
				if tt.profileUpdateError != nil {
					// For profile updates, we need to mock the atomic update operation
					suite.mockAccountRepo.On("AddProfileToAccount", context.Background(), tt.accountID, mock.AnythingOfType("string")).Return(tt.profileUpdateError)
				} else {
					suite.mockAccountRepo.On("AddProfileToAccount", context.Background(), tt.accountID, mock.AnythingOfType("string")).Return(nil)
				}
			}

			// Mock other updates if needed
			if hasOtherUpdates {
				if tt.getAccountError != nil {
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
				} else {
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil)

					if tt.expectedErr == account.ErrInvalidTOTPCode && tt.account != nil {
						key := "totp_fails:" + tt.account.ID.Hex()
						suite.mockCache.On("Incr", mock.Anything, key, suite.serviceConfig.IdLimiterExpiration).Return(int64(1), nil)
						suite.mockCache.On("Get", mock.Anything, key).Return("1", nil)
					}

					// Check if password update is involved
					hasPasswordUpdate := false
					for _, update := range tt.updates {
						if update.Path == "/password" {
							hasPasswordUpdate = true
							break
						}
					}

					if hasPasswordUpdate && tt.mfaError != nil {
						// TODO(mfa-mock): Implement MFA failure path mocking; capture error reference to avoid empty branch
						_ = tt.mfaError
					}

					if tt.updateAccountError != nil {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateAccountError)
					} else if tt.expectedErr == nil {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)
					}
				}
			}

			// Execute the method
			err := suite.service.UpdateAccount(context.Background(), tt.accountID, tt.updates, tt.mfa)

			if tt.skipReason != "" {
				suite.T().Skip(tt.skipReason)
			}

			switch {
			case tt.expectValidationErr:
				suite.Require().Error(err)
				var valErr validator.ValidationErrors
				suite.ErrorAs(err, &valErr)
				suite.NotEmpty(valErr)
			case tt.expectedErr != nil:
				suite.Require().Error(err)
				suite.ErrorIs(err, tt.expectedErr)
			default:
				suite.NoError(err)
			}
		})
	}
}

// TestDeleteAccount tests the DeleteAccount method
func (suite *AccountTestSuite) TestDeleteAccount() {
	validExpires := time.Now().Add(10 * time.Minute)
	expiredExpires := time.Now().Add(-10 * time.Minute)
	strPtr := func(s string) *string { return &s }
	reauthValue := "reauth-token"
	passwordValue := "CurrentPassword123!"
	backupCode := "backup-code-123"

	totpRequireID := primitive.NewObjectID()
	totpRequireAccount := &model.Account{
		ID:                  totpRequireID,
		Email:               "test@example.com",
		DeletionCode:        "DELETE123",
		DeletionCodeExpires: &validExpires,
		MFA: model.MFASettings{
			TOTP: model.TotpSettings{
				Enabled: true,
			},
		},
	}

	totpSuccessID := primitive.NewObjectID()
	totpSuccessAccount := &model.Account{
		ID:                  totpSuccessID,
		Email:               "test@example.com",
		DeletionCode:        "DELETE123",
		DeletionCodeExpires: &validExpires,
		Profiles:            []string{"profile1", "profile2"},
		MFA: model.MFASettings{
			TOTP: model.TotpSettings{
				Enabled:     true,
				BackupCodes: []string{backupCode},
			},
		},
	}

	totpVerifyAccount := &model.Account{
		ID:                  totpSuccessID,
		Email:               "test@example.com",
		DeletionCode:        "DELETE123",
		DeletionCodeExpires: &validExpires,
		MFA: model.MFASettings{
			TOTP: model.TotpSettings{
				Enabled:     true,
				BackupCodes: []string{backupCode},
			},
		},
	}

	tests := []struct {
		name               string
		accountID          string
		request            requests.AccountDeletionRequest
		mfa                *model.MfaData
		account            *model.Account
		getAccountError    error
		deleteProfileError error
		deleteAccountError error
		expectedError      string
		setupProfiles      bool
		setupDeleteAccount bool
		verifyTotpAccount  *model.Account
	}{
		{
			name:      "Successful deletion",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: &validExpires,
				Profiles:            []string{"profile1", "profile2"},
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			setupProfiles:      true,
			setupDeleteAccount: true,
		},
		{
			name:      "Account not found",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa:             &model.MfaData{},
			getAccountError: dbErrors.ErrAccountNotFound,
			expectedError:   "account not found",
		},
		{
			name:      "Invalid deletion code",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "WRONG123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: &validExpires,
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			expectedError: "invalid deletion code",
		},
		{
			name:      "Deletion code expired",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: &expiredExpires,
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			expectedError: "deletion code expired",
		},
		{
			name:      "No expiration time set",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: nil,
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			expectedError: "deletion code expired",
		},
		{
			name:      "Password auth requires TOTP",
			accountID: totpRequireID.Hex(),
			request: requests.AccountDeletionRequest{
				DeletionCode:    "DELETE123",
				CurrentPassword: strPtr(passwordValue),
			},
			mfa:           &model.MfaData{},
			account:       totpRequireAccount,
			expectedError: account.ErrTOTPRequired.Error(),
		},
		{
			name:      "Password auth with TOTP backup code",
			accountID: totpSuccessID.Hex(),
			request: requests.AccountDeletionRequest{
				DeletionCode:    "DELETE123",
				CurrentPassword: strPtr(passwordValue),
			},
			mfa: &model.MfaData{
				OTP: backupCode,
			},
			account:            totpSuccessAccount,
			setupProfiles:      true,
			setupDeleteAccount: true,
			verifyTotpAccount:  totpVerifyAccount,
		},
		{
			name:      "Profile deletion fails",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: &validExpires,
				Profiles:            []string{"profile1"},
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			deleteProfileError: errors.New("profile deletion failed"),
			expectedError:      "profile deletion failed",
			setupProfiles:      true,
		},
		{
			name:      "Account deletion fails",
			accountID: "507f1f77bcf86cd799439011",
			request: requests.AccountDeletionRequest{
				DeletionCode: "DELETE123",
				ReauthToken:  strPtr(reauthValue),
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:                  primitive.NewObjectID(),
				Email:               "test@example.com",
				DeletionCode:        "DELETE123",
				DeletionCodeExpires: &validExpires,
				Profiles:            []string{},
				Tokens: []model.Token{{
					Type:      auth.TokenTypeReauthAccountDeletion,
					Value:     reauthValue,
					ExpiresAt: validExpires,
				}},
			},
			deleteAccountError: errors.New("account deletion failed"),
			expectedError:      "account deletion failed",
			setupProfiles:      true,
			setupDeleteAccount: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockProfileRepo.ExpectedCalls = nil

			// Prepare account state when password verification is required
			if tt.account != nil && tt.request.CurrentPassword != nil {
				err := tt.account.SetPassword(*tt.request.CurrentPassword)
				suite.Require().NoError(err)
			}
			if tt.verifyTotpAccount != nil && tt.request.CurrentPassword != nil && tt.verifyTotpAccount.Password == nil {
				err := tt.verifyTotpAccount.SetPassword(*tt.request.CurrentPassword)
				suite.Require().NoError(err)
			}

			// Mock GetAccount
			if tt.getAccountError != nil {
				suite.mockAccountRepo.On("GetAccount", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
			} else {
				suite.mockAccountRepo.On("GetAccount", context.Background(), tt.accountID).Return(tt.account, nil)

				shouldProcess := tt.account != nil && tt.account.DeletionCode == tt.request.DeletionCode && tt.account.DeletionCodeExpires != nil && time.Now().Before(*tt.account.DeletionCodeExpires)
				if shouldProcess && tt.verifyTotpAccount != nil {
					lookupID := tt.account.ID.Hex()
					suite.mockAccountRepo.On("GetAccountById", context.Background(), lookupID).Return(tt.verifyTotpAccount, nil)
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.verifyTotpAccount, nil)
				}

				if shouldProcess && tt.setupProfiles {
					for _, profileID := range tt.account.Profiles {
						if tt.deleteProfileError != nil {
							suite.mockProfileRepo.On("GetProfileById", context.Background(), profileID).Return(&model.Profile{
								ProfileId: profileID,
								AccountId: tt.accountID,
							}, tt.deleteProfileError)
							break
						}

						suite.mockProfileRepo.On("GetProfileById", context.Background(), profileID).Return(&model.Profile{
							ProfileId: profileID,
							AccountId: tt.accountID,
						}, nil)
						suite.mockProfileRepo.On("DeleteProfileById", mock.AnythingOfType("*context.cancelCtx"), profileID).Return(nil)
						suite.mockQueryLogsRepo.On("DeleteQueryLogs", context.Background(), profileID).Return(nil)
						suite.mockCache.On("DeleteProfileSettings", context.Background(), profileID).Return(nil)
					}
				}

				if shouldProcess && tt.setupDeleteAccount && tt.deleteProfileError == nil {
					if tt.deleteAccountError != nil {
						suite.mockAccountRepo.On("DeleteAccountById", context.Background(), tt.accountID).Return(tt.deleteAccountError)
					} else {
						suite.mockAccountRepo.On("DeleteAccountById", context.Background(), tt.accountID).Return(nil)
					}
				}
			}

			err := suite.service.DeleteAccount(context.Background(), tt.accountID, tt.request, tt.mfa)

			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}
		})
	}
}

// TestGenerateDeletionCode tests the GenerateDeletionCode method
func (suite *AccountTestSuite) TestGenerateDeletionCode() {
	tests := []struct {
		name                    string
		accountID               string
		updateDeletionCodeError error
		expectedError           string
		expectSuccess           bool
	}{
		{
			name:          "Successful code generation",
			accountID:     "507f1f77bcf86cd799439011",
			expectSuccess: true,
		},
		{
			name:                    "Update deletion code fails",
			accountID:               "507f1f77bcf86cd799439011",
			updateDeletionCodeError: errors.New("update failed"),
			expectedError:           "update failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil

			// Mock UpdateDeletionCode
			if tt.updateDeletionCodeError != nil {
				suite.mockAccountRepo.On("UpdateDeletionCode", context.Background(), tt.accountID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(tt.updateDeletionCodeError)
			} else {
				suite.mockAccountRepo.On("UpdateDeletionCode", context.Background(), tt.accountID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
			}

			// Execute the method
			result, err := suite.service.GenerateDeletionCode(context.Background(), tt.accountID)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				suite.NotEmpty(result.Code)
				suite.False(result.ExpiresAt.IsZero())
			}
		})
	}
}

// TestTotpEnable tests the TOTP enablement/initialization process
func (suite *AccountTestSuite) TestTotpEnable() {
	tests := []struct {
		name               string
		accountID          string
		account            *model.Account
		getAccountError    error
		setCacheError      error
		expectedError      string
		expectTOTPGenerate bool
	}{
		{
			name:      "Success - Generate TOTP secret successfully",
			accountID: "507f1f77bcf86cd799439011",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectTOTPGenerate: true,
		},
		{
			name:            "Error - Account not found",
			accountID:       "507f1f77bcf86cd799439011",
			getAccountError: errors.New("account not found"),
			expectedError:   "account not found",
		},
		{
			name:      "Error - Cache storage fails",
			accountID: "507f1f77bcf86cd799439011",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			setCacheError: errors.New("cache write failed"),
			expectedError: "cache write failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountById
			if tt.getAccountError != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
			} else {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil)

				// Mock Cache.SetTOTPSecret
				if tt.setCacheError != nil {
					suite.mockCache.On("SetTOTPSecret", context.Background(), tt.accountID, mock.AnythingOfType("string"), suite.serviceConfig.OTPExpirationTime).Return(tt.setCacheError)
				} else {
					suite.mockCache.On("SetTOTPSecret", context.Background(), tt.accountID, mock.AnythingOfType("string"), suite.serviceConfig.OTPExpirationTime).Return(nil)
				}
			}

			// Execute the method
			result, err := suite.service.TotpEnable(context.Background(), tt.accountID)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				suite.NotEmpty(result.Secret)
				suite.NotEmpty(result.URI)
				suite.Equal(tt.account.Email, result.Account)
				suite.Contains(result.URI, "otpauth://totp/modDNS:")
				suite.Contains(result.URI, tt.account.Email)
			}

			// Verify mock expectations
			suite.mockAccountRepo.AssertExpectations(suite.T())
			if tt.account != nil {
				suite.mockCache.AssertExpectations(suite.T())
			}
		})
	}
}

// TestTotpConfirm tests the TOTP confirmation/activation process
func (suite *AccountTestSuite) TestTotpConfirm() {
	testSecret := "JBSWY3DPEHPK3PXP"
	validOTP, _ := totp.GenerateCode(testSecret, time.Now())

	tests := []struct {
		name               string
		accountID          string
		otp                string
		cachedSecret       string
		getCacheError      error
		account            *model.Account
		getAccountError    error
		updateAccountError error
		expectedError      string
		expectBackupCodes  bool
	}{
		{
			name:         "Success - Confirm TOTP with valid code",
			accountID:    "507f1f77bcf86cd799439011",
			otp:          validOTP,
			cachedSecret: testSecret,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectBackupCodes: true,
		},
		{
			name:          "Error - Secret not found in cache",
			accountID:     "507f1f77bcf86cd799439011",
			otp:           validOTP,
			getCacheError: errors.New("cache key not found"),
			expectedError: "cache key not found",
		},
		{
			name:            "Error - Account not found",
			accountID:       "507f1f77bcf86cd799439011",
			otp:             validOTP,
			cachedSecret:    testSecret,
			getAccountError: errors.New("account not found"),
			expectedError:   "account not found",
		},
		{
			name:         "Error - TOTP already enabled",
			accountID:    "507f1f77bcf86cd799439011",
			otp:          validOTP,
			cachedSecret: testSecret,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "2FA already configured",
		},
		{
			name:         "Error - Invalid OTP code",
			accountID:    "507f1f77bcf86cd799439011",
			otp:          "000000",
			cachedSecret: testSecret,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectedError: "incorrect OTP",
		},
		{
			name:         "Error - Account update fails",
			accountID:    "507f1f77bcf86cd799439011",
			otp:          validOTP,
			cachedSecret: testSecret,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			updateAccountError: errors.New("database update failed"),
			expectedError:      "database update failed",
			expectBackupCodes:  false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock Cache.GetTOTPSecret
			if tt.getCacheError != nil {
				suite.mockCache.On("GetTOTPSecret", context.Background(), tt.accountID).Return("", tt.getCacheError)
			} else {
				suite.mockCache.On("GetTOTPSecret", context.Background(), tt.accountID).Return(tt.cachedSecret, nil)

				// Mock GetAccountById
				if tt.getAccountError != nil {
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
				} else {
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil)

					if tt.account != nil && !tt.account.MFA.TOTP.Enabled {
						suite.mockCache.On("Incr", context.Background(), mock.MatchedBy(func(key string) bool {
							return strings.Contains(key, "totp_confirm_fails")
						}), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Maybe()
						suite.mockCache.On("Get", context.Background(), mock.MatchedBy(func(key string) bool {
							return strings.Contains(key, "totp_confirm_fails")
						})).Return("0", nil).Maybe()
					}

					// Mock UpdateAccount if OTP is valid and TOTP not already enabled
					if tt.account != nil && !tt.account.MFA.TOTP.Enabled && totp.Validate(tt.otp, tt.cachedSecret) {
						if tt.updateAccountError != nil {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.MatchedBy(func(acc *model.Account) bool {
								return acc.ID == tt.account.ID && acc.MFA.TOTP.Enabled
							})).Return(nil, tt.updateAccountError)
						} else {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.MatchedBy(func(acc *model.Account) bool {
								return acc.ID == tt.account.ID && acc.MFA.TOTP.Enabled
							})).Return(tt.account, nil)
						}
					}
				}
			}

			// Execute the method
			result, err := suite.service.TotpConfirm(context.Background(), tt.accountID, tt.otp)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				if tt.expectBackupCodes {
					suite.Len(result.BackupCodes, 8, "Should generate 8 backup codes")
					for _, code := range result.BackupCodes {
						suite.Len(code, 16, "Each backup code should be 16 characters")
						suite.NotEmpty(code)
					}
				}
			}

			// Verify mock expectations
			suite.mockCache.AssertExpectations(suite.T())
			if tt.cachedSecret != "" {
				suite.mockAccountRepo.AssertExpectations(suite.T())
			}
		})
	}
}

// TestTotpDisable tests the TOTP disabling process
func (suite *AccountTestSuite) TestTotpDisable() {
	testSecret := "JBSWY3DPEHPK3PXP"
	validOTP, _ := totp.GenerateCode(testSecret, time.Now())
	validBackupCode := "BACKUP123456CODE"

	tests := []struct {
		name               string
		accountID          string
		otp                string
		account            *model.Account
		getAccountError    error
		updateAccountError error
		expectedError      string
		expectSuccess      bool
	}{
		{
			name:      "Success - Disable TOTP with valid OTP",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{"CODE1", "CODE2"},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Disable TOTP with valid backup code",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validBackupCode,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode, "CODE2"},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Error - TOTP already disabled",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectedError: "2FA already disabled",
		},
		{
			name:      "Error - Invalid OTP code",
			accountID: "507f1f77bcf86cd799439011",
			otp:       "000000",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{"CODE1", "CODE2"},
					},
				},
			},
			expectedError: "invalid 2FA code",
		},
		{
			name:      "Error - Backup code already used",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validBackupCode,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:         true,
						Secret:          testSecret,
						BackupCodes:     []string{"CODE2"},
						BackupCodesUsed: []string{validBackupCode},
					},
				},
			},
			expectedError: "2FA backup is already used",
		},
		{
			name:            "Error - Account not found",
			accountID:       "507f1f77bcf86cd799439011",
			otp:             validOTP,
			getAccountError: errors.New("account not found"),
			expectedError:   "account not found",
		},
		{
			name:      "Error - Account update fails",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{"CODE1"},
					},
				},
			},
			updateAccountError: errors.New("database update failed"),
			expectedError:      "database update failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountById (first call in VerifyTotp)
			if tt.getAccountError != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
			} else {
				// First call to GetAccountById in VerifyTotp
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil).Once()

				if tt.account != nil && tt.account.MFA.TOTP.Enabled {
					// Mock cache for rate limiting (used in VerifyTotp)
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("", nil).Maybe()
					suite.mockCache.On("Set", context.Background(), mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil).Maybe()
					suite.mockCache.On("Increment", context.Background(), mock.AnythingOfType("string")).Return(int64(1), nil).Maybe()
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Maybe()

					// Check if verification should succeed
					verifySucceeds := totp.Validate(tt.otp, tt.account.MFA.TOTP.Secret)
					if !verifySucceeds {
						// Check backup codes
						for _, code := range tt.account.MFA.TOTP.BackupCodes {
							if code == tt.otp {
								verifySucceeds = true
								break
							}
						}
						// Check if it's in used codes
						for _, usedCode := range tt.account.MFA.TOTP.BackupCodesUsed {
							if usedCode == tt.otp {
								verifySucceeds = false
								break
							}
						}
					}

					// If verification succeeds, mock the update for VerifyTotp (when backup code used)
					if verifySucceeds && !totp.Validate(tt.otp, tt.account.MFA.TOTP.Secret) {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil).Once()
					}

					// Mock UpdateAccount for the actual disable operation
					if verifySucceeds && tt.updateAccountError == nil {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.MatchedBy(func(acc *model.Account) bool {
							return !acc.MFA.TOTP.Enabled && acc.MFA.TOTP.Secret == "" && len(acc.MFA.TOTP.BackupCodes) == 0
						})).Return(tt.account, nil)
					} else if verifySucceeds && tt.updateAccountError != nil {
						suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateAccountError)
					}
				}
			}

			// Execute the method
			result, err := suite.service.TotpDisable(context.Background(), tt.accountID, tt.otp)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				if tt.expectSuccess {
					suite.False(result.MFA.TOTP.Enabled)
					suite.Empty(result.MFA.TOTP.Secret)
					suite.Empty(result.MFA.TOTP.BackupCodes)
					suite.Empty(result.MFA.TOTP.BackupCodesUsed)
				}
			}

			// Verify mock expectations
			suite.mockAccountRepo.AssertExpectations(suite.T())
		})
	}
}

// TestVerifyTotp tests the TOTP verification logic
func (suite *AccountTestSuite) TestVerifyTotp() {
	testSecret := "JBSWY3DPEHPK3PXP"
	validOTP, _ := totp.GenerateCode(testSecret, time.Now())
	validBackupCode := "BACKUP123456CODE"

	tests := []struct {
		name               string
		accountID          string
		otp                string
		action             string
		account            *model.Account
		getAccountError    error
		updateAccountError error
		rateLimitExceeded  bool
		expectedError      string
		expectSuccess      bool
	}{
		{
			name:      "Success - Valid TOTP code for login action",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Valid TOTP code for disable action",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			action:    "disable",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Valid backup code (not previously used)",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validBackupCode,
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode, "CODE2"},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Error - TOTP not configured (login action)",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectedError: "2FA is not configured",
		},
		{
			name:      "Error - TOTP already disabled (disable action)",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			action:    "disable",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectedError: "2FA already disabled",
		},
		{
			name:      "Error - Invalid action",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validOTP,
			action:    "invalid",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "invalid action",
		},
		{
			name:      "Error - Invalid TOTP code",
			accountID: "507f1f77bcf86cd799439011",
			otp:       "000000",
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "invalid 2FA code",
		},
		{
			name:      "Error - Backup code already used",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validBackupCode,
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:         true,
						Secret:          testSecret,
						BackupCodes:     []string{"CODE2"},
						BackupCodesUsed: []string{validBackupCode},
					},
				},
			},
			expectedError: "2FA backup is already used",
		},
		{
			name:      "Error - Invalid backup code",
			accountID: "507f1f77bcf86cd799439011",
			otp:       "WRONGCODE",
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode},
					},
				},
			},
			expectedError: "invalid 2FA code",
		},
		{
			name:      "Error - Rate limit exceeded",
			accountID: "507f1f77bcf86cd799439011",
			otp:       "000000",
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			rateLimitExceeded: true,
			expectedError:     "invalid 2FA code",
		},
		{
			name:            "Error - Account not found",
			accountID:       "507f1f77bcf86cd799439011",
			otp:             validOTP,
			action:          "login",
			getAccountError: errors.New("account not found"),
			expectedError:   "account not found",
		},
		{
			name:      "Error - Account update fails after backup code use",
			accountID: "507f1f77bcf86cd799439011",
			otp:       validBackupCode,
			action:    "login",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode, "CODE2"},
					},
				},
			},
			updateAccountError: errors.New("database update failed"),
			expectedError:      "database update failed",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountById
			if tt.getAccountError != nil {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
			} else {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil)

				// Set up cache mocks for rate limiting
				if tt.account != nil && (tt.account.MFA.TOTP.Enabled || (tt.action == "disable" && !tt.account.MFA.TOTP.Enabled)) {
					if tt.rateLimitExceeded {
						// Simulate rate limit exceeded - Get returns value > Max
						// Note: Incr is still called before IsAllowed check
						suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(suite.serviceConfig.IdLimiterMax+2), nil)
						suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return(fmt.Sprintf("%d", suite.serviceConfig.IdLimiterMax+1), nil)
					} else {
						suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("0", nil).Maybe()
						suite.mockCache.On("Set", context.Background(), mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil).Maybe()
						suite.mockCache.On("Increment", context.Background(), mock.AnythingOfType("string")).Return(int64(1), nil).Maybe()
						suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Maybe()
					}

					// Check if this is a valid backup code scenario
					isValidBackup := false
					if tt.account.MFA.TOTP.Enabled {
						for _, code := range tt.account.MFA.TOTP.BackupCodes {
							if code == tt.otp {
								// Check if not already used
								alreadyUsed := false
								for _, usedCode := range tt.account.MFA.TOTP.BackupCodesUsed {
									if usedCode == tt.otp {
										alreadyUsed = true
										break
									}
								}
								if !alreadyUsed {
									isValidBackup = true
								}
								break
							}
						}
					}

					// Mock UpdateAccount if backup code is valid
					if isValidBackup {
						if tt.updateAccountError != nil {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateAccountError)
						} else {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)
						}
					}
				}
			}

			// Execute the method
			result, err := suite.service.VerifyTotp(context.Background(), tt.accountID, tt.otp, tt.action)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
				suite.NotNil(result)
				if tt.expectSuccess {
					suite.Equal(tt.account.ID, result.ID)
				}
			}

			// Verify mock expectations
			suite.mockAccountRepo.AssertExpectations(suite.T())
		})
	}
}

// TestMfaCheck tests the MFA validation used in UpdateAccount
func (suite *AccountTestSuite) TestMfaCheck() {
	testSecret := "JBSWY3DPEHPK3PXP"
	validOTP, _ := totp.GenerateCode(testSecret, time.Now())
	validBackupCode := "BACKUP123456CODE"

	tests := []struct {
		name          string
		account       *model.Account
		mfa           *model.MfaData
		expectedError string
		expectSuccess bool
	}{
		{
			name: "Success - TOTP enabled with valid OTP",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			mfa:           &model.MfaData{OTP: validOTP},
			expectSuccess: true,
		},
		{
			name: "Success - TOTP not enabled",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			mfa:           &model.MfaData{OTP: ""},
			expectSuccess: true,
		},
		{
			name: "Success - TOTP enabled with valid backup code",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode, "CODE2"},
					},
				},
			},
			mfa:           &model.MfaData{OTP: validBackupCode},
			expectSuccess: true,
		},
		{
			name: "Error - TOTP enabled but OTP missing",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			mfa:           &model.MfaData{OTP: ""},
			expectedError: "TOTP is required",
		},
		{
			name: "Error - TOTP enabled but invalid OTP",
			account: &model.Account{
				ID:    primitive.NewObjectID(),
				Email: "test@example.com",
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			mfa:           &model.MfaData{OTP: "000000"},
			expectedError: "invalid 2FA code",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			// Mock GetAccountById for VerifyTotp calls
			if tt.account.MFA.TOTP.Enabled && tt.mfa.OTP != "" {
				suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.account.ID.Hex()).Return(tt.account, nil).Maybe()

				// Mock cache for rate limiting
				suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("0", nil).Maybe()
				suite.mockCache.On("Set", context.Background(), mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil).Maybe()
				suite.mockCache.On("Increment", context.Background(), mock.AnythingOfType("string")).Return(int64(1), nil).Maybe()
				suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Maybe()

				// Check if it's a valid backup code
				isValidBackup := false
				for _, code := range tt.account.MFA.TOTP.BackupCodes {
					if code == tt.mfa.OTP {
						isValidBackup = true
						break
					}
				}

				// Mock UpdateAccount if backup code is valid
				if isValidBackup {
					suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil).Maybe()
				}
			}

			// Execute the method
			err := suite.service.MfaCheck(context.Background(), tt.account, tt.mfa)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}
		})
	}
}

// TestUpdateAccountWith2FA tests UpdateAccount integration with 2FA requirements
func (suite *AccountTestSuite) TestUpdateAccountWith2FA() {
	testSecret := "JBSWY3DPEHPK3PXP"
	validOTP, _ := totp.GenerateCode(testSecret, time.Now())
	validBackupCode := "BACKUP123456CODE"
	bp, _ := bcrypt.GenerateFromPassword([]byte("currentPass123!"), 14)
	hashedPassword := string(bp)

	tests := []struct {
		name               string
		accountID          string
		updates            []model.AccountUpdate
		mfa                *model.MfaData
		account            *model.Account
		getAccountError    error
		updateAccountError error
		expectedError      string
		expectSuccess      bool
	}{
		{
			name:      "Success - Password update with valid TOTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{OTP: validOTP},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Error - Password update missing test op",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{OTP: validOTP},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA:      model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: testSecret}},
			},
			expectedError: "password test operation required",
		},
		{
			name:      "Success - Email update with valid TOTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{OTP: validOTP},
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "old@example.com",
				Password:      &hashedPassword,
				EmailVerified: true,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Password update without TOTP when not enabled",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Email update without TOTP when not enabled",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{},
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "old@example.com",
				Password:      &hashedPassword,
				EmailVerified: true,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: false,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Password update with valid backup code",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{OTP: validBackupCode},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:     true,
						Secret:      testSecret,
						BackupCodes: []string{validBackupCode, "CODE2"},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Success - Multiple updates with TOTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
				{Operation: "replace", Path: "/error_reports_consent", Value: true},
			},
			mfa: &model.MfaData{OTP: validOTP},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectSuccess: true,
		},
		{
			name:      "Error - Password update with TOTP enabled but no OTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{OTP: ""},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "TOTP is required",
		},
		{
			name:      "Error - Email update with TOTP enabled but no OTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{OTP: ""},
			account: &model.Account{
				ID:            primitive.NewObjectID(),
				Email:         "old@example.com",
				Password:      &hashedPassword,
				EmailVerified: true,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "TOTP is required",
		},
		{
			name:      "Error - Password update with invalid TOTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "test", Path: "/password", Value: "currentPass123!"},
				{Operation: "replace", Path: "/password", Value: "NewStrongPassword123!"},
			},
			mfa: &model.MfaData{OTP: "000000"},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "test@example.com",
				Password: &hashedPassword,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "invalid 2FA code",
		},
		{
			name:      "Error - Email update with invalid TOTP",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{OTP: "000000"},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "old@example.com",
				Password: &hashedPassword,

				EmailVerified: true,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled: true,
						Secret:  testSecret,
					},
				},
			},
			expectedError: "invalid 2FA code",
		},
		{
			name:      "Error - Email update with backup code already used",
			accountID: "507f1f77bcf86cd799439011",
			updates: []model.AccountUpdate{
				{Operation: "replace", Path: "/email", Value: map[string]any{"current_password": "currentPass123!", "new_email": "new@example.com"}},
			},
			mfa: &model.MfaData{OTP: validBackupCode},
			account: &model.Account{
				ID:       primitive.NewObjectID(),
				Email:    "old@example.com",
				Password: &hashedPassword,

				EmailVerified: true,
				MFA: model.MFASettings{
					TOTP: model.TotpSettings{
						Enabled:         true,
						Secret:          testSecret,
						BackupCodes:     []string{"CODE2"},
						BackupCodesUsed: []string{validBackupCode},
					},
				},
			},
			// Note: The actual error returned depends on rate limiting logic
			// When backup code is already used, it goes through rate limiting
			// and returns either "2FA backup is already used" or "invalid 2FA code"
			expectedError: "2FA",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations
			suite.mockAccountRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil
			suite.mockMailer.ExpectedCalls = nil
			suite.mockIDGenerator.ExpectedCalls = nil

			// Separate profile updates from other updates
			hasProfileUpdates := false
			hasOtherUpdates := false
			for _, update := range tt.updates {
				if update.Path == "/profiles" {
					hasProfileUpdates = true
				} else {
					hasOtherUpdates = true
				}
			}

			// Mock profile updates if needed
			if hasProfileUpdates {
				for _, update := range tt.updates {
					if update.Path == "/profiles" {
						switch update.Operation {
						case "add":
							suite.mockAccountRepo.On("AddProfileToAccount", context.Background(), tt.accountID, mock.AnythingOfType("string")).Return(nil)
						case "remove":
							suite.mockAccountRepo.On("RemoveProfileFromAccount", context.Background(), tt.accountID, mock.AnythingOfType("string")).Return(nil)
						}
					}
				}
			}

			// Mock other updates if needed
			if hasOtherUpdates {
				if tt.getAccountError != nil {
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(nil, tt.getAccountError)
				} else {
					// First GetAccountById for the main update
					suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.accountID).Return(tt.account, nil).Once()

					// Setup mocks for MFA verification if account has TOTP enabled
					if tt.account.MFA.TOTP.Enabled && tt.mfa != nil && tt.mfa.OTP != "" {
						// Mock GetAccountById for VerifyTotp
						suite.mockAccountRepo.On("GetAccountById", context.Background(), tt.account.ID.Hex()).Return(tt.account, nil).Maybe()

						// Mock cache for rate limiting
						suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return("0", nil).Maybe()
						suite.mockCache.On("Set", context.Background(), mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil).Maybe()
						suite.mockCache.On("Increment", context.Background(), mock.AnythingOfType("string")).Return(int64(1), nil).Maybe()
						suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(int64(1), nil).Maybe()

						// Check if it's a valid backup code
						isValidBackup := false
						for _, code := range tt.account.MFA.TOTP.BackupCodes {
							if code == tt.mfa.OTP {
								// Check if not already used
								alreadyUsed := false
								for _, usedCode := range tt.account.MFA.TOTP.BackupCodesUsed {
									if usedCode == tt.mfa.OTP {
										alreadyUsed = true
										break
									}
								}
								if !alreadyUsed {
									isValidBackup = true
								}
								break
							}
						}

						// Mock UpdateAccount for backup code use
						if isValidBackup {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil).Once()
						}
					}

					// Mock UpdateAccount for the actual account update (if MFA check passes)
					shouldUpdateSucceed := true
					if tt.account.MFA.TOTP.Enabled {
						if tt.mfa == nil || tt.mfa.OTP == "" {
							shouldUpdateSucceed = false
						} else if !totp.Validate(tt.mfa.OTP, tt.account.MFA.TOTP.Secret) {
							// Check backup codes
							isValidBackup := false
							for _, code := range tt.account.MFA.TOTP.BackupCodes {
								if code == tt.mfa.OTP {
									// Check if not already used
									alreadyUsed := false
									for _, usedCode := range tt.account.MFA.TOTP.BackupCodesUsed {
										if usedCode == tt.mfa.OTP {
											alreadyUsed = true
											break
										}
									}
									if !alreadyUsed {
										isValidBackup = true
									}
									break
								}
							}
							shouldUpdateSucceed = isValidBackup
						}
					}

					if shouldUpdateSucceed {
						// Mock for email updates that require additional operations
						for _, update := range tt.updates {
							if update.Path == "/email" {
								suite.mockIDGenerator.On("Generate").Return("verification-token-123", nil)
								suite.mockCache.On("Set", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(nil)
								suite.mockMailer.On("SendEmailVerificationOTP", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
							}
						}

						if tt.updateAccountError != nil {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(nil, tt.updateAccountError)
						} else {
							suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(tt.account, nil)
						}
					}
				}
			}

			// Execute the method
			err := suite.service.UpdateAccount(context.Background(), tt.accountID, tt.updates, tt.mfa)

			// Verify results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}

			// Verify mock expectations
			suite.mockAccountRepo.AssertExpectations(suite.T())
		})
	}
}

func TestAccountTestSuite(t *testing.T) {
	suite.Run(t, new(AccountTestSuite))
}
