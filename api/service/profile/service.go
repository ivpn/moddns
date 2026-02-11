package profile

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/idgen"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/blocklist"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
)

// Account-based rate limiting for query logs retrieval.
const queryLogsRateLimitMax = 60
const queryLogsRateLimitWindow = time.Minute

type ProfileService struct {
	ProfileRepository repository.ProfileRepository
	QueryLogsService  *querylogs.QueryLogsService
	StatisticsService *statistics.StatisticsService
	BlocklistService  *blocklist.BlocklistService
	Cache             cache.Cache
	IdGen             idgen.Generator
	Validate          *validator.Validate
	ServerConfig      config.ServerConfig
	ServiceConfig     config.ServiceConfig
}

// NewProfileService creates a new profile service
func NewProfileService(serverCfg config.ServerConfig, serviceCfg config.ServiceConfig, db repository.ProfileRepository, blocklistService *blocklist.BlocklistService, qlService *querylogs.QueryLogsService, statsService *statistics.StatisticsService, cache cache.Cache, idGen idgen.Generator, validator *validator.Validate) *ProfileService {
	return &ProfileService{
		ProfileRepository: db,
		BlocklistService:  blocklistService,
		QueryLogsService:  qlService,
		StatisticsService: statsService,
		Cache:             cache,
		IdGen:             idGen,
		Validate:          validator,
		ServerConfig:      serverCfg,
		ServiceConfig:     serviceCfg,
	}
}

// Create creates a new profile
func (p *ProfileService) CreateProfile(ctx context.Context, name, accountId string) (*model.Profile, error) {
	if name == "" {
		return nil, ErrProfileNameEmpty
	}
	profile, err := model.NewProfile(p.IdGen, name, accountId)
	if err != nil {
		return nil, err
	}

	// create default settings associated with the profile ID
	settings, err := p.createSettings(ctx, profile.ProfileId)
	if err != nil {
		return nil, err
	}
	profile.Settings = settings

	profiles, err := p.ProfileRepository.GetProfilesByAccountId(ctx, accountId)
	if err != nil {
		return nil, err
	}

	// Check if maximum number of profiles limit is reached
	if len(profiles) >= p.ServiceConfig.MaxProfiles {
		return nil, ErrMaxProfilesLimitReached
	}

	for _, profile := range profiles {
		log.Trace().Str("profile_name", profile.Name).Msg("Checking for duplicate profile names")
		if profile.Name == name {
			return nil, ErrProfileNameAlreadyExists
		}
	}

	// TODO: update mongodb account document with new profile ID
	if err := p.ProfileRepository.CreateProfile(ctx, profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// GetProfile returns all profiles belonging to the account
func (p *ProfileService) GetProfiles(ctx context.Context, accountId string) ([]model.Profile, error) {
	profiles, err := p.ProfileRepository.GetProfilesByAccountId(ctx, accountId)
	if err != nil {
		return nil, err
	}

	return profiles, nil
}

// GetProfile returns profile data by ID
func (p *ProfileService) GetProfile(ctx context.Context, accountId, profileId string) (*model.Profile, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

// DeleteProfile deletes profile data by ID
func (p *ProfileService) DeleteProfile(ctx context.Context, accountId, profileId string, removeLast bool) error {
	_, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	if !removeLast {
		profiles, err := p.ProfileRepository.GetProfilesByAccountId(ctx, accountId)
		if err != nil {
			return err
		}
		if len(profiles) <= 1 {
			return ErrLastProfileInAccount
		}
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		// delete all profile-related data from DB
		return p.ProfileRepository.DeleteProfileById(egCtx, profileId)
	})

	eg.Go(func() (err error) {
		// delete query logs
		return p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId)
	})

	eg.Go(func() (err error) {
		// delete all profile-related data from cache
		return p.Cache.DeleteProfileSettings(ctx, profileId)
	})

	if err := eg.Wait(); err != nil {
		log.Err(err).Msg(ErrFailedToDeleteProfile.Error())
		return err
	}

	return nil
}

// GetProfileQueryLogs returns profile DNS query logs
func (p *ProfileService) GetProfileQueryLogs(ctx context.Context, accountId, profileId, status, timespan, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	limiter := utils.IDLimiter{Cache: p.Cache, Label: "rate_limits", ID: accountId + ":query_logs", Max: queryLogsRateLimitMax, Exp: queryLogsRateLimitWindow}
	if tickErr := limiter.Tick(); tickErr != nil {
		log.Err(tickErr).Msg("failed to tick query logs rate limiter")
	} else if !limiter.IsAllowed() {
		return nil, ErrQueryLogsRateLimited
	}

	return p.QueryLogsService.GetProfileQueryLogs(ctx, profileId, profile.Settings.Logs.Retention, status, timespan, deviceId, search, sortBy, page, limit)
}

// DownloadProfileQueryLogs returns all existing profile DNS query logs
func (p *ProfileService) DownloadProfileQueryLogs(ctx context.Context, accountId, profileId string, page, limit int) ([]model.QueryLog, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	return p.QueryLogsService.DownloadProfileQueryLogs(ctx, profileId, profile.Settings.Logs.Retention, page, limit)
}

// GetProfileStatistics returns profile DNS statistics data
func (p *ProfileService) GetStatistics(ctx context.Context, accountId, profileId, timespan string) ([]model.StatisticsAggregated, error) {
	_, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	return p.StatisticsService.GetProfileStatistics(ctx, profileId, timespan)
}

// validateProfileIdAffiliation checks whether profile is within the current account profiles list
func (p *ProfileService) validateProfileIdAffiliation(ctx context.Context, accountId, profileId string) (*model.Profile, error) {
	profile, err := p.ProfileRepository.GetProfileById(ctx, profileId)
	if err != nil {
		if errors.Is(err, dbErrors.ErrProfileNotFound) {
			return nil, dbErrors.ErrProfileNotFound
		}
		return nil, err
	}

	if profile.AccountId != accountId {
		return nil, dbErrors.ErrProfileNotFound
	}
	return profile, nil
}

// DeleteProfileQueryLogs deletes profile DNS query logs
func (p *ProfileService) DeleteProfileQueryLogs(ctx context.Context, accountId, profileId string) error {
	_, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	return p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId)
}

