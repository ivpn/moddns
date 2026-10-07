package profile

import (
	"context"
	"encoding/json"
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
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/utils"
	apivalidator "github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/blocklist"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/ivpn/dns/libs/servicescatalog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
)

// ServicesCatalogReader is a narrow interface for looking up service IDs in the
// catalog. It matches the public surface of *servicescatalogcache.Loader that
// the importer needs. Defined here (consumer-side) to avoid a dependency on the
// cache package and to keep mocking straightforward in tests.
type ServicesCatalogReader interface {
	Get() (*servicescatalog.Catalog, error)
}

// Account-based rate limiting for query logs retrieval.
const queryLogsRateLimitMax = 120
const queryLogsRateLimitWindow = time.Minute

// Cache-aside for the distinct-device aggregation (unindexable full-window
// bucket unpack — see api-endpoint-behaviour #J5).
const queryLogDevicesCachePrefix = "query_log_devices:"
const queryLogDevicesCacheTTL = 10 * time.Minute

type ProfileService struct {
	ProfileRepository repository.ProfileRepository
	AccountRepository repository.AccountRepository
	QueryLogsService  *querylogs.QueryLogsService
	StatisticsService *statistics.StatisticsService
	BlocklistService  *blocklist.BlocklistService
	ServicesCatalog   ServicesCatalogReader

	// MfaVerifier gates the password path of profile export/import reauth.
	// It is wired post-construction via SetMfaVerifier (typically pointed at
	// AccountService) because the construction order in service.New builds
	// ProfileService before AccountService. A nil MfaVerifier means "skip
	// MFA" — preserves the no-MFA test-setup behaviour and is appropriate
	// for unit tests that do not exercise the MFA path.
	MfaVerifier reauth.MfaVerifier

	// clientEnricher adds ASN and country to the top-clients list; nil leaves
	// those fields null (see SetClientEnricher).
	clientEnricher ClientEnricher

	// now is the clock; nil means time.Now (see SetClock).
	now func() time.Time

	// Zero means the defaults (see SetQueryLogsPurgeTimeouts).
	logsPurgeJobTime, logsPurgeRunTime time.Duration

	Cache         cache.Cache
	IdGen         idgen.Generator
	Validate      *validator.Validate
	ServerConfig  config.ServerConfig
	ServiceConfig config.ServiceConfig
}

// SetClock replaces the service clock; tests use it to fix time-derived values.
func (p *ProfileService) SetClock(now func() time.Time) { p.now = now }

// clock returns the current time in UTC.
func (p *ProfileService) clock() time.Time {
	if p.now == nil {
		return time.Now().UTC()
	}
	return p.now().UTC()
}

// SetMfaVerifier wires the MFA verifier used by the password path of profile
// export/import reauth. Call once during service-graph construction; the
// helper is safe to invoke before this is set (MFA is simply skipped — same
// behaviour as before the unification).
func (p *ProfileService) SetMfaVerifier(v reauth.MfaVerifier) {
	p.MfaVerifier = v
}

