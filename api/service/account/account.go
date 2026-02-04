package account

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	validatorv10 "github.com/go-playground/validator/v10"
	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/api/responses"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/internal/idgen"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
)

const (
	delCodeExpTime   = 15 * time.Minute
	firstProfileName = "Profile 01"
)

type AccountService struct {
	ServiceCfg           config.ServiceConfig
	AccountRepository    repository.AccountRepository
	ProfileService       *profile.ProfileService
	StatisticsService    *statistics.StatisticsService
	SubscriptionService  *subscription.SubscriptionService
	CredentialRepository repository.WebAuthnCredentialRepository
	Cache                cache.Cache
	Mailer               email.Mailer
	IDGenerator          idgen.Generator
	Validate             *validatorv10.Validate
	Http                 client.Http
}

// NewAccountService creates a new profile service
func NewAccountService(serviceCfg config.ServiceConfig, db repository.AccountRepository, profileSrv *profile.ProfileService, statsSrv *statistics.StatisticsService, subSrv *subscription.SubscriptionService, credRepo repository.WebAuthnCredentialRepository, cache cache.Cache, mailer email.Mailer, idGen idgen.Generator, validate *validatorv10.Validate, http client.Http) *AccountService {
	return &AccountService{
		AccountRepository:    db,
		ServiceCfg:           serviceCfg,
		ProfileService:       profileSrv,
		StatisticsService:    statsSrv,
		SubscriptionService:  subSrv,
		CredentialRepository: credRepo,
		Cache:                cache,
		Mailer:               mailer,
		IDGenerator:          idGen,
		Validate:             validate,
		Http:                 http,
	}
}

func (a *AccountService) GetUnfinishedSignupOrPostAccount(ctx context.Context, email, password string, subscriptionID string) (*model.Account, error) {
	// 1. Ensure subscription cache presence (activeUntil retrieval)
	activeUntil, cacheErr := a.Cache.GetSubscription(ctx, subscriptionID)
	if cacheErr != nil {
		return nil, ErrUnableToCreateAccount
	}

	// 2. Lookup account by email
	existingAcc, accErr := a.AccountRepository.GetAccountByEmail(ctx, email)
	if accErr != nil && !errors.Is(accErr, dbErrors.ErrAccountNotFound) {
		return nil, accErr
	}

	// Helper: determine finished (derived) - password set indicates finished
	isFinished := func(acc *model.Account) bool {
		if acc == nil {
			return false
		}
		// Password present -> finished
		if acc.Password != nil {
			return true
		}
		// Check passkey credentials count; treat errors as not finished (log only)
		if a.CredentialRepository != nil {
			count, err := a.CredentialRepository.GetCredentialsCount(ctx, acc.ID)
			if err != nil {
				log.Debug().Err(err).Str("account_id", acc.ID.Hex()).Msg("Failed to get credential count; assuming unfinished")
				return false
			}
			return count > 0
		}
		return false
	}

	if existingAcc != nil { // Reuse path
		if isFinished(existingAcc) { // Scenario 4B: finished account reuse attempt
			return nil, ErrUnableToCreateAccount
		}
		// Unfinished account path (Scenario 4A)
		// Ensure subscription exists; if absent create it
		_, subErr := a.SubscriptionService.GetSubscription(ctx, existingAcc.ID.Hex())
		if subErr != nil {
			if errors.Is(subErr, dbErrors.ErrSubscriptionNotFound) {
				// create subscription now using previously validated cache activeUntil
				createErr := a.SubscriptionService.CreateSubscription(ctx, existingAcc.ID.Hex(), subscriptionID, activeUntil)
				if createErr != nil {
					return nil, ErrUnableToCreateAccount
				}
			} else {
				return nil, subErr
			}
		}
		// If password provided and not yet set, set it and mark finished (derived)
		if existingAcc.Password == nil && strings.TrimSpace(password) != "" {
			if err := existingAcc.SetPassword(password); err != nil {
				return nil, err
			}
			if _, updErr := a.AccountRepository.UpdateAccount(ctx, existingAcc); updErr != nil {
				return nil, updErr
			}
		}

		log.Debug().Msg("Reusing unfinished account for registration - completing signup")
		if password != "" {
			if err := a.CompleteRegistration(ctx, existingAcc, subscriptionID); err != nil {
				log.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to complete registration")
				return nil, err
			}
		}
		return existingAcc, nil
	}

	// 3. Account does not exist
	// Check if subscription UUID already exists (Scenario 2)
	// Implement via repository method; if found -> unified failure
	if subById, _ := a.SubscriptionService.GetSubscriptionById(ctx, subscriptionID); subById != nil {
		return nil, ErrUnableToCreateAccount
	}

	// 4. Proceed with new account creation (Scenario 3)
	acc, regErr := a.RegisterAccountWithActiveUntil(ctx, email, password, subscriptionID, activeUntil)
	if regErr != nil {
		return nil, regErr
	}

	log.Debug().Msg("Created new account for registration - before webhook")
	if password != "" {
		if err := a.CompleteRegistration(ctx, acc, subscriptionID); err != nil {
			log.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to complete registration")
			return nil, err
		}
	}
	return acc, nil
}

