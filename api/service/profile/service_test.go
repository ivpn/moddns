package profile_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/ivpn/dns/api/config"
	intvldtr "github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/blocklist"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
)

type ProfileTestSuite struct {
	suite.Suite
	service            *profile.ProfileService
	mockProfileRepo    *mocks.ProfileRepository
	mockBlocklistRepo  *mocks.BlocklistRepository
	mockQueryLogsRepo  *mocks.QueryLogsRepository
	mockStatisticsRepo *mocks.StatisticsRepository
	mockCache          *mocks.Cachecache
	mockIDGen          *mocks.Generatoridgen
	blocklistService   *blocklist.BlocklistService
	queryLogsService   *querylogs.QueryLogsService
	statisticsService  *statistics.StatisticsService
	validator          *validator.Validate
	serverConfig       config.ServerConfig
	serviceConfig      config.ServiceConfig
}

func (suite *ProfileTestSuite) SetupSuite() {
	// Set required environment variables
	os.Setenv("SERVER_ALLOWED_DOMAINS", "test.com,allowed.com")
	os.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53,1.1.1.1:53")

	// Set optional environment variables with default values
	os.Setenv("API_PORT", "8080")
	os.Setenv("DB_NAME", "test_db")
	os.Setenv("CACHE_ADDRESS", "localhost:6379")

	cfg, err := config.New()
	suite.Require().NoError(err, "Failed to create config")

	// Initialize mocks
	suite.mockProfileRepo = mocks.NewProfileRepository(suite.T())
	suite.mockBlocklistRepo = mocks.NewBlocklistRepository(suite.T())
	suite.mockQueryLogsRepo = mocks.NewQueryLogsRepository(suite.T())
	suite.mockStatisticsRepo = mocks.NewStatisticsRepository(suite.T())
	suite.mockCache = mocks.NewCachecache(suite.T())
	suite.mockIDGen = mocks.NewGeneratoridgen(suite.T())
	suite.validator = validator.New()

	// Extract configs
	suite.serverConfig = *cfg.Server
	suite.serviceConfig = *cfg.Service

	// Create the BlocklistService with mocked dependencies
	suite.blocklistService = blocklist.NewBlocklistService(suite.mockBlocklistRepo, suite.mockCache)

	// Create the QueryLogsService with mocked dependencies
	suite.queryLogsService = querylogs.NewQueryLogsService(suite.mockQueryLogsRepo)

	// Create the StatisticsService with mocked dependencies
	suite.statisticsService = statistics.NewStatisticsService(suite.mockStatisticsRepo)

	// Create the ProfileService with mocks
	suite.service = profile.NewProfileService(
		suite.serverConfig,
		suite.serviceConfig,
		suite.mockProfileRepo,
		suite.blocklistService,
		suite.queryLogsService,
		suite.statisticsService,
		suite.mockCache,
		suite.mockIDGen,
		suite.validator,
	)
}