// NewProfileService creates a new profile service.
// servicesCatalog may be nil; when nil, service-ID catalog validation during
// import is skipped and all service IDs are accepted (safe-default for callers
// that do not need catalog validation, such as tests for unrelated features).
func NewProfileService(serverCfg config.ServerConfig, serviceCfg config.ServiceConfig, db repository.ProfileRepository, accountRepo repository.AccountRepository, blocklistService *blocklist.BlocklistService, qlService *querylogs.QueryLogsService, statsService *statistics.StatisticsService, servicesCatalog ServicesCatalogReader, cache cache.Cache, idGen idgen.Generator, validator *validator.Validate) *ProfileService {
	return &ProfileService{
		ProfileRepository: db,
		AccountRepository: accountRepo,
		BlocklistService:  blocklistService,
		ServicesCatalog:   servicesCatalog,
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
	name = apivalidator.NormalizeName(name)
	if name == "" {
		return nil, ErrProfileNameEmpty
	}
	if !apivalidator.IsSafeName(name) {
		return nil, ErrProfileNameInvalid
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
		log.Ctx(ctx).Trace().Str("profile_name", profile.Name).Msg("Checking for duplicate profile names")
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
		// delete statistics (ctx, not egCtx: a sibling failure must not cancel it);
		// leftovers of a failed purge are removed by the unconsented-statistics purge
		p.StatisticsService.PurgeBestEffort(ctx, profileId)
		return nil
	})

	eg.Go(func() (err error) {
		// delete all profile-related data from cache
		return p.Cache.DeleteProfileSettings(ctx, profileId)
	})

	if err := eg.Wait(); err != nil {
		log.Ctx(ctx).Err(err).Msg(ErrFailedToDeleteProfile.Error())
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
		log.Ctx(ctx).Err(tickErr).Msg("failed to tick query logs rate limiter")
	} else if !limiter.IsAllowed() {
		return nil, ErrQueryLogsRateLimited
	}

	return p.QueryLogsService.GetProfileQueryLogs(ctx, profileId, profile.Settings.Logs.Retention, status, timespan, deviceId, search, sortBy, page, limit)
}

// GetProfileQueryLogDevices returns the distinct device IDs seen in the
// profile's query logs (current retention window). Cache-aside with a short
// TTL: the aggregation must unpack every bucket of the profile's window
// (device_id is a measurement field — no index can serve the $group), which
// costs ~1.4s at 1M docs. Staleness is masked client-side: the frontend
// unions this list with device ids observed in fetched rows.
func (p *ProfileService) GetProfileQueryLogDevices(ctx context.Context, accountId, profileId string) ([]model.QueryLogDevice, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	cacheKey := queryLogDevicesCachePrefix + profileId
	if raw, cacheErr := p.Cache.Get(ctx, cacheKey); cacheErr == nil && raw != "" {
		var cached []model.QueryLogDevice
		if jsonErr := json.Unmarshal([]byte(raw), &cached); jsonErr == nil {
			return cached, nil
		}
	}

	devices, err := p.QueryLogsService.GetProfileQueryLogDevices(ctx, profileId, profile.Settings.Logs.Retention)
	if err != nil {
		return nil, err
	}
	// Never cache an empty list: a fresh profile queried before the collector's
	// first flush would otherwise pin "no devices" for the whole TTL. Empty-window
	// aggregations are cheap — there are no buckets to unpack.
	if len(devices) > 0 {
		if raw, jsonErr := json.Marshal(devices); jsonErr == nil {
			if cacheErr := p.Cache.Set(ctx, cacheKey, raw, queryLogDevicesCacheTTL); cacheErr != nil {
				log.Ctx(ctx).Warn().Err(cacheErr).Msg("failed to cache query log devices")
			}
		}
	}

	return devices, nil
}

// DownloadProfileQueryLogs returns all existing profile DNS query logs
func (p *ProfileService) DownloadProfileQueryLogs(ctx context.Context, accountId, profileId string, page, limit int) ([]model.QueryLog, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	return p.QueryLogsService.DownloadProfileQueryLogs(ctx, profileId, profile.Settings.Logs.Retention, page, limit)
}

// GetStatistics returns the profile's statistics for the timespan; the enabled gate lives in the statistics service.
func (p *ProfileService) GetStatistics(ctx context.Context, accountId, profileId, timespan string) (*model.StatisticsResponse, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	var settings *model.StatisticsSettings
	if profile.Settings != nil {
		settings = profile.Settings.Statistics
	}
	return p.StatisticsService.GetProfileStatistics(ctx, profileId, timespan, settings)
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

	if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
		return err
	}
	p.invalidateQueryLogCaches(ctx, profileId)
	return nil
}

// invalidateQueryLogCaches drops every cache derived from the profile's query logs so it
// cannot outlive the data (best-effort; TTL bounds a miss).
func (p *ProfileService) invalidateQueryLogCaches(ctx context.Context, profileId string) {
	if cacheErr := p.Cache.Del(ctx, queryLogDevicesCachePrefix+profileId); cacheErr != nil {
		log.Ctx(ctx).Warn().Err(cacheErr).Msg("failed to invalidate query log devices cache")
	}
	p.invalidateQueryLogTopCache(ctx, profileId)
}