// CompleteRegistration finalizes registration steps for an account
// Sends signup webhook, removes subscription cache entry, sends welcome email
func (a *AccountService) CompleteRegistration(ctx context.Context, account *model.Account, subscriptionID string) error {
	err := a.Http.SignupWebhook(subscriptionID)
	if err != nil {
		log.Debug().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to send signup webhook")
	}

	if err == nil {
		// Remove subscription cache key (idempotent; ignore removal error as non-critical)
		if err := a.Cache.RemoveSubscription(ctx, subscriptionID); err != nil {
			log.Debug().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to remove subscription cache entry")
		}
		err = a.sendWelcomeEmail(ctx, account, account.Email)
		if err != nil {
			return err
		}
	}
	return err
}

// RegisterAccount registers (creates) a new account
func (a *AccountService) RegisterAccount(ctx context.Context, email, passwordPlain, subscriptionID string) (*model.Account, error) {
	activeUntil, err := a.Cache.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// check if given email is already registered
	existingAcc, err := a.AccountRepository.GetAccountByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, dbErrors.ErrAccountNotFound) {
			return nil, err
		}
	}
	if existingAcc != nil {
		return nil, ErrAccountAlreadyExists
	}

	accountId := primitive.NewObjectID()
	profile, err := a.ProfileService.CreateProfile(ctx, firstProfileName, accountId.Hex())
	if err != nil {
		return nil, err
	}

	acc, err := a.AccountRepository.CreateAccount(ctx, email, passwordPlain, accountId.Hex(), profile.ProfileId)
	if err != nil {
		return nil, err
	}

	err = a.SubscriptionService.CreateSubscription(ctx, acc.ID.Hex(), subscriptionID, activeUntil)
	if err != nil {
		return nil, err
	}

	if err = a.Cache.RemoveSubscription(ctx, subscriptionID); err != nil {
		return nil, err
	}

	// eg, _ := errgroup.WithContext(ctx)
	// eg.Go(func() (err error) {
	// 	return a.Mailer.Verify(email)
	// })

	err = a.sendWelcomeEmail(ctx, acc, email)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (a *AccountService) sendWelcomeEmail(ctx context.Context, acc *model.Account, email string) error {
	eg, _ := errgroup.WithContext(ctx)
	eg.Go(func() (err error) { return a.Mailer.Verify(email) })
	eg.Go(func() error {
		return a.sendEmailCategory(acc, EmailCategoryWelcome, func() error {
			err := a.Mailer.SendWelcomeEmail(ctx, email, "")
			if err != nil {
				log.Err(err).Msg("Failed to send welcome email")
			}
			return err
		})
	})
	if err := eg.Wait(); err != nil {
		log.Err(err).Msg(ErrFailedToCreateAccount.Error())
		return err
	}
	return nil
}