// TestCreateProfile tests the CreateProfile method using table-driven tests
func (suite *ProfileTestSuite) TestCreateProfile() {
	tests := []struct {
		name             string
		profileName      string
		accountID        string
		maxProfiles      int
		existingProfiles []model.Profile
		repoCreateError  error
		repoGetError     error
		idGenError       error
		expectedError    string
		expectedName     string
	}{
		{
			name:             "Successful profile creation",
			profileName:      "Test Profile",
			accountID:        "account123",
			maxProfiles:      5,
			existingProfiles: []model.Profile{},
			expectedError:    "",
			expectedName:     "Test Profile",
		},
		{
			name:          "Empty profile name",
			profileName:   "",
			accountID:     "account123",
			maxProfiles:   5,
			expectedError: "profile name cannot be empty",
		},
		{
			name:        "Profile name already exists",
			profileName: "Existing Profile",
			accountID:   "account123",
			maxProfiles: 5,
			existingProfiles: []model.Profile{
				{Name: "Existing Profile", AccountId: "account123"},
			},
			expectedError: "profile with this name already exists",
		},
		{
			name:        "Maximum profiles limit reached",
			profileName: "New Profile",
			accountID:   "account123",
			maxProfiles: 2,
			existingProfiles: []model.Profile{
				{Name: "Profile 1", AccountId: "account123"},
				{Name: "Profile 2", AccountId: "account123"},
			},
			expectedError: "maximum number of profiles reached",
		},
		{
			name:          "ID generation fails",
			profileName:   "Test Profile",
			accountID:     "account123",
			maxProfiles:   5,
			idGenError:    errors.New("id generation failed"),
			expectedError: "id generation failed",
		},
		{
			name:            "Repository create fails",
			profileName:     "Test Profile",
			accountID:       "account123",
			maxProfiles:     5,
			repoCreateError: errors.New("repo create error"),
			expectedError:   "repo create error",
		},
		{
			name:          "Repository get fails",
			profileName:   "Test Profile",
			accountID:     "account123",
			maxProfiles:   5,
			repoGetError:  errors.New("repo get error"),
			expectedError: "repo get error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockIDGen.ExpectedCalls = nil
			suite.mockBlocklistRepo.ExpectedCalls = nil

			// Update service config for this test case
			originalMaxProfiles := suite.service.ServiceConfig.MaxProfiles
			suite.service.ServiceConfig.MaxProfiles = tt.maxProfiles
			defer func() {
				suite.service.ServiceConfig.MaxProfiles = originalMaxProfiles
			}()

			// Configure mock expectations
			if tt.repoGetError != nil {
				suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(nil, tt.repoGetError)
			} else {
				suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(tt.existingProfiles, nil)
			}

			if tt.profileName != "" {
				// ID generation happens early, so mock it for any non-empty name
				if tt.idGenError != nil {
					suite.mockIDGen.On("Generate").Return("", tt.idGenError)
				} else {
					suite.mockIDGen.On("Generate").Return("profile123", nil)

					// Mock blocklist service call (for createSettings)
					defaultBlocklists := []*model.Blocklist{
						{
							Name:    "Default Blocklist",
							Default: true,
						},
					}
					suite.mockBlocklistRepo.On("Get", context.Background(), map[string]any{"default": true}, "updated").Return(defaultBlocklists, nil)

					// Mock cache call for saving profile settings
					suite.mockCache.On("CreateOrUpdateProfileSettings", context.Background(), mock.AnythingOfType("*model.ProfileSettings"), true).Return(nil)
				}

				if tt.repoGetError == nil && tt.idGenError == nil {
					// Repository calls happen after profile creation and settings setup
					// Check if we should expect CreateProfile to be called
					nameExists := false
					for _, p := range tt.existingProfiles {
						if p.Name == tt.profileName && p.AccountId == tt.accountID {
							nameExists = true
							break
						}
					}

					// Only expect CreateProfile call if name doesn't exist AND max profiles not reached
					if !nameExists && len(tt.existingProfiles) < tt.maxProfiles {
						if tt.repoCreateError != nil {
							suite.mockProfileRepo.On("CreateProfile", context.Background(), mock.AnythingOfType("*model.Profile")).Return(tt.repoCreateError)
						} else {
							suite.mockProfileRepo.On("CreateProfile", context.Background(), mock.AnythingOfType("*model.Profile")).Return(nil)
						}
					}
				}
			}

			// Call the method under test
			profile, err := suite.service.CreateProfile(context.Background(), tt.profileName, tt.accountID)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(profile)
			} else {
				suite.NoError(err)
				suite.NotNil(profile)
				suite.Equal(tt.expectedName, profile.Name)
				suite.Equal(tt.accountID, profile.AccountId)
				suite.NotEmpty(profile.ProfileId)
			}
		})
	}
}

// TestGetProfile tests the GetProfile method
func (suite *ProfileTestSuite) TestGetProfile() {
	tests := []struct {
		name            string
		profileID       string
		accountID       string
		existingProfile *model.Profile
		repoError       error
		expectedError   string
		expectProfile   bool
	}{
		{
			name:      "Successfully get profile",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:            "Profile not found",
			profileID:       "nonexistent",
			accountID:       "account123",
			existingProfile: nil,
			expectedError:   "not found",
			expectProfile:   false,
		},
		{
			name:          "Repository error",
			profileID:     "profile123",
			accountID:     "account123",
			repoError:     errors.New("repo error"),
			expectedError: "repo error",
			expectProfile: false,
		},
		{
			name:      "Profile belongs to different account",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account456", // Different account
				Name:      "Test Profile",
			},
			expectedError: "not found",
			expectProfile: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil

			switch {
			case tt.repoError != nil:
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoError)
			case tt.existingProfile != nil:
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)
			default:
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, errors.New("not found"))
			}

			// Call the method under test
			profile, err := suite.service.GetProfile(context.Background(), tt.accountID, tt.profileID)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(profile)
			} else {
				suite.NoError(err)
				if tt.expectProfile {
					suite.NotNil(profile)
					suite.Equal(tt.accountID, profile.AccountId)
				} else {
					suite.Nil(profile)
				}
			}
		})
	}
}