// UpdateProfile validates every operation against the stored profile, then writes the
// touched fields as one atomic update (api-endpoint-behaviour.md G20-G23).
func (p *ProfileService) UpdateProfile(ctx context.Context, accountId, profileId string, updates []model.ProfileUpdate) (*model.Profile, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	touched, err := p.applyPatchOperations(ctx, profile, accountId, updates)
	if err != nil {
		return nil, err
	}
	if len(touched) == 0 {
		return profile, nil
	}

	upd := repository.ProfileFieldsUpdate{EnabledAtNow: p.clock()}
	var redisFields []cache.SettingsField
	for _, path := range touched {
		f := patchFields[path]
		value := f.value(profile)
		upd.Set = append(upd.Set, repository.FieldSet{Field: f.field, Value: value})
		if f.hash != "" {
			redisFields = append(redisFields, cache.SettingsField{Hash: f.hash, Field: f.hashField, Value: value})
		}
	}
	if slices.Contains(touched, pathDefaultRule) && profile.Settings.Privacy.DefaultRule == model.DEFAULT_RULE_BLOCK {
		// TODO: improve after wildcard support is implemented
		for _, domain := range p.ServerConfig.AllowedDomains {
			upd.AppendCustomRules = append(upd.AppendCustomRules, &model.CustomRule{
				ID:     primitive.NewObjectID(),
				Action: model.ACTION_ALLOW,
				Value:  domain,
			})
		}
	}

	before, after, err := p.ProfileRepository.UpdateFields(ctx, profileId, upd)
	if err != nil {
		return nil, err
	}

	var cacheErr error
	if len(redisFields) > 0 {
		cacheErr = p.Cache.SetProfileSettingsFields(ctx, profileId, redisFields)
	}
	// The proxy reads custom rules only from Redis (api-endpoint-behaviour.md G24).
	if appended := appendedCustomRules(after, upd.AppendCustomRules); len(appended) > 0 {
		if err := p.Cache.AddCustomRules(ctx, profileId, appended); err != nil && cacheErr == nil {
			cacheErr = err
		}
	}

	// The change is persisted in Mongo either way, so its side effects must run.
	beforeSnap := snapshotSettings(before.Settings)
	p.applySettingsTransitions(ctx, profileId, beforeSnap, beforeSnap.patched(touched, profile.Settings))
	if cacheErr != nil {
		return nil, cacheErr
	}
	return after, nil
}

// appendedCustomRules returns the candidates the update actually stored, as re-read.
func appendedCustomRules(after *model.Profile, candidates []*model.CustomRule) []*model.CustomRule {
	if len(candidates) == 0 || after == nil || after.Settings == nil {
		return nil
	}
	var out []*model.CustomRule
	for _, rule := range after.Settings.CustomRules {
		if slices.ContainsFunc(candidates, func(c *model.CustomRule) bool { return c.ID == rule.ID }) {
			out = append(out, rule)
		}
	}
	return out
}

