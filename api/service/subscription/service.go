package subscription

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrPASessionNotFound = errors.New("pre-auth session not found or expired")
	ErrPANotFound        = errors.New("pre-auth entry not found")
	ErrTokenHashMismatch = errors.New("token validation failed")
)

type SubscriptionService struct {
	ServiceCfg             config.ServiceConfig
	APICfg                 config.APIConfig
	SubscriptionRepository repository.SubscriptionRepository
	ProfileRepository      repository.ProfileRepository
	Cache                  cache.Cache
	Http                   client.Http
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(db repository.SubscriptionRepository, profileRepo repository.ProfileRepository, cache cache.Cache, srvCfg config.ServiceConfig, apiCfg config.APIConfig, http client.Http) *SubscriptionService {
	return &SubscriptionService{
		SubscriptionRepository: db,
		ProfileRepository:      profileRepo,
		Cache:                  cache,
		ServiceCfg:             srvCfg,
		APICfg:                 apiCfg,
		Http:                   http,
	}
}

// GetSubscription returns subscription data by account ID with computed status fields.
func (s *SubscriptionService) GetSubscription(ctx context.Context, accountId string) (*model.Subscription, error) {
	subscription, err := s.SubscriptionRepository.GetSubscriptionByAccountId(ctx, accountId)
	if err != nil {
		if errors.Is(err, dbErrors.ErrSubscriptionNotFound) {
			return nil, dbErrors.ErrSubscriptionNotFound
		}
		return nil, err
	}

	subscription.Status = subscription.GetStatus()
	// Outage UI flag: true when never synced (zero UpdatedAt) OR genuinely stale (>48h)
	subscription.Outage = subscription.UpdatedAt.IsZero() || subscription.IsOutage()

	return subscription, nil
}

// UpdateSubscription updates subscription data.
func (s *SubscriptionService) UpdateSubscription(ctx context.Context, accountId string, updates []model.SubscriptionUpdate) (*model.Subscription, error) {
	subscription, err := s.SubscriptionRepository.GetSubscriptionByAccountId(ctx, accountId)
	if err != nil {
		return nil, err
	}

	err = s.SubscriptionRepository.Upsert(ctx, *subscription)
	return subscription, err
}

// CreateSubscriptionFromPreauth creates a new subscription using preauth entry data.
func (s *SubscriptionService) CreateSubscriptionFromPreauth(ctx context.Context, accountId string, preauth *model.Preauth) error {
	accOID, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}

	sub := model.Subscription{
		ID:          uuid.New(),
		AccountID:   accOID,
		ActiveUntil: preauth.ActiveUntil,
		IsActive:    preauth.IsActive,
		Tier:        preauth.Tier,
		TokenHash:   preauth.TokenHash,
		UpdatedAt:   time.Now(),
		Limits: model.SubscriptionLimits{
			MaxQueriesPerMonth: 0,
		},
	}

	return s.SubscriptionRepository.Create(ctx, sub)
}

// AddPASession stores a PASession in cache.
func (s *SubscriptionService) AddPASession(ctx context.Context, session *model.PASession) error {
	return s.Cache.AddPASession(ctx, session, s.APICfg.PreauthTTL)
}

// RotatePASessionID atomically rotates a session ID: fetches old, creates new, deletes old.
func (s *SubscriptionService) RotatePASessionID(ctx context.Context, oldID string) (string, error) {
	paSession, err := s.Cache.GetPASession(ctx, oldID)
	if err != nil {
		log.Debug().Err(err).Str("old_id", oldID).Msg("Failed to get PA session for rotation")
		return "", ErrPASessionNotFound
	}

	newID := uuid.NewString()
	rotated := &model.PASession{
		ID:        newID,
		Token:     paSession.Token,
		PreauthID: paSession.PreauthID,
	}

	if err := s.Cache.AddPASession(ctx, rotated, 15*time.Minute); err != nil {
		return "", err
	}

	if err := s.Cache.RemovePASession(ctx, oldID); err != nil {
		log.Debug().Err(err).Str("old_id", oldID).Msg("Failed to delete old PA session after rotation")
	}

	return newID, nil
}

// ValidateAndGetPreauth validates the PASession token against the preauth service entry.
func (s *SubscriptionService) ValidateAndGetPreauth(ctx context.Context, sessionID string) (*model.Preauth, error) {
	paSession, err := s.Cache.GetPASession(ctx, sessionID)
	if err != nil {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("ValidateAndGetPreauth: PASession not found in cache")
		return nil, ErrPASessionNotFound
	}

	preauth, err := s.Http.GetPreauth(paSession.PreauthID)
	if err != nil {
		log.Warn().Err(err).Str("preauth_id", paSession.PreauthID).Msg("ValidateAndGetPreauth: preauth service call failed")
		return nil, ErrPANotFound
	}

	tokenHash := sha256.Sum256([]byte(paSession.Token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])

	if subtle.ConstantTimeCompare([]byte(tokenHashStr), []byte(preauth.TokenHash)) != 1 {
		log.Warn().
			Str("session_id", sessionID).
			Str("preauth_id", paSession.PreauthID).
			Str("computed_hash", tokenHashStr).
			Str("preauth_hash", preauth.TokenHash).
			Msg("ValidateAndGetPreauth: token hash mismatch")
		return nil, ErrTokenHashMismatch
	}

	return &preauth, nil
}

// UpdateSubscriptionFromPASession validates the PASession, updates subscription fields from preauth, and persists.
func (s *SubscriptionService) UpdateSubscriptionFromPASession(ctx context.Context, sub *model.Subscription, sessionID string) error {
	preauth, err := s.ValidateAndGetPreauth(ctx, sessionID)
	if err != nil {
		return err
	}

	sub.ActiveUntil = preauth.ActiveUntil
	sub.IsActive = preauth.IsActive
	sub.Tier = preauth.Tier
	sub.TokenHash = preauth.TokenHash
	sub.UpdatedAt = time.Now()

	if err := s.SubscriptionRepository.Upsert(ctx, *sub); err != nil {
		log.Error().Err(err).Msg("Failed to update subscription from PASession")
		return err
	}

	// Re-populate Redis profile settings for the account's profiles.
	// This handles recovery from pending-delete state where DNS was cut (profile settings deleted from Redis).
	s.repopulateProfileCache(ctx, sub.AccountID.Hex())

	subID := sub.ID.String()
	if err := s.Http.SignupWebhook(subID); err != nil {
		log.Error().Err(err).Str("sub_id", subID).Msg("Failed to send signup webhook after subscription update")
		return err
	}

	return nil
}

// repopulateProfileCache loads the account's profiles from MongoDB and writes their settings to Redis.
// Errors are logged but do not fail the caller -- DNS recovery is best-effort during resync.
func (s *SubscriptionService) repopulateProfileCache(ctx context.Context, accountID string) {
	profiles, err := s.ProfileRepository.GetProfilesByAccountId(ctx, accountID)
	if err != nil {
		log.Error().Err(err).Str("account_id", accountID).Msg("Failed to load profiles for cache repopulation")
		return
	}

	for _, profile := range profiles {
		if profile.Settings == nil {
			continue
		}
		if err := s.Cache.CreateOrUpdateProfileSettings(ctx, profile.Settings, false); err != nil {
			log.Error().Err(err).Str("profile_id", profile.ProfileId).Msg("Failed to repopulate profile settings in cache")
		}
	}
}