// TestUpdateProfile tests the UpdateProfile method - expanded to cover all possible update paths
func (suite *ProfileTestSuite) TestUpdateProfile() {
	tests := []struct {
		name                 string
		profileID            string
		accountID            string
		updates              []model.ProfileUpdate
		existingProfile      *model.Profile
		existingProfiles     []model.Profile // For duplicate name checks
		repoGetError         error
		repoUpdateError      error
		cacheError           error
		expectedError        string
		expectProfile        bool
		shouldMockDuplicates bool
	}{
		// Basic profile name update tests
		{
			name:      "Successfully update profile name",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			expectedError:        "",
			expectProfile:        true,
		},
		{
			name:      "Profile name update with duplicate name",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Existing Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles: []model.Profile{
				{Name: "Existing Profile", AccountId: "account123"},
			},
			expectedError: "profile with this name already exists",
			expectProfile: false,
		},
		{
			name:      "Profile name update with empty name",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			expectedError:        "profile name cannot be empty",
			expectProfile:        false,
		},
		{
			name:      "Profile name update with non-string value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     123,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			expectedError:        "",
			expectProfile:        true,
		},

		// Query logs settings tests
		{
			name:      "Successfully update query logs enabled",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/enabled",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Enabled: false},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update query logs enabled with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/enabled",
					Value:     "invalid",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Enabled: false},
				},
			},
			expectedError: "parsing",
			expectProfile: false,
		},
		{
			name:      "Successfully update query logs log_clients_ips",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/log_clients_ips",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{LogClientsIPs: false},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update query logs log_clients_ips with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/log_clients_ips",
					Value:     "not_a_bool",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{LogClientsIPs: false},
				},
			},
			expectedError: "parsing",
			expectProfile: false,
		},
		{
			name:      "Successfully update query logs log_domains",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/log_domains",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{LogDomains: false},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update query logs log_domains with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/log_domains",
					Value:     []string{"invalid"},
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{LogDomains: false},
				},
			},
			expectedError: "unable to cast",
			expectProfile: false,
		},
		{
			name:      "Successfully update query logs retention",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "1w",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update query logs retention with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "invalid",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "invalid retention",
			expectProfile: false,
		},
		{
			name:      "Update query logs retention with non-string value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     12345,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "invalid retention",
			expectProfile: false,
		},

		// Statistics settings tests
		{
			name:      "Successfully update statistics enabled",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/statistics/enabled",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Statistics: &model.StatisticsSettings{Enabled: false},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update statistics enabled with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/statistics/enabled",
					Value:     "invalid",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Statistics: &model.StatisticsSettings{Enabled: false},
				},
			},
			expectedError: "parsing",
			expectProfile: false,
		},

		// Privacy settings tests
		{
			name:      "Successfully update privacy default_rule to block",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/default_rule",
					Value:     model.DEFAULT_RULE_BLOCK,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update privacy default_rule to allow",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/default_rule",
					Value:     model.DEFAULT_RULE_ALLOW,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_BLOCK,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update privacy default_rule with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/default_rule",
					Value:     "invalid",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "default rule action is invalid",
			expectProfile: false,
		},
		{
			name:      "Update privacy default_rule with non-string value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/default_rule",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "default rule action is invalid",
			expectProfile: false,
		},
		{
			name:      "Successfully update privacy blocklists_subdomains_rule to block",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/blocklists_subdomains_rule",
					Value:     model.ACTION_BLOCK,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update privacy blocklists_subdomains_rule to allow",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/blocklists_subdomains_rule",
					Value:     model.ACTION_ALLOW,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_BLOCK,
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update privacy blocklists_subdomains_rule with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/blocklists_subdomains_rule",
					Value:     "invalid",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "blocklists_subdomains_rule value is invalid",
			expectProfile: false,
		},
		{
			name:      "Update privacy blocklists_subdomains_rule with non-string value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/blocklists_subdomains_rule",
					Value:     false,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
				},
			},
			expectedError: "blocklists_subdomains_rule value is invalid",
			expectProfile: false,
		},

		// Advanced settings tests
		{
			name:      "Successfully update advanced recursor to unbound",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/advanced/recursor",
					Value:     model.RECURSOR_UNBOUND,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Advanced: &model.Advanced{Recursor: model.RECURSOR_SDNS},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update advanced recursor to sdns",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/advanced/recursor",
					Value:     model.RECURSOR_SDNS,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Advanced: &model.Advanced{Recursor: model.RECURSOR_UNBOUND},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update advanced recursor with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/advanced/recursor",
					Value:     "invalid_recursor",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Advanced: &model.Advanced{Recursor: model.RECURSOR_SDNS},
				},
			},
			expectedError: "recursor value is invalid",
			expectProfile: false,
		},
		{
			name:      "Update advanced recursor with non-string value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/advanced/recursor",
					Value:     123,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Advanced: &model.Advanced{Recursor: model.RECURSOR_SDNS},
				},
			},
			expectedError: "recursor value is invalid",
			expectProfile: false,
		},

		// DNSSEC settings tests
		{
			name:      "Successfully update DNSSEC enabled",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/security/dnssec/enabled",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Security: &model.Security{
						DNSSECSettings: model.DNSSECSettings{Enabled: false},
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update DNSSEC enabled with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/security/dnssec/enabled",
					Value:     "not_a_bool",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Security: &model.Security{
						DNSSECSettings: model.DNSSECSettings{Enabled: false},
					},
				},
			},
			expectedError: "parsing",
			expectProfile: false,
		},
		{
			name:      "Successfully update DNSSEC send_do_bit",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/security/dnssec/send_do_bit",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Security: &model.Security{
						DNSSECSettings: model.DNSSECSettings{SendDoBit: false},
					},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Update DNSSEC send_do_bit with invalid value",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/security/dnssec/send_do_bit",
					Value:     "not_a_bool",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Security: &model.Security{
						DNSSECSettings: model.DNSSECSettings{SendDoBit: false},
					},
				},
			},
			expectedError: "parsing",
			expectProfile: false,
		},

		// Value casting edge cases
		{
			name:      "Handle map value casting correctly",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/enabled",
					Value:     map[string]interface{}{"value": true},
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Enabled: false},
				},
			},
			expectedError: "",
			expectProfile: true,
		},

		// Multiple updates test
		{
			name:      "Successfully apply multiple updates",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/enabled",
					Value:     true,
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/statistics/enabled",
					Value:     true,
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/privacy/default_rule",
					Value:     model.DEFAULT_RULE_BLOCK,
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/advanced/recursor",
					Value:     model.RECURSOR_UNBOUND,
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/security/dnssec/enabled",
					Value:     true,
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings: &model.ProfileSettings{
					Logs:       &model.LogsSettings{Enabled: false},
					Statistics: &model.StatisticsSettings{Enabled: false},
					Privacy: &model.Privacy{
						DefaultRule:    model.DEFAULT_RULE_ALLOW,
						BlocklistsSubdomainsRule: model.ACTION_ALLOW,
					},
					Advanced: &model.Advanced{Recursor: model.RECURSOR_SDNS},
					Security: &model.Security{DNSSECSettings: model.DNSSECSettings{Enabled: false}},
				},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			expectedError:        "",
			expectProfile:        true,
		},

		// Error handling tests
		{
			name:      "Profile not found",
			profileID: "nonexistent",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			repoGetError:  errors.New("not found"),
			expectedError: "not found",
			expectProfile: false,
		},
		{
			name:      "Profile belongs to different account",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account456", // Different account
				Name:      "Original Profile",
			},
			expectedError: "not found",
			expectProfile: false,
		},
		{
			name:      "Repository update fails",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			repoUpdateError:      errors.New("update failed"),
			expectedError:        "update failed",
			expectProfile:        false,
		},
		{
			name:      "Cache update fails but profile is still returned",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			cacheError:           errors.New("cache error"),
			expectedError:        "cache error",
			expectProfile:        false, // Cache error prevents profile from being returned
		},

		// Edge case: unknown update paths (should be ignored)
		{
			name:      "Unknown update path is ignored",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/unknown/path",
					Value:     "some value",
				},
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/name",
					Value:     "Updated Profile",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Original Profile",
				Settings:  &model.ProfileSettings{},
			},
			shouldMockDuplicates: true,
			existingProfiles:     []model.Profile{},
			expectedError:        "",
			expectProfile:        true,
		},

		// Test all valid retention values
		{
			name:      "Successfully update retention to 1h",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "1h",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update retention to 6h",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "6h",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update retention to 1d",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "1d",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1h"},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
		{
			name:      "Successfully update retention to 1m",
			profileID: "profile123",
			accountID: "account123",
			updates: []model.ProfileUpdate{
				{
					Operation: model.UpdateOperationReplace,
					Path:      "/settings/logs/retention",
					Value:     "1m",
				},
			},
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{Retention: "1d"},
				},
			},
			expectedError: "",
			expectProfile: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			if tt.repoGetError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoGetError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					// For name updates, we need to mock GetProfilesByAccountId to check for duplicates
					if tt.shouldMockDuplicates {
						if len(tt.existingProfiles) > 0 && tt.updates[0].Path == "/name" {
							// Return existing profiles to simulate duplicate name check
							suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(tt.existingProfiles, nil)
						} else {
							// Return empty list to simulate no duplicate names
							suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return([]model.Profile{}, nil)
						}
					}

					// Mock the update and cache operations
					if tt.repoUpdateError != nil {
						suite.mockProfileRepo.On("Update", context.Background(), tt.profileID, mock.AnythingOfType("*model.Profile")).Return(tt.repoUpdateError)
					} else {
						suite.mockProfileRepo.On("Update", context.Background(), tt.profileID, mock.AnythingOfType("*model.Profile")).Return(nil)
						if tt.cacheError != nil {
							suite.mockCache.On("CreateOrUpdateProfileSettings", context.Background(), mock.AnythingOfType("*model.ProfileSettings"), false).Return(tt.cacheError)
						} else {
							suite.mockCache.On("CreateOrUpdateProfileSettings", context.Background(), mock.AnythingOfType("*model.ProfileSettings"), false).Return(nil)
						}
					}
				}
			}

			// Call the method under test
			profile, err := suite.service.UpdateProfile(context.Background(), tt.accountID, tt.profileID, tt.updates)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				if tt.expectProfile {
					suite.NotNil(profile)
				} else {
					suite.Nil(profile)
				}
			} else {
				suite.NoError(err)
				if tt.expectProfile {
					suite.NotNil(profile)
					suite.Equal(tt.accountID, profile.AccountId)
				} else {
					suite.Nil(profile)
				}
			}
		})
	}
}