// applyPatchOperations validates and applies every operation to profile in memory and
// returns the paths that replace a value, in first-seen order.
func (p *ProfileService) applyPatchOperations(ctx context.Context, profile *model.Profile, accountId string, updates []model.ProfileUpdate) ([]string, error) {
	var touched []string
	for _, update := range updates {
		// following code is a workaround for the case when the value is a map (openapi-cli-gen converts interface to {} in YAML spec, which is generated in python client as Dict[str, Any])
		internalValue, err := cast.ToStringMapE(update.Value)
		if err != nil {
			log.Ctx(ctx).Trace().Msg("Failed to cast value to string map")
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

		if strings.Contains(update.Path, "/settings/security/rebinding_protection/") {
			if err = p.handleRebindingProtectionUpdate(profile, update.Path, update); err != nil {
				return nil, err
			}
		}

		if strings.Contains(update.Path, "/settings/advanced/") {
			if err = p.handleAdvancedSettingsUpdate(profile, update.Path, update); err != nil {
				return nil, err
			}
		}

		if _, ok := patchFields[update.Path]; ok && update.Operation == model.UpdateOperationReplace && !slices.Contains(touched, update.Path) {
			touched = append(touched, update.Path)
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

	return touched, nil
}

const (
	pathStatisticsEnabled   = "/settings/statistics/enabled"
	pathStatisticsRetention = "/settings/statistics/retention"
	pathLogsEnabled         = "/settings/logs/enabled"
	pathDefaultRule         = "/settings/privacy/default_rule"
)

// patchField maps a PATCH path to its stored field and, when the proxy reads it, its
// Redis settings hash field.
type patchField struct {
	field     string
	hash      string
	hashField string
	value     func(*model.Profile) any
}

var patchFields = map[string]patchField{
	"/name":                                           {field: "name", value: func(p *model.Profile) any { return p.Name }},
	pathStatisticsEnabled:                             {"settings.statistics.enabled", "statistics", "enabled", func(p *model.Profile) any { return p.Settings.Statistics.Enabled }},
	pathStatisticsRetention:                           {"settings.statistics.retention", "statistics", "retention", func(p *model.Profile) any { return p.Settings.Statistics.Retention }},
	pathLogsEnabled:                                   {"settings.logs.enabled", "logs", "enabled", func(p *model.Profile) any { return p.Settings.Logs.Enabled }},
	"/settings/logs/log_clients_ips":                  {"settings.logs.log_clients_ips", "logs", "log_clients_ips", func(p *model.Profile) any { return p.Settings.Logs.LogClientsIPs }},
	"/settings/logs/log_domains":                      {"settings.logs.log_domains", "logs", "log_domains", func(p *model.Profile) any { return p.Settings.Logs.LogDomains }},
	"/settings/logs/retention":                        {"settings.logs.retention", "logs", "retention", func(p *model.Profile) any { return p.Settings.Logs.Retention }},
	pathDefaultRule:                                   {"settings.privacy.default_rule", "privacy", "default_rule", func(p *model.Profile) any { return p.Settings.Privacy.DefaultRule }},
	"/settings/privacy/blocklists_subdomains_rule":    {"settings.privacy.blocklists_subdomains_rule", "privacy", "blocklists_subdomains_rule", func(p *model.Profile) any { return p.Settings.Privacy.BlocklistsSubdomainsRule }},
	"/settings/privacy/custom_rules_subdomains_rule":  {"settings.privacy.custom_rules_subdomains_rule", "privacy", "custom_rules_subdomains_rule", func(p *model.Profile) any { return p.Settings.Privacy.CustomRulesSubdomainsRule }},
	"/settings/security/dnssec/enabled":               {"settings.security.dnssec.enabled", "security:dnssec", "enabled", func(p *model.Profile) any { return p.Settings.Security.DNSSECSettings.Enabled }},
	"/settings/security/dnssec/send_do_bit":           {"settings.security.dnssec.send_do_bit", "security:dnssec", "send_do_bit", func(p *model.Profile) any { return p.Settings.Security.DNSSECSettings.SendDoBit }},
	"/settings/security/rebinding_protection/enabled": {"settings.security.rebinding_protection.enabled", "security:rebinding_protection", "enabled", func(p *model.Profile) any { return p.Settings.Security.RebindingProtection.Enabled }},
	"/settings/advanced/recursor":                     {"settings.advanced.recursor", "advanced", "recursor", func(p *model.Profile) any { return p.Settings.Advanced.Recursor }},
}

// settingsSnapshot is the part of the settings whose transitions have side effects.
type settingsSnapshot struct {
	statistics  *model.StatisticsSettings
	logsEnabled bool
}

func snapshotSettings(s *model.ProfileSettings) settingsSnapshot {
	var snap settingsSnapshot
	if s == nil {
		return snap
	}
	if s.Statistics != nil {
		c := *s.Statistics
		snap.statistics = &c
	}
	snap.logsEnabled = s.Logs != nil && s.Logs.Enabled
	return snap
}

// patched is the snapshot with this PATCH's values applied to the paths it touched.
func (s settingsSnapshot) patched(touched []string, patch *model.ProfileSettings) settingsSnapshot {
	out := s
	if slices.Contains(touched, pathStatisticsEnabled) || slices.Contains(touched, pathStatisticsRetention) {
		next := model.StatisticsSettings{}
		if s.statistics != nil {
			next = *s.statistics
		}
		if slices.Contains(touched, pathStatisticsEnabled) {
			next.Enabled = patch.Statistics.Enabled
		}
		if slices.Contains(touched, pathStatisticsRetention) {
			next.Retention = patch.Statistics.Retention
		}
		out.statistics = &next
	}
	if slices.Contains(touched, pathLogsEnabled) {
		out.logsEnabled = patch.Logs.Enabled
	}
	return out
}

func (s settingsSnapshot) statisticsRetention() model.StatisticsRetention {
	if s.statistics == nil {
		return ""
	}
	return s.statistics.Retention
}

func (s settingsSnapshot) statisticsEnabled() bool {
	return s.statistics != nil && s.statistics.Enabled
}

// applySettingsTransitions runs the side effects of a settings change once per
// PATCH, after the change is persisted, however many paths it touched.
func (p *ProfileService) applySettingsTransitions(ctx context.Context, profileId string, before, after settingsSnapshot) {
	if before.statisticsEnabled() && !after.statisticsEnabled() {
		p.StatisticsService.PurgeBestEffort(ctx, profileId)
	}
	if after.statisticsEnabled() && after.statistics.Retention.Window() < before.statisticsRetention().Window() {
		p.StatisticsService.MoveBestEffort(ctx, profileId, after.statistics.Retention)
	}
	if before.logsEnabled && !after.logsEnabled {
		p.purgeQueryLogsBestEffort(ctx, profileId)
	}
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
	case pathStatisticsRetention:
		return p.updateStatisticsRetention(profile, update)
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

func (p *ProfileService) updateStatisticsRetention(profile *model.Profile, update model.ProfileUpdate) error {
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		value, err := cast.ToStringE(update.Value)
		if err != nil {
			return ErrStatisticsRetentionInvalid
		}
		retention := model.StatisticsRetention(value)
		if !retention.Valid() {
			return ErrStatisticsRetentionInvalid
		}
		profile.Settings.Statistics.Retention = retention
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
		newName = apivalidator.NormalizeName(newName)
		if newName == "" {
			return ErrProfileNameCannotBeEmpty
		}
		err = p.Validate.Struct(model.Profile{
			Name: newName,
		})
		if err != nil {
			log.Ctx(ctx).Debug().Err(err).Msg("Failed to validate profile name")
			return ErrProfileNameInvalid
		}

		profiles, err := p.ProfileRepository.GetProfilesByAccountId(ctx, accountId)
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			log.Ctx(ctx).Trace().Str("profile_name", profile.Name).Msg("Checking for duplicate profile names")
			if profile.Name == newName {
				return ErrProfileNameAlreadyExists
			}
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

func (p *ProfileService) handleRebindingProtectionUpdate(profile *model.Profile, updatePath string, update model.ProfileUpdate) error {
	switch updatePath { // nolint
	case "/settings/security/rebinding_protection/enabled":
		return p.updateRebindingProtectionEnabled(profile, update)
	}

	return nil
}

func (p *ProfileService) updateRebindingProtectionEnabled(profile *model.Profile, update model.ProfileUpdate) (err error) {
	var enabled bool
	switch update.Operation { // nolint
	case model.UpdateOperationReplace:
		enabled, err = cast.ToBoolE(update.Value)
		if err != nil {
			return err
		}
		profile.Settings.Security.RebindingProtection.Enabled = enabled
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