// UpdateProfile updates profile data
func (p *ProfileService) UpdateProfile(ctx context.Context, accountId, profileId string, updates []model.ProfileUpdate) (*model.Profile, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	for _, update := range updates {
		// following code is a workaround for the case when the value is a map (openapi-cli-gen converts interface to {} in YAML spec, which is generated in python client as Dict[str, Any])
		internalValue, err := cast.ToStringMapE(update.Value)
		if err != nil {
			log.Trace().Msg("Failed to cast value to string map")
		} else {
			update.Value = internalValue["value"]
		}

		if strings.Contains(update.Path, "/settings/logs/") {
			err = p.handleQueryLogsSettingsUpdate(profile, update.Path, update)
			if err != nil {
				return nil, err
			}
		}

		if strings.Contains(update.Path, "/settings/statistics/") {
			err = p.handleStatisticsSettingsUpdate(profile, update.Path, update)
			if err != nil {
				return nil, err
			}
		}

		if strings.Contains(update.Path, "/settings/security/dnssec/") {
			if err = p.handleDNSSECSettingsUpdate(profile, update.Path, update); err != nil {
				return nil, err
			}
		}

		if strings.Contains(update.Path, "/settings/advanced/") {
			if err = p.handleAdvancedSettingsUpdate(profile, update.Path, update); err != nil {
				return nil, err
			}
		}

		switch update.Path {
		case "/name":
			err = p.handleProfileNameUpdate(ctx, profile, accountId, update)
			if err != nil {
				return nil, err
			}
		case "/settings/privacy/default_rule":
			err = p.handleDefaultRuleUpdate(profile, update)
			if err != nil {
				return nil, err
			}

			if profile.Settings.Privacy.DefaultRule == model.DEFAULT_RULE_BLOCK {
				// TODO: improve after wildcard support is implemented
				confguredDomains := make([]string, len(profile.Settings.CustomRules))
				for _, userRule := range profile.Settings.CustomRules {
					confguredDomains = append(confguredDomains, userRule.Value)
				}
				for _, domain := range p.ServerConfig.AllowedDomains {
					// whitelist DNS servers domains
					if !slices.Contains(confguredDomains, domain) {
						profile.Settings.CustomRules = append(profile.Settings.CustomRules, &model.CustomRule{
							ID:     primitive.NewObjectID(),
							Action: model.ACTION_ALLOW,
							Value:  domain,
						})
					}
				}
			}
		case "/settings/privacy/blocklists_subdomains_rule":
			err = p.handleBlocklistsSubdomainsRuleUpdate(profile, update)
			if err != nil {
				return nil, err
			}
		case "/settings/privacy/custom_rules_subdomains_rule":
			err = p.handleCustomRulesSubdomainsRuleUpdate(profile, update)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := p.ProfileRepository.Update(ctx, profileId, profile); err != nil {
		return nil, err
	}

	if err = p.Cache.CreateOrUpdateProfileSettings(ctx, profile.Settings, false); err != nil {
		return nil, err
	}
	return profile, err
}

func (p *ProfileService) handleQueryLogsSettingsUpdate(profile *model.Profile, updatePath string, update model.ProfileUpdate) error {
	switch updatePath {
	case "/settings/logs/enabled":
		return p.updateQueryLogsEnabled(profile, update)
	case "/settings/logs/log_clients_ips":
		return p.updateQueryLogsLogClientsIPs(profile, update)
	case "/settings/logs/log_domains":
		return p.updateQueryLogsLogDomains(profile, update)
	case "/settings/logs/retention":
		return p.updateQueryLogsRetention(profile, update)
	}

	return nil
}

func (p *ProfileService) handleStatisticsSettingsUpdate(profile *model.Profile, updatePath string, update model.ProfileUpdate) error {
	switch updatePath { // nolint
	case "/settings/statistics/enabled":
		return p.updateStatisticsEnabled(profile, update)
	}

	return nil
}

func (p *ProfileService) updateStatisticsEnabled(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		enabled, err := cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Statistics.Enabled = enabled
	}
	return nil
}

func (p *ProfileService) updateQueryLogsEnabled(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		enabled, err := cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Logs.Enabled = enabled
	}
	return nil
}