// TestDeleteProfile tests the DeleteProfile method
func (suite *ProfileTestSuite) TestDeleteProfile() {
	tests := []struct {
		name             string
		profileID        string
		accountID        string
		removeLast       bool
		existingProfile  *model.Profile
		existingProfiles []model.Profile
		repoGetError     error
		repoDeleteError  error
		cacheError       error
		expectedError    string
	}{
		{
			name:       "Successfully delete profile",
			profileID:  "profile123",
			accountID:  "account123",
			removeLast: false,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			existingProfiles: []model.Profile{
				{ProfileId: "profile123", AccountId: "account123", Name: "Test Profile"},
				{ProfileId: "profile456", AccountId: "account123", Name: "Another Profile"},
			},
			expectedError: "",
		},
		{
			name:          "Profile not found",
			profileID:     "nonexistent",
			accountID:     "account123",
			removeLast:    false,
			repoGetError:  errors.New("not found"),
			expectedError: "not found",
		},
		{
			name:       "Last profile in account",
			profileID:  "profile123",
			accountID:  "account123",
			removeLast: false,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			existingProfiles: []model.Profile{
				{ProfileId: "profile123", AccountId: "account123", Name: "Test Profile"},
			},
			expectedError: "cannot delete the last profile in the account",
		},
		{
			name:       "Force delete last profile",
			profileID:  "profile123",
			accountID:  "account123",
			removeLast: true,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil
			suite.mockQueryLogsRepo.ExpectedCalls = nil

			if tt.repoGetError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoGetError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					// Mock the GetProfilesByAccountId call for checking if it's the last profile
					if !tt.removeLast {
						suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(tt.existingProfiles, nil)
					}

					// If we have more than 1 profile or removeLast is true, expect delete operations
					if len(tt.existingProfiles) > 1 || tt.removeLast {
						// Mock all three concurrent delete operations
						if tt.repoDeleteError != nil {
							suite.mockProfileRepo.On("DeleteProfileById", mock.Anything, tt.profileID).Return(tt.repoDeleteError)
						} else {
							suite.mockProfileRepo.On("DeleteProfileById", mock.Anything, tt.profileID).Return(nil)
						}

						// Mock QueryLogs service deletion
						suite.mockQueryLogsRepo.On("DeleteQueryLogs", context.Background(), tt.profileID).Return(nil)

						// Mock cache deletion
						if tt.cacheError != nil {
							suite.mockCache.On("DeleteProfileSettings", context.Background(), tt.profileID).Return(tt.cacheError)
						} else {
							suite.mockCache.On("DeleteProfileSettings", context.Background(), tt.profileID).Return(nil)
						}
					}
				}
			}

			// Call the method under test
			err := suite.service.DeleteProfile(context.Background(), tt.accountID, tt.profileID, tt.removeLast)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}
		})
	}
}

