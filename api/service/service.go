package service

import (
	"context"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/internal/idgen"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
	"github.com/ivpn/dns/api/service/apple"
	"github.com/ivpn/dns/api/service/blocklist"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/ivpn/dns/libs/urlshort"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	Cfg      config.Config
	Store    db.Db
	Cache    cache.Cache
	Webauthn *webauthn.WebAuthn
	HTTP     webhookClient.Http
	AccountServicer
	ProfileServicer
	AppleServicer
	BlocklistServicer
	SubscriptionServicer
	SessionServicer
	PasskeyServicer
}

func New(cfg config.Config, store db.Db, cache cache.Cache, idGen idgen.Generator, apiValidator *validator.APIValidator, mailer email.Mailer, shortener *urlshort.URLShortener, webauthn *webauthn.WebAuthn) Service {
	blocklistSrv := blocklist.NewBlocklistService(store, cache)
	queryLogsSrv := querylogs.NewQueryLogsService(store)
	statsSrv := statistics.NewStatisticsService(store)
	profSrv := profile.NewProfileService(*cfg.Server, *cfg.Service, store, blocklistSrv, queryLogsSrv, statsSrv, cache, idGen, apiValidator.Validator)
	subSrv := subscription.NewSubscriptionService(store, cache, *cfg.Service)
	httpClient := webhookClient.New(*cfg.API)
	accSrv := account.NewAccountService(*cfg.Service, store, profSrv, statsSrv, subSrv, store, cache, mailer, idGen, apiValidator.Validator, *httpClient)
	appleSrv := apple.NewAppleService(&cfg, shortener)
	return Service{
		Cfg:                  cfg,
		Store:                store,
		Cache:                cache,
		AccountServicer:      accSrv,
		ProfileServicer:      profSrv,
		AppleServicer:        appleSrv,
		BlocklistServicer:    blocklistSrv,
		SubscriptionServicer: subSrv,
		Webauthn:             webauthn,
		HTTP:                 *httpClient,
	}
}

type Servicer interface {
	SessionServicer
	AccountServicer
	ProfileServicer
	AppleServicer
	BlocklistServicer
	SubscriptionServicer
	PasskeyServicer
	CredentialServicer
}

type CredentialServicer interface {
	GetCredentials(context.Context, primitive.ObjectID) ([]model.Credential, error)
	SaveCredential(context.Context, webauthn.Credential, primitive.ObjectID) error
	UpdateCredential(context.Context, webauthn.Credential, primitive.ObjectID) error
	DeleteCredential(ctx context.Context, credentialID []byte, accountID primitive.ObjectID) error
	DeleteCredentialByID(context.Context, primitive.ObjectID, primitive.ObjectID) error
}

type PasskeyServicer interface {
	BeginRegistration(ctx context.Context, account *model.Account) (*protocol.CredentialCreation, string, error)
	FinishRegistration(ctx context.Context, token string, httpReq *http.Request) error
	BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, string, error)
	FinishLogin(ctx context.Context, token string, httpReq *http.Request, saveSession bool) (*model.Account, string, string, error)
	GetPasskeys(ctx context.Context, account *model.Account) ([]model.Credential, error)
	// DeletePasskey(ctx context.Context, account *model.Account, credentialID []byte) error
	BeginReauth(ctx context.Context, purpose, accountID string) (*protocol.CredentialAssertion, string, error)
	FinishReauth(ctx context.Context, token string, httpReq *http.Request) (*model.Token, error)
}

type SessionServicer interface {
	GetSession(context.Context, string) (model.Session, bool, error)
	SaveSession(context.Context, webauthn.SessionData, string, string, string) error
	DeleteSession(context.Context, string) error
	DeleteSessionsByAccountID(ctx context.Context, accID string) error
	DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error
	CountSessionsByAccountID(ctx context.Context, accID string) (int64, error)
}