// RegisterAccountWithActiveUntil is internal helper when activeUntil already retrieved & validated
func (a *AccountService) RegisterAccountWithActiveUntil(ctx context.Context, email, passwordPlain, subscriptionID, activeUntil string) (*model.Account, error) {
	// check if given email is already registered (defensive re-check)
	existingAcc, err := a.AccountRepository.GetAccountByEmail(ctx, email)
	if err != nil && !errors.Is(err, dbErrors.ErrAccountNotFound) {
		return nil, err
	}
	if existingAcc != nil {
		return nil, ErrAccountAlreadyExists
	}

	accountId := primitive.NewObjectID()
	profile, err := a.ProfileService.CreateProfile(ctx, firstProfileName, accountId.Hex())
	if err != nil {
		return nil, err
	}

	// create subscription
	if err = a.SubscriptionService.CreateSubscription(ctx, accountId.Hex(), subscriptionID, activeUntil); err != nil {
		return nil, err
	}

	acc, err := a.AccountRepository.CreateAccount(ctx, email, passwordPlain, accountId.Hex(), profile.ProfileId)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

// SendResetPasswordEmail generates secure token and sends reset password email.
// Returns nil even when the account is not found to prevent account enumeration.
func (a *AccountService) SendResetPasswordEmail(ctx context.Context, email string) error {
	acc, err := a.AccountRepository.GetAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, dbErrors.ErrAccountNotFound) {
			// Do not reveal whether the account exists.
			log.Debug().Msg("Password reset requested for non-existent account")
			return nil
		}
		return err
	}

	if !acc.EmailVerified {
		log.Warn().Msg("Password reset requested for unverified email")
	}

	eg, _ := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		return a.Mailer.Verify(email)
	})

	token, err := auth.NewToken(auth.TokenTypePasswordReset)
	if err != nil {
		return err
	}
	acc.Tokens = append(acc.Tokens, *token)
	_, err = a.AccountRepository.UpdateAccount(ctx, acc)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		return a.sendEmailCategory(acc, EmailCategoryPasswordReset, func() error {
			err := a.Mailer.SendPasswordResetEmail(ctx, email, token.Value)
			if err != nil {
				log.Err(err).Msg("Failed to send password reset email")
			}
			return err
		})
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// GetAccount returns account data by ID
func (p *AccountService) GetAccount(ctx context.Context, accountId string) (*model.Account, error) {
	account, err := p.AccountRepository.GetAccountById(ctx, accountId)
	if err != nil {
		return nil, err
	}

	stats, err := p.GetAccountMetrics(ctx, account, model.LAST_MONTH)
	if err != nil {
		return nil, err
	}
	account.Queries = stats.Total
	if err := p.populateAuthMethods(ctx, account); err != nil {
		return nil, err
	}

	return account, nil
}

// populateAuthMethods derives available authentication methods without persisting them.
func (a *AccountService) populateAuthMethods(ctx context.Context, acc *model.Account) error {
	methods := make([]string, 0, 2)
	if acc.Password != nil {
		methods = append(methods, model.AuthMethodPassword)
	}
	if a.CredentialRepository != nil {
		count, err := a.CredentialRepository.GetCredentialsCount(ctx, acc.ID)
		if err != nil {
			return err
		}
		if count > 0 {
			methods = append(methods, model.AuthMethodPasskey)
		}
	}
	acc.AuthMethods = methods
	return nil
}

// GetAccountStatistics returns profile DNS statistics data
func (a *AccountService) GetAccountMetrics(ctx context.Context, account *model.Account, timespan string) (*model.StatisticsAggregated, error) {
	accMetricsAggregated := &model.StatisticsAggregated{}
	for _, profileId := range account.Profiles {
		profileStats, err := a.ProfileService.GetStatistics(ctx, account.ID.Hex(), profileId, timespan)
		if err != nil {
			return nil, err
		}

		accMetricsAggregated.Total += profileStats[0].Total
	}

	return accMetricsAggregated, nil
}