// TestGetProfiles tests the GetProfiles method
func (suite *ProfileTestSuite) TestGetProfiles() {
	tests := []struct {
		name             string
		accountID        string
		existingProfiles []model.Profile
		repoError        error
		expectedError    string
		expectedLength   int
	}{
		{
			name:      "Successfully get profiles",
			accountID: "account123",
			existingProfiles: []model.Profile{
				{Name: "Profile 1", AccountId: "account123"},
				{Name: "Profile 2", AccountId: "account123"},
			},
			expectedError:  "",
			expectedLength: 2,
		},
		{
			name:           "Repository error",
			accountID:      "account123",
			repoError:      errors.New("repo error"),
			expectedError:  "repo error",
			expectedLength: 0,
		},
		{
			name:             "No profiles found",
			accountID:        "account123",
			existingProfiles: []model.Profile{},
			expectedError:    "",
			expectedLength:   0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil

			if tt.repoError != nil {
				suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(nil, tt.repoError)
			} else {
				suite.mockProfileRepo.On("GetProfilesByAccountId", context.Background(), tt.accountID).Return(tt.existingProfiles, nil)
			}

			// Call the method under test
			profiles, err := suite.service.GetProfiles(context.Background(), tt.accountID)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(profiles)
			} else {
				suite.NoError(err)
				suite.Len(profiles, tt.expectedLength)
			}
		})
	}
}

