package account_test

import (
	"context"
	"testing"
	"time"

	validatorv10 "github.com/go-playground/validator/v10"
	"github.com/ivpn/dns/api/config"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	account "github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReauthTokenSuite covers issuance and consumption of reauth tokens for email change
type ReauthTokenSuite struct {
	suite.Suite
	service         *account.AccountService
	mockAccountRepo *mocks.AccountRepository
	mockMailer      *mocks.Maileremail
	mockCache       *mocks.Cachecache
}

func (suite *ReauthTokenSuite) SetupSuite() {
	// minimal env vars
	t := suite.T()
	t.Setenv("SERVER_ALLOWED_DOMAINS", "example.com")
	t.Setenv("SERVER_DNS_SERVER_ADDRESSES", "8.8.8.8:53")
	t.Setenv("API_PORT", "8080")
	t.Setenv("DB_NAME", "test_db")
	t.Setenv("CACHE_ADDRESS", "localhost:6379")
	cfg, err := config.New()
	suite.Require().NoError(err)

	suite.mockAccountRepo = mocks.NewAccountRepository(suite.T())
	suite.mockMailer = mocks.NewMaileremail(suite.T())
	suite.mockCache = mocks.NewCachecache(suite.T())
	val := validatorv10.New()
	// Provide a minimal subscription service dependency required by constructor
	mockSubRepo := mocks.NewSubscriptionRepository(suite.T())
	mockProfileRepo := mocks.NewProfileRepository(suite.T())
	subService := subscription.NewSubscriptionService(mockSubRepo, mockProfileRepo, suite.mockCache, config.ServiceConfig{}, config.APIConfig{}, webhookClient.Http{})
	// credential repo is not used in these tests; pass nil
	suite.service = account.NewAccountService(*cfg.Service, suite.mockAccountRepo, nil, nil, subService, nil, suite.mockCache, suite.mockMailer, nil, val, webhookClient.Http{})
}

func (suite *ReauthTokenSuite) newAccount(email string) *model.Account {
	pw := "$2a$10$hash"
	return &model.Account{ID: primitive.NewObjectID(), Email: email, Password: &pw, Tokens: []model.Token{}, EmailVerified: true}
}

// NOTE: The actual issuance of reauth tokens occurs in API layer via WebAuthn finish; here we test consumption logic inside handleEmailUpdate indirectly via UpdateAccount
// We will simulate an account having a valid reauth token and perform an email change.
func (suite *ReauthTokenSuite) TestEmailChangeWithReauthToken() {
	acc := suite.newAccount("user@example.com")
	acc.MFA = model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}
	// Add reauth token (unexpired)
	tok := model.Token{Type: "reauth_email_change", Value: "token123", ExpiresAt: time.Now().Add(2 * time.Minute)}
	acc.Tokens = []model.Token{tok}

	// Mock GetAccountById
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// Expect persistence on success path
	suite.mockAccountRepo.On("UpdateAccount", context.Background(), mock.AnythingOfType("*model.Account")).Return(acc, nil)

	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com", "reauth_token": tok.Value}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().NoError(err)

	suite.Contains(acc.Email, "new@example.com")
}

func (suite *ReauthTokenSuite) TestEmailChangeWithExpiredReauthToken() {
	acc := suite.newAccount("user@example.com")
	acc.MFA = model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}
	tok := model.Token{Type: "reauth_email_change", Value: "token123", ExpiresAt: time.Now().Add(-1 * time.Minute)}
	acc.Tokens = []model.Token{tok}
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// No UpdateAccount call expected on failure
	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com", "reauth_token": tok.Value}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, account.ErrReauthTokenExpired)
}

func (suite *ReauthTokenSuite) TestEmailChangeWithInvalidReauthToken() {
	acc := suite.newAccount("user@example.com")
	acc.MFA = model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}
	// Different token present
	tok := model.Token{Type: "reauth_email_change", Value: "other", ExpiresAt: time.Now().Add(2 * time.Minute)}
	acc.Tokens = []model.Token{tok}
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// No UpdateAccount call expected on failure
	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com", "reauth_token": "wrong"}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, account.ErrInvalidReauthToken)
}

func (suite *ReauthTokenSuite) TestEmailChangeWithBothPasswordAndReauth() {
	acc := suite.newAccount("user@example.com")
	acc.MFA = model.MFASettings{TOTP: model.TotpSettings{Enabled: true, Secret: "SECRET"}}
	tok := model.Token{Type: "reauth_email_change", Value: "tok", ExpiresAt: time.Now().Add(2 * time.Minute)}
	acc.Tokens = []model.Token{tok}
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// No UpdateAccount call expected on failure
	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com", "reauth_token": "tok", "current_password": "pw"}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().Error(err)
	var valErr validatorv10.ValidationErrors
	suite.ErrorAs(err, &valErr)
	suite.NotEmpty(valErr)
	if len(valErr) > 0 {
		suite.Equal("excluded_with", valErr[0].Tag())
	}
}

func (suite *ReauthTokenSuite) TestEmailChangeMissingAuthMethod() {
	acc := suite.newAccount("user@example.com")
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// No UpdateAccount call expected on failure
	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com"}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, account.ErrMissingAuthMethod)
}

// NOTE: Password path unchanged; covered by existing tests (if any). We could add one here quickly.
func (suite *ReauthTokenSuite) TestEmailChangePasswordPathInvalidPassword() {
	acc := suite.newAccount("user@example.com")
	// Provide a different bcrypt hash (hash for "different") so checking "pw" fails
	pw := "$2a$10$CjwKkmG4F.Yh2lYpQeK1OegX1PpH2qJfJzqSxVhJx7Xy8d9kQOZs2"
	acc.Password = &pw
	suite.mockAccountRepo.On("GetAccountById", context.Background(), acc.ID.Hex()).Return(acc, nil)
	// No UpdateAccount call expected on failure
	updates := []model.AccountUpdate{{Operation: "replace", Path: "/email", Value: map[string]any{"new_email": "new@example.com", "current_password": "pw"}}}
	err := suite.service.UpdateAccount(context.Background(), acc.ID.Hex(), updates, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, account.ErrInvalidCurrentPassword)
}

func TestReauthTokenSuite(t *testing.T) { suite.Run(t, new(ReauthTokenSuite)) }