// UpdateAccount updates account data
func (a *AccountService) UpdateAccount(ctx context.Context, accountId string, updates []model.AccountUpdate, mfa *model.MfaData) error {
	var profileUpdates []model.AccountUpdate
	var otherUpdates []model.AccountUpdate

	// Separate profile updates from other updates
	for _, update := range updates {
		if update.Path == "/profiles" {
			profileUpdates = append(profileUpdates, update)
		} else {
			otherUpdates = append(otherUpdates, update)
		}
	}

	// Handle profile updates atomically using MongoDB operators
	for _, update := range profileUpdates {
		if err := a.handleProfilesUpdateAtomic(ctx, accountId, update); err != nil {
			return err
		}
	}

	// Handle other updates if any exist
	if len(otherUpdates) > 0 {
		acc, err := a.AccountRepository.GetAccountById(ctx, accountId)
		if err != nil {
			return err
		}

		// Collect password updates for deferred sequence enforcement
		var passwordUpdates []model.AccountUpdate

		for _, update := range otherUpdates {
			// Collect password updates for deferred handling; process other paths immediately
			if update.Path == "/password" {
				// defer password logic entirely to handlePasswordUpdate
				passwordUpdates = append(passwordUpdates, update)
				continue
			}
			switch update.Path {
			case "/email":
				// Email change requires current password (provided in update.Value as {current_password,new_email})
				if err = a.handleEmailUpdate(ctx, acc, update, mfa); err != nil {
					return err
				}
			case "/error_reports_consent":
				acc.ErrorReportsConsent = cast.ToBool(update.Value)
			}
		}
		// After iterating, handle collected password updates (enforce test->replace sequence inside handler)
		if len(passwordUpdates) > 0 {
			if err := a.MfaCheck(ctx, acc, mfa); err != nil {
				return err
			}
			if err := a.handlePasswordUpdate(acc, passwordUpdates); err != nil {
				return err
			}
		}
		_, err = a.AccountRepository.UpdateAccount(ctx, acc)
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (a *AccountService) handleProfilesUpdateAtomic(ctx context.Context, accountId string, update model.AccountUpdate) error {
	value, err := cast.ToStringE(update.Value)
	if err != nil {
		return err
	}

	switch update.Operation {
	case model.UpdateOperationAdd:
		return a.AccountRepository.AddProfileToAccount(ctx, accountId, value)
	case model.UpdateOperationRemove:
		return a.AccountRepository.RemoveProfileFromAccount(ctx, accountId, value)
	default:
		return ErrInvalidUpdateOperation
	}
}

func (a *AccountService) handleEmailUpdate(ctx context.Context, acc *model.Account, update model.AccountUpdate, mfa *model.MfaData) error {
	// Validate email update payload
	// Marshal then unmarshal to structured type (supports map or raw interface value)
	b, err := json.Marshal(update.Value)
	if err != nil {
		return err
	}
	var emailUpd requests.AccountEmailUpdate
	if err := json.Unmarshal(b, &emailUpd); err != nil {
		return ErrInvalidEmailUpdatePayload
	}
	// Run validator tag checks if validator is available (API layer normally guarantees this)
	if a.Validate != nil {
		if err = a.Validate.Struct(emailUpd); err != nil {
			return err
		}
	}
	// Exactly one auth method check
	if emailUpd.CurrentPassword == nil && emailUpd.ReauthToken == nil {
		return ErrMissingAuthMethod
	}

	if emailUpd.CurrentPassword != nil && emailUpd.ReauthToken != nil {
		return ErrMultipleAuthMethods
	}

	if emailUpd.CurrentPassword != nil {
		if err := a.MfaCheck(ctx, acc, mfa); err != nil {
			return err
		}
		currPass, err := cast.ToStringE(emailUpd.CurrentPassword)
		if err != nil {
			return err
		}
		// Verify current password
		if acc.Password == nil || !auth.CheckPasswordHash(currPass, *acc.Password) {
			return ErrInvalidCurrentPassword
		}
	}
	if emailUpd.ReauthToken != nil {
		reauthToken, err := cast.ToStringE(emailUpd.ReauthToken)
		if err != nil {
			return err
		}
		// Validate reauth token in acc.Tokens
		var remaining []model.Token
		var matched bool
		for _, t := range acc.Tokens {
			if t.Type == auth.TokenTypeReauthEmailChange && t.Value == reauthToken {
				if time.Now().After(t.ExpiresAt) {
					return ErrReauthTokenExpired
				}
				matched = true
				// do not append (consume)
				continue
			}
			remaining = append(remaining, t)
		}
		if !matched {
			return ErrInvalidReauthToken
		}
		acc.Tokens = remaining // consume token
	}
	// Email format & presence validated at API layer via AccountEmailUpdate struct (requests.AccountEmailUpdate)
	// Retain only defensive empty check in case future callers bypass handler.
	if emailUpd.NewEmail == "" {
		return ErrInvalidNewEmail
	}
	// Check if new email is same as current email
	if strings.EqualFold(emailUpd.NewEmail, acc.Email) {
		return ErrSameEmailAddress
	}
	// Reset verification state and tokens
	acc.Email = emailUpd.NewEmail
	acc.EmailVerified = false
	filtered := make([]model.Token, 0, len(acc.Tokens))
	for _, t := range acc.Tokens { // remove any previous email verification tokens
		if t.Type != EmailCategoryVerificationOTP {
			filtered = append(filtered, t)
		}
	}
	acc.Tokens = filtered
	return nil
}

func (a *AccountService) handlePasswordUpdate(acc *model.Account, updates []model.AccountUpdate) error {
	verified := acc.Password == nil
	for _, upd := range updates {
		switch upd.Operation {
		case model.UpdateOperationTest:
			currPass, err := cast.ToStringE(upd.Value)
			if err != nil {
				return err
			}
			if acc.Password == nil || !auth.CheckPasswordHash(currPass, *acc.Password) {
				return ErrInvalidCurrentPassword
			}
			verified = true
		case model.UpdateOperationReplace:
			if !verified {
				return ErrPasswordTestRequired
			}
			pass, err := cast.ToStringE(upd.Value)
			if err != nil {
				return err
			}
			if ok := validator.ValidatePassword(pass); !ok {
				return ErrPasswordTooSimple
			}
			if err := acc.SetPassword(pass); err != nil {
				return err
			}
			// consume verification so another replace needs new test
			verified = false
		default:
			return ErrInvalidUpdateOperation
		}
	}
	return nil
}

// DeleteAccount deletes account with all connected data
func (a *AccountService) DeleteAccount(ctx context.Context, accountId string, req requests.AccountDeletionRequest, mfa *model.MfaData) error {
	if mfa == nil {
		mfa = &model.MfaData{}
	}
	// Get account to verify deletion code
	account, err := a.AccountRepository.GetAccount(ctx, accountId)
	if err != nil {
		return err
	}

	// Verify deletion code
	if account.DeletionCode != req.DeletionCode {
		return ErrInvalidDeletionCode
	}

	// Check if code is expired
	if account.DeletionCodeExpires == nil || time.Now().After(*account.DeletionCodeExpires) {
		return ErrDeletionCodeExpired
	}

	if req.CurrentPassword == nil && req.ReauthToken == nil {
		return ErrMissingAuthMethod
	}
	if req.CurrentPassword != nil && req.ReauthToken != nil {
		return ErrMultipleAuthMethods
	}

	if req.CurrentPassword != nil {
		if err := a.MfaCheck(ctx, account, mfa); err != nil {
			return err
		}
		if account.Password == nil || !auth.CheckPasswordHash(*req.CurrentPassword, *account.Password) {
			return ErrInvalidCurrentPassword
		}
	}

	if req.ReauthToken != nil {
		tokenValue := *req.ReauthToken
		var (
			remaining []model.Token
			matched   bool
		)
		for _, t := range account.Tokens {
			if t.Type == auth.TokenTypeReauthAccountDeletion && t.Value == tokenValue {
				if time.Now().After(t.ExpiresAt) {
					return ErrReauthTokenExpired
				}
				matched = true
				continue
			}
			remaining = append(remaining, t)
		}
		if !matched {
			return ErrInvalidReauthToken
		}
		account.Tokens = remaining
	}

	// Delete all profiles associated with the account
	for _, profileId := range account.Profiles {
		if err = a.ProfileService.DeleteProfile(ctx, accountId, profileId, true); err != nil {
			return err
		}
	}

	// Delete the account
	err = a.AccountRepository.DeleteAccountById(ctx, accountId)
	if err != nil {
		return err
	}

	return nil
}

func (a *AccountService) MfaCheck(ctx context.Context, acc *model.Account, mfa *model.MfaData) error {
	if acc.MFA.TOTP.Enabled && mfa.OTP == "" {
		return ErrTOTPRequired
	}

	if acc.MFA.TOTP.Enabled && mfa.OTP != "" {
		_, err := a.VerifyTotp(ctx, acc.ID.Hex(), mfa.OTP, "login")
		if err != nil {
			return err
		}
	}
	return nil
}

// GenerateDeletionCode generates a deletion code for account deletion
func (a *AccountService) GenerateDeletionCode(ctx context.Context, accountId string) (*responses.DeletionCodeResponse, error) {
	// Generate deletion code using the deletion code generator
	deletionCodeGen, err := idgen.NewGenerator(idgen.TypeDeletionCode, 0)
	if err != nil {
		return nil, err
	}

	code, err := deletionCodeGen.Generate()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(delCodeExpTime)

	// Update account with deletion code
	err = a.AccountRepository.UpdateDeletionCode(ctx, accountId, code, expiresAt)
	if err != nil {
		return nil, err
	}

	return &responses.DeletionCodeResponse{
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}