// TestGetProfileQueryLogs tests the GetProfileQueryLogs method
func (suite *ProfileTestSuite) TestGetProfileQueryLogs() {
	tests := []struct {
		name            string
		profileID       string
		accountID       string
		status          string
		timespan        string
		deviceId        string
		sortBy          string
		page            int
		limit           int
		existingProfile *model.Profile
		repoError       error
		queryLogsError  error
		rateLimitValue  string
		expectedError   string
		expectedLogs    []model.QueryLog
	}{
		{
			name:      "Successfully get profile query logs",
			profileID: "profile123",
			accountID: "account123",
			status:    "all",
			timespan:  "LAST_1_DAY",
			deviceId:  "",
			sortBy:    "created",
			page:      0,
			limit:     0,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{
						Retention: model.RetentionOneDay,
					},
				},
			},
			expectedError: "",
			expectedLogs: []model.QueryLog{
				{DNSRequest: model.DNSRequest{Domain: "example.com"}, Status: "blocked"},
				{DNSRequest: model.DNSRequest{Domain: "google.com"}, Status: "processed"},
			},
		},
		{
			name:          "Profile not found",
			profileID:     "nonexistent",
			accountID:     "account123",
			status:        "all",
			timespan:      "LAST_1_DAY",
			deviceId:      "",
			sortBy:        "created",
			repoError:     errors.New("not found"),
			expectedError: "not found",
		},
		{
			name:      "Profile belongs to different account",
			profileID: "profile123",
			accountID: "account123",
			status:    "all",
			timespan:  "LAST_1_DAY",
			deviceId:  "",
			sortBy:    "created",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account456", // Different account
				Name:      "Test Profile",
			},
			expectedError: "not found",
		},
		{
			name:      "Query logs service error",
			profileID: "profile123",
			accountID: "account123",
			status:    "all",
			timespan:  "LAST_1_DAY",
			deviceId:  "",
			sortBy:    "created",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{
						Retention: model.RetentionOneDay,
					},
				},
			},
			queryLogsError: errors.New("query logs error"),
			expectedError:  "query logs error",
		},
		{
			name:      "Successfully get profile query logs with device ID filter",
			profileID: "profile123",
			accountID: "account123",
			status:    "all",
			timespan:  "LAST_1_DAY",
			deviceId:  "laptop",
			sortBy:    "created",
			page:      0,
			limit:     0,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{
						Retention: model.RetentionOneDay,
					},
				},
			},
			expectedError: "",
			expectedLogs: []model.QueryLog{
				{DNSRequest: model.DNSRequest{Domain: "example.com"}, Status: "blocked", DeviceId: "laptop"},
			},
		},
		{
			name:      "Rate limited after exceeding threshold",
			profileID: "profile123",
			accountID: "account123",
			status:    "all",
			timespan:  "LAST_1_DAY",
			deviceId:  "",
			sortBy:    "created",
			page:      0,
			limit:     0,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings:  &model.ProfileSettings{Logs: &model.LogsSettings{Retention: model.RetentionOneDay}},
			},
			rateLimitValue: "61", // simulate cache value exceeding Max (60)
			expectedError:  profile.ErrQueryLogsRateLimited.Error(),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockQueryLogsRepo.ExpectedCalls = nil
			suite.mockCache.ExpectedCalls = nil

			if tt.repoError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					// Rate limit cache expectations
					incrVal := int64(1)
					getVal := "1"
					if tt.rateLimitValue != "" {
						incrVal = 121
						getVal = tt.rateLimitValue
					}
					suite.mockCache.On("Incr", context.Background(), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(incrVal, nil)
					suite.mockCache.On("Get", context.Background(), mock.AnythingOfType("string")).Return(getVal, nil)
					if tt.queryLogsError != nil {
						suite.mockQueryLogsRepo.On("GetQueryLogs", context.Background(), tt.profileID, tt.existingProfile.Settings.Logs.Retention, tt.status, mock.AnythingOfType("int"), tt.deviceId, "", tt.sortBy, tt.page, tt.limit).Return(nil, tt.queryLogsError)
					} else if tt.expectedError == "" { // Only set successful expectation when not rate limited
						suite.mockQueryLogsRepo.On("GetQueryLogs", context.Background(), tt.profileID, tt.existingProfile.Settings.Logs.Retention, tt.status, mock.AnythingOfType("int"), tt.deviceId, "", tt.sortBy, tt.page, tt.limit).Return(tt.expectedLogs, nil)
					}
				}
			}

			// Call the method under test
			logs, err := suite.service.GetProfileQueryLogs(context.Background(), tt.accountID, tt.profileID, tt.status, tt.timespan, tt.deviceId, "", tt.sortBy, tt.page, tt.limit)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(logs)
			} else {
				suite.NoError(err)
				suite.Equal(tt.expectedLogs, logs)
			}
		})
	}
}

// TestDownloadProfileQueryLogs tests the DownloadProfileQueryLogs method
func (suite *ProfileTestSuite) TestDownloadProfileQueryLogs() {
	tests := []struct {
		name            string
		profileID       string
		accountID       string
		page            int
		limit           int
		existingProfile *model.Profile
		repoError       error
		queryLogsError  error
		expectedError   string
		expectedLogs    []model.QueryLog
	}{
		{
			name:      "Successfully download profile query logs",
			profileID: "profile123",
			accountID: "account123",
			page:      0,
			limit:     0,
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{
						Retention: model.RetentionOneWeek,
					},
				},
			},
			expectedError: "",
			expectedLogs: []model.QueryLog{
				{DNSRequest: model.DNSRequest{Domain: "example.com"}, Status: "blocked"},
				{DNSRequest: model.DNSRequest{Domain: "google.com"}, Status: "processed"},
			},
		},
		{
			name:          "Profile not found",
			profileID:     "nonexistent",
			accountID:     "account123",
			repoError:     errors.New("not found"),
			expectedError: "not found",
		},
		{
			name:      "Query logs service error",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
				Settings: &model.ProfileSettings{
					Logs: &model.LogsSettings{
						Retention: model.RetentionOneWeek,
					},
				},
			},
			queryLogsError: errors.New("download error"),
			expectedError:  "download error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockQueryLogsRepo.ExpectedCalls = nil

			if tt.repoError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					if tt.queryLogsError != nil {
						suite.mockQueryLogsRepo.On("GetQueryLogs", context.Background(), tt.profileID, tt.existingProfile.Settings.Logs.Retention, "all", 0, "", "", "created", tt.page, tt.limit).Return(nil, tt.queryLogsError)
					} else {
						suite.mockQueryLogsRepo.On("GetQueryLogs", context.Background(), tt.profileID, tt.existingProfile.Settings.Logs.Retention, "all", 0, "", "", "created", tt.page, tt.limit).Return(tt.expectedLogs, nil)
					}
				}
			}

			// Call the method under test
			logs, err := suite.service.DownloadProfileQueryLogs(context.Background(), tt.accountID, tt.profileID, tt.page, tt.limit)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(logs)
			} else {
				suite.NoError(err)
				suite.Equal(tt.expectedLogs, logs)
			}
		})
	}
}