func (p *ProfileService) updateQueryLogsLogClientsIPs(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		logClientsIPs, err := cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Logs.LogClientsIPs = logClientsIPs
	}

	return nil
}

func (p *ProfileService) updateQueryLogsLogDomains(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		logDomains, err := cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Logs.LogDomains = logDomains
	}

	return nil
}

func (p *ProfileService) updateQueryLogsRetention(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		value, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}
		ret, err := model.NewRetention(value)
		if err != nil {
			return err
		}
		profile.Settings.Logs.Retention = ret
	}

	return nil
}

func (p *ProfileService) handleProfileNameUpdate(ctx context.Context, profile *model.Profile, accountId string, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		newName, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}
		profiles, err := p.ProfileRepository.GetProfilesByAccountId(ctx, accountId)
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			log.Trace().Str("profile_name", profile.Name).Msg("Checking for duplicate profile names")
			if profile.Name == newName {
				return ErrProfileNameAlreadyExists
			}
		}

		err = p.Validate.Struct(model.Profile{
			Name: newName,
		})
		if err != nil {
			log.Debug().Err(err).Msg("Failed to validate profile name")
			return ErrProfileNameCannotBeEmpty
		}

		profile.Name = newName
	}
	return nil
}

func (p *ProfileService) handleBlocklistsSubdomainsRuleUpdate(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		blockSubdomains, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Privacy.BlocklistsSubdomainsRule = blockSubdomains
		err = p.Validate.Struct(profile.Settings.Privacy)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to validate blocklists_subdomains_rule")
			return ErrBlocklistsSubdomainsInvalid
		}
	}
	return nil
}

func (p *ProfileService) handleCustomRulesSubdomainsRuleUpdate(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		value, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Privacy.CustomRulesSubdomainsRule = value
		err = p.Validate.Struct(profile.Settings.Privacy)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to validate custom_rules_subdomains_rule")
			return ErrCustomRulesSubdomainsInvalid
		}
	}
	return nil
}

func (p *ProfileService) handleDefaultRuleUpdate(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		defaultRule, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}

		profile.Settings.Privacy.DefaultRule = defaultRule
		err = p.Validate.Struct(profile.Settings.Privacy)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to validate default rule")
			return ErrDefaultRuleInvalid
		}
	}
	return nil
}

func (p *ProfileService) handleDNSSECSettingsUpdate(profile *model.Profile, updatePath string, update model.ProfileUpdate) error {
	switch updatePath { // nolint
	case "/settings/security/dnssec/enabled":
		return p.updateDNSSECEnabled(profile, update)
	case "/settings/security/dnssec/send_do_bit":
		return p.updateDNSSECOKBit(profile, update)
	}

	return nil
}

func (p *ProfileService) handleAdvancedSettingsUpdate(profile *model.Profile, updatePath string, update model.ProfileUpdate) error {
	switch updatePath { // nolint
	case "/settings/advanced/recursor":
		return p.updateRecursor(profile, update)
	}

	return nil
}

func (p *ProfileService) updateRecursor(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		recursor, err := cast.ToStringE(update.Value)
		if err != nil {
			return err
		}
		if !slices.Contains(model.RECURSORS, recursor) {
			return ErrRecursorInvalid
		}
		profile.Settings.Advanced.Recursor = recursor
	}
	return nil
}

func (p *ProfileService) updateDNSSECEnabled(profile *model.Profile, update model.ProfileUpdate) (err error) {
	var enabled bool
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		enabled, err = cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Security.DNSSECSettings.Enabled = enabled
	}

	return nil
}

func (p *ProfileService) updateDNSSECOKBit(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		send, err := cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Security.DNSSECSettings.SendDoBit = send
	}
	return nil
}
