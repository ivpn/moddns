package requestcontext

import (
	"context"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/model"
	"github.com/rs/zerolog"
)

type RequestContext struct {
	// Ctx                     context.Context
	ProfileId               string                  `json:"profile_id"`
	DeviceId                string                  `json:"device_id"`
	PrivacySettings         map[string]string       `json:"privacy_settings"`
	LogsSettings            map[string]string       `json:"logs_settings"`
	AdvancedSettings        map[string]string       `json:"advanced_settings"`
	DNSSECSettings          map[string]string       `json:"dnssec_settings"`
	PartialFilteringResults []model.StageResult     `json:"partial_filtering_results"`
	FilterResult            model.FilterResult      `json:"filter_result"`
	Logger                  logging.LoggerInterface `json:"-"`
	LoggerConfig            logging.LoggingConfig   `json:"logger_config"`
}

func NewRequestContext(ctx context.Context, p *proxy.Proxy, profileId string, deviceId string, privacySettings, logsSettings, dnssecSettings, advancedSettings map[string]string, logger logging.LoggerInterface) *RequestContext {
	return &RequestContext{
		// Ctx:              ctx,
		ProfileId:        profileId,
		DeviceId:         deviceId,
		PrivacySettings:  privacySettings,
		LogsSettings:     logsSettings,
		DNSSECSettings:   dnssecSettings,
		AdvancedSettings: advancedSettings,
		Logger:           logger,
		LoggerConfig:     logger.Config(),
	}
}

// DomainLoggingEnabled returns true if logs settings allow domain logging.
func (r *RequestContext) DomainLoggingEnabled() bool {
	return r.LoggerConfig.LogDomains
}

// ClientIPLoggingEnabled returns true if logs settings allow client IP logging.
func (r *RequestContext) ClientIPLoggingEnabled() bool {
	return r.LoggerConfig.LogClientIPs
}

// AddDomain conditionally adds the domain field to the provided zerolog event.
// Returns the same event for chaining.
func (r *RequestContext) AddDomain(e *zerolog.Event, domain string) *zerolog.Event {
	if r.DomainLoggingEnabled() {
		return e.Str("domain", domain)
	}
	return e
}

// MaybeDomain conditionally adds any domain-like string field.
func (r *RequestContext) MaybeDomain(e *zerolog.Event, key, value string) *zerolog.Event {
	if r.DomainLoggingEnabled() {
		return e.Str(key, value)
	}
	return e
}

// AddClientIP conditionally adds the client_ip field.
func (r *RequestContext) AddClientIP(e *zerolog.Event, ip string) *zerolog.Event {
	if r.ClientIPLoggingEnabled() {
		return e.Str("client_ip", ip)
	}
	return e
}

// MaybeClientIP conditionally adds a custom client IP related field.
func (r *RequestContext) MaybeClientIP(e *zerolog.Event, key, value string) *zerolog.Event {
	if r.ClientIPLoggingEnabled() {
		return e.Str(key, value)
	}
	return e
}