// TestGetStatistics tests the GetStatistics method
func (suite *ProfileTestSuite) TestGetStatistics() {
	tests := []struct {
		name            string
		profileID       string
		accountID       string
		timespan        string
		existingProfile *model.Profile
		repoError       error
		statsError      error
		expectedError   string
		expectedStats   []model.StatisticsAggregated
	}{
		{
			name:      "Successfully get statistics",
			profileID: "profile123",
			accountID: "account123",
			timespan:  "LAST_1_DAY",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			expectedError: "",
			expectedStats: []model.StatisticsAggregated{
				{Total: 600}, // 100 blocked + 500 processed = 600 total
			},
		},
		{
			name:          "Profile not found",
			profileID:     "nonexistent",
			accountID:     "account123",
			timespan:      "LAST_1_DAY",
			repoError:     errors.New("not found"),
			expectedError: "not found",
		},
		{
			name:      "Profile belongs to different account",
			profileID: "profile123",
			accountID: "account123",
			timespan:  "LAST_1_DAY",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account456", // Different account
				Name:      "Test Profile",
			},
			expectedError: "not found",
		},
		{
			name:      "Statistics service error",
			profileID: "profile123",
			accountID: "account123",
			timespan:  "LAST_1_DAY",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			statsError:    errors.New("stats error"),
			expectedError: "stats error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockStatisticsRepo.ExpectedCalls = nil

			if tt.repoError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					if tt.statsError != nil {
						suite.mockStatisticsRepo.On("GetProfileStatistics", context.Background(), tt.profileID, mock.AnythingOfType("int")).Return(nil, tt.statsError)
					} else {
						suite.mockStatisticsRepo.On("GetProfileStatistics", context.Background(), tt.profileID, mock.AnythingOfType("int")).Return(tt.expectedStats, nil)
					}
				}
			}

			// Call the method under test
			stats, err := suite.service.GetStatistics(context.Background(), tt.accountID, tt.profileID, tt.timespan)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
				suite.Nil(stats)
			} else {
				suite.NoError(err)
				suite.Equal(tt.expectedStats, stats)
			}
		})
	}
}

// TestDeleteProfileQueryLogs tests the DeleteProfileQueryLogs method
func (suite *ProfileTestSuite) TestDeleteProfileQueryLogs() {
	tests := []struct {
		name            string
		profileID       string
		accountID       string
		existingProfile *model.Profile
		repoError       error
		deleteError     error
		expectedError   string
	}{
		{
			name:      "Successfully delete profile query logs",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			expectedError: "",
		},
		{
			name:          "Profile not found",
			profileID:     "nonexistent",
			accountID:     "account123",
			repoError:     errors.New("not found"),
			expectedError: "not found",
		},
		{
			name:      "Profile belongs to different account",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account456", // Different account
				Name:      "Test Profile",
			},
			expectedError: "not found",
		},
		{
			name:      "Delete query logs error",
			profileID: "profile123",
			accountID: "account123",
			existingProfile: &model.Profile{
				ProfileId: "profile123",
				AccountId: "account123",
				Name:      "Test Profile",
			},
			deleteError:   errors.New("delete error"),
			expectedError: "delete error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock expectations for each test case
			suite.mockProfileRepo.ExpectedCalls = nil
			suite.mockQueryLogsRepo.ExpectedCalls = nil

			if tt.repoError != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(nil, tt.repoError)
			} else if tt.existingProfile != nil {
				suite.mockProfileRepo.On("GetProfileById", context.Background(), tt.profileID).Return(tt.existingProfile, nil)

				if tt.existingProfile.AccountId == tt.accountID {
					if tt.deleteError != nil {
						suite.mockQueryLogsRepo.On("DeleteQueryLogs", context.Background(), tt.profileID).Return(tt.deleteError)
					} else {
						suite.mockQueryLogsRepo.On("DeleteQueryLogs", context.Background(), tt.profileID).Return(nil)
					}
				}
			}

			// Call the method under test
			err := suite.service.DeleteProfileQueryLogs(context.Background(), tt.accountID, tt.profileID)

			// Assert results
			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}
		})
	}
}