type AccountServicer interface {
	GetAccount(ctx context.Context, accountId string) (*model.Account, error)
	GetAccountMetrics(ctx context.Context, account *model.Account, timespan string) (*model.StatisticsAggregated, error)
	UpdateAccount(ctx context.Context, accountId string, updates []model.AccountUpdate, mfa *model.MfaData) error
	DeleteAccount(ctx context.Context, accountId string, req requests.AccountDeletionRequest, mfa *model.MfaData) error
	GenerateDeletionCode(ctx context.Context, accountId string) (*responses.DeletionCodeResponse, error)
	MfaCheck(ctx context.Context, acc *model.Account, mfa *model.MfaData) error
	RegisterAccount(ctx context.Context, email, password, subID string) (*model.Account, error)
	CompleteRegistration(ctx context.Context, account *model.Account, subscriptionID string) error
	GetUnfinishedSignupOrPostAccount(ctx context.Context, email, password string, subscriptionID string) (*model.Account, error)
	SendResetPasswordEmail(ctx context.Context, email string) error
	VerifyPasswordReset(ctx context.Context, tokenValue, newPassword string, mfa *model.MfaData) error
	TotpEnable(ctx context.Context, accountId string) (*model.TOTPNew, error)
	TotpConfirm(ctx context.Context, accountId, otp string) (*model.TOTPBackup, error)
	TotpDisable(ctx context.Context, accountId, otp string) (*model.Account, error)
	VerifyTotp(ctx context.Context, accountId, otp, action string) (*model.Account, error)
	RequestEmailVerificationOTP(ctx context.Context, accountId string) error
	VerifyEmailOTP(ctx context.Context, accountId, otp string) error
}

// ProfileServicer defines the interface for managing DNS profiles
type ProfileServicer interface {
	GetProfile(ctx context.Context, accountId, profileId string) (*model.Profile, error)
	GetProfiles(ctx context.Context, accountId string) ([]model.Profile, error)
	CreateProfile(ctx context.Context, name, accountId string) (*model.Profile, error)
	UpdateProfile(ctx context.Context, accountId, profileId string, updates []model.ProfileUpdate) (*model.Profile, error)
	DeleteProfile(ctx context.Context, accountId, profileId string, removeLast bool) error

	// Query logs
	GetProfileQueryLogs(ctx context.Context, accountId, profileId, status, timespan, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error)
	DownloadProfileQueryLogs(ctx context.Context, accountId, profileId string, page, limit int) ([]model.QueryLog, error)
	DeleteProfileQueryLogs(ctx context.Context, accountId, profileId string) error

	// Statistics
	GetStatistics(ctx context.Context, accountId, profileId, timespan string) ([]model.StatisticsAggregated, error)

	// Custom Rules
	DeleteCustomRule(ctx context.Context, accountId, profileId, customRuleId string) error
	CreateCustomRule(ctx context.Context, accountId, profileId, action, value string) error
	CreateCustomRulesBulk(ctx context.Context, accountId, profileId, action string, values []string) (*profile.BulkCustomRuleResult, error)

	// Blocklists
	EnableBlocklists(ctx context.Context, accountId, profileId string, blocklistIds []string) error
	DisableBlocklists(ctx context.Context, accountId, profileId string, blocklistIds []string) error
}

// QueryLogsServicer defines the interface for managing query logs
// Note: QueryLogsServicer is not part of the Servicer interface as ProfileServicer covers its operations
type QueryLogsServicer interface {
	GetProfileQueryLogs(ctx context.Context, profileId string, retention model.Retention, status, timespan, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error)
	DownloadProfileQueryLogs(ctx context.Context, profileId string, retention model.Retention, page, limit int) ([]model.QueryLog, error)
	DeleteProfileQueryLogs(ctx context.Context, profileId string) error
}

type AppleServicer interface {
	GenerateMobileConfig(ctx context.Context, req requests.MobileConfigReq, accountId string, genLink bool) (data []byte, link string, err error)
}

type BlocklistServicer interface {
	GetBlocklist(ctx context.Context, filter map[string]any) ([]*model.Blocklist, error)
}

type SubscriptionServicer interface {
	GetSubscription(ctx context.Context, accountId string) (*model.Subscription, error)
	UpdateSubscription(ctx context.Context, accountId string, updates []model.SubscriptionUpdate) (*model.Subscription, error)
	CreateSubscription(ctx context.Context, accountId, subscriptionId, activeUntil string) error
	AddSubscription(ctx context.Context, subscriptionId string, activeUntil string) error
}

// DeleteAccount deletes account with all connected data including sessions
func (s *Service) DeleteAccount(ctx context.Context, accountId string, req requests.AccountDeletionRequest, mfa *model.MfaData) error {
	if err := s.AccountServicer.DeleteAccount(ctx, accountId, req, mfa); err != nil {
		return err
	}
	return s.DeleteSessionsByAccountID(ctx, accountId)
}

// GenerateDeletionCode generates a deletion code for account deletion
func (s *Service) GenerateDeletionCode(ctx context.Context, accountId string) (*responses.DeletionCodeResponse, error) {
	return s.AccountServicer.GenerateDeletionCode(ctx, accountId)
}