// TestCreateCustomRulesBulkAutoPrepend verifies the auto-prepend "*." logic
// in CreateCustomRulesBulk when custom_rules_subdomains_rule is "include".
// It ensures non-FQDN inputs (IPs, ASNs, CIDRs, dot-prefixes, wildcards)
// are NOT incorrectly prefixed.
func (suite *ProfileTestSuite) TestCreateCustomRulesBulkAutoPrepend() {
	const accountID = "acc-autoprepend"
	const profileID = "prof-autoprepend"

	tests := []struct {
		name           string
		subdomainsRule string
		input          string
		wantValue      string // expected normalized value in Created (empty = expect Skipped)
		wantSkipped    bool
	}{
		{
			name:           "include mode: plain FQDN gets wildcard prepended",
			subdomainsRule: "include",
			input:          "facebook.com",
			wantValue:      "*.facebook.com",
		},
		{
			name:           "exact mode: plain FQDN stays as-is",
			subdomainsRule: "exact",
			input:          "facebook.com",
			wantValue:      "facebook.com",
		},
		{
			name:           "include mode: dot-prefix normalized to wildcard, no double prefix",
			subdomainsRule: "include",
			input:          ".facebook.com",
			wantValue:      "*.facebook.com",
		},
		{
			name:           "exact mode: dot-prefix still normalized to wildcard",
			subdomainsRule: "exact",
			input:          ".facebook.com",
			wantValue:      "*.facebook.com",
		},
		{
			name:           "include mode: existing wildcard not double-prefixed",
			subdomainsRule: "include",
			input:          "*.facebook.com",
			wantValue:      "*.facebook.com",
		},
		{
			name:           "include mode: IPv4 not prefixed",
			subdomainsRule: "include",
			input:          "192.168.1.1",
			wantValue:      "192.168.1.1",
		},
		{
			name:           "include mode: IPv6 not prefixed",
			subdomainsRule: "include",
			input:          "::1",
			wantValue:      "::1",
		},
		// ASN syntax is not supported on this branch yet
		// {
		// 	name:           "include mode: ASN with prefix not prefixed",
		// 	subdomainsRule: "include",
		// 	input:          "AS15169",
		// 	wantValue:      "15169",
		// },
		// {
		// 	name:           "include mode: ASN without prefix not prefixed",
		// 	subdomainsRule: "include",
		// 	input:          "15169",
		// 	wantValue:      "15169",
		// },
		{
			name:           "include mode: CIDR not prefixed (skipped as invalid syntax)",
			subdomainsRule: "include",
			input:          "1.2.3.0/24",
			wantSkipped:    true,
			wantValue:      "1.2.3.0/24",
		},
		{
			name:           "include mode: IPv6 CIDR not prefixed (skipped as invalid syntax)",
			subdomainsRule: "include",
			input:          "2001:db8::/32",
			wantSkipped:    true,
			wantValue:      "2001:db8::/32",
		},
		{
			name:           "include mode: subdomain FQDN gets wildcard prepended",
			subdomainsRule: "include",
			input:          "www.facebook.com",
			wantValue:      "*.www.facebook.com",
		},
		{
			name:           "include mode: trailing dot stripped then prepended",
			subdomainsRule: "include",
			input:          "facebook.com.",
			wantValue:      "*.facebook.com",
		},
	}

	apiVldtr, err := intvldtr.NewAPIValidator()
	suite.Require().NoError(err)

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockProfileRepo := mocks.NewProfileRepository(suite.T())
			mockCache := mocks.NewCachecache(suite.T())

			svc := profile.NewProfileService(
				suite.serverConfig,
				suite.serviceConfig,
				mockProfileRepo,
				suite.blocklistService,
				suite.queryLogsService,
				suite.statisticsService,
				mockCache,
				suite.mockIDGen,
				apiVldtr.Validator,
			)

			existingProfile := &model.Profile{
				AccountId: accountID,
				Settings: &model.ProfileSettings{
					Privacy: &model.Privacy{
						BlocklistsSubdomainsRule:  "block",
						CustomRulesSubdomainsRule: tt.subdomainsRule,
						DefaultRule:               "allow",
					},
					CustomRules: []*model.CustomRule{},
				},
			}

			mockProfileRepo.On("GetProfileById", mock.Anything, profileID).
				Return(existingProfile, nil)

			if !tt.wantSkipped {
				mockProfileRepo.On("CreateCustomRules", mock.Anything, profileID, mock.Anything).
					Return(nil)
				mockCache.On("AddCustomRule", mock.Anything, profileID, mock.Anything).
					Return(nil)
			}

			result, err := svc.CreateCustomRulesBulk(
				context.Background(), accountID, profileID, "block", []string{tt.input},
			)

			suite.NoError(err)
			suite.NotNil(result)

			if tt.wantSkipped {
				suite.Len(result.Created, 0, "expected no rules created")
				suite.Require().Len(result.Skipped, 1, "expected one skipped entry")
				suite.Equal(tt.wantValue, result.Skipped[0].Value,
					"skipped entry should show un-mangled value")
			} else {
				suite.Require().Len(result.Created, 1, "expected one rule created")
				suite.Equal(tt.wantValue, result.Created[0].Value)
				suite.Len(result.Skipped, 0)
			}

			mockProfileRepo.AssertExpectations(suite.T())
			mockCache.AssertExpectations(suite.T())
		})
	}
}

func TestProfileTestSuite(t *testing.T) {
	suite.Run(t, new(ProfileTestSuite))
}
