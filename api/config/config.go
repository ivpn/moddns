package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/store"
)

// parseBoolEnv returns true if the environment variable equals "true" (case-sensitive)
func parseBoolEnv(key string) bool {
	return os.Getenv(key) == "true"
}

// envOrDefault returns the value of the environment variable or fallback if empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Config represents the application configuration
type Config struct {
	Server  *ServerConfig
	Service *ServiceConfig
	API     *APIConfig
	DB      *store.Config
	Cache   *cache.Config
	Email   *EmailSenderConfig
	Sentry  *SentryConfig
}

// ServiceConfig represents the service configuration
type ServiceConfig struct {
	OTPExpirationTime          time.Duration
	MobileConfigPrivateKeyPath string
	MobileConfigCertPath       string
	IdLimiterMax               int
	IdLimiterExpiration        time.Duration
	MaxProfiles                int
	MaxCredentials             int
	ServicesCatalogPath        string
	ServicesCatalogReloadEvery time.Duration

	// Startup migrations (removable after all environments are migrated)
	MigrateSubscriptionUUIDSubtype bool
}

// SentryConfig represents the Sentry configuration
type SentryConfig struct {
	DSN         string
	Environment string
	Release     string
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Name            string
	FQDN            string
	DnsDomain       string
	ServerAddresses []string
	FrontendDomain  string
	AllowedDomains  []string
}

// APIConfig represents the API configuration
type APIConfig struct {
	Port                  string
	BasicAuthUser         string
	BasicAuthPassword     string
	ApiAllowOrigin        string
	ApiAllowIP            string
	SessionExpirationTime time.Duration
	SessionLimit          int64
	ProfileIDMinLength    int
	PSK                   string
	SignupWebhookURL      string
	SignupWebhookPSK      string
	DisableRateLimit      bool
	PreauthURL            string
	PreauthPSK            string
	PreauthTTL            time.Duration
}

type EmailSenderConfig struct {
	SenderType string
	InboxId    string
	AuthToken  string //nolint:gosec // G117 - intentional sensitive field
}

// New creates a new Config instance
func New() (*Config, error) {
	cacheAddrs := strings.Split(os.Getenv("CACHE_ADDRESSES"), ",")

	serverName := envOrDefault("SERVER_NAME", "modDNS API")
	serverFQDN := envOrDefault("SERVER_FQDN", "app.moddns.net")

	envAllowedDomains := os.Getenv("SERVER_ALLOWED_DOMAINS")
	if envAllowedDomains == "" {
		return nil, errors.New("SERVER_ALLOWED_DOMAINS is not set")
	}
	allowedDomains := strings.Split(envAllowedDomains, ",")

	envDnsServerAddresses := os.Getenv("SERVER_DNS_SERVER_ADDRESSES")
	if envDnsServerAddresses == "" {
		return nil, errors.New("SERVER_DNS_SERVER_ADDRESSES is not set")
	}
	dnsServerAddresses := strings.Split(envDnsServerAddresses, ",")

	otpExp, err := time.ParseDuration(envOrDefault("OTP_EXPIRATION", "5m"))
	if err != nil {
		return nil, err
	}

	preauthTTL, err := time.ParseDuration(envOrDefault("PREAUTH_TTL", "60m"))
	if err != nil {
		return nil, err
	}

	sessionLimitInt, err := strconv.ParseInt(envOrDefault("API_SESSION_LIMIT", "10"), 10, 64)
	if err != nil {
		return nil, err
	}

	sessionExp, err := time.ParseDuration(envOrDefault("API_SESSION_EXPIRATION", "1h"))
	if err != nil {
		return nil, err
	}

	maxProfiles, err := strconv.Atoi(envOrDefault("MAX_PROFILES", "100"))
	if err != nil {
		return nil, err
	}

	maxCredentialsStr := os.Getenv("MAX_CREDENTIALS")
	if maxCredentialsStr == "" {
		log.Debug().Msg("MAX_CREDENTIALS is not set, defaulting to 10")
		maxCredentialsStr = "10"
	}
	maxCredentials, err := strconv.Atoi(maxCredentialsStr)
	if err != nil {
		return nil, err
	}
	profileIDMinLen, err := strconv.Atoi(envOrDefault("PROFILE_ID_MIN_LENGTH", "10"))
	if err != nil {
		return nil, err
	}
	if profileIDMinLen <= 0 {
		profileIDMinLen = 10
	}

	idLimiterMax, err := strconv.Atoi(envOrDefault("ID_LIMITER_MAX", "5"))
	if err != nil {
		return nil, err
	}
	idLimiterExpiration, err := time.ParseDuration(envOrDefault("ID_LIMITER_EXPIRATION", "1h"))
	if err != nil {
		return nil, err
	}

	servicesCatalogPath := envOrDefault("SERVICES_CATALOG_PATH", "/opt/services/catalog.yml")
	servicesCatalogReloadEvery, err := time.ParseDuration(envOrDefault("SERVICES_CATALOG_RELOAD", "5m"))
	if err != nil {
		return nil, err
	}

	// Warn about missing security-critical configuration.
	if os.Getenv("API_PSK") == "" {
		log.Warn().Msg("API_PSK is not set; the subscription provisioning endpoint will reject all requests")
	}
	if os.Getenv("API_BASIC_AUTH_USER") == "" || os.Getenv("API_BASIC_AUTH_PASSWORD") == "" {
		log.Warn().Msg("API_BASIC_AUTH_USER or API_BASIC_AUTH_PASSWORD is not set; Swagger docs endpoint will be unprotected")
	}
	if os.Getenv("PREAUTH_URL") == "" {
		log.Warn().Msg("PREAUTH_URL is not set; ZLA pre-auth session flow will be unavailable")
	}

	return &Config{
		Server: &ServerConfig{
			Name:            serverName,
			FQDN:            serverFQDN,
			DnsDomain:       os.Getenv("SERVER_DNS_DOMAIN"),
			ServerAddresses: dnsServerAddresses,
			FrontendDomain:  os.Getenv("SERVER_FRONTEND_DOMAIN"),
			AllowedDomains:  allowedDomains,
		},
		API: &APIConfig{
			Port:                  os.Getenv("API_PORT"),
			BasicAuthUser:         os.Getenv("API_BASIC_AUTH_USER"),
			BasicAuthPassword:     os.Getenv("API_BASIC_AUTH_PASSWORD"),
			ApiAllowOrigin:        os.Getenv("API_ALLOW_ORIGIN"),
			ApiAllowIP:            os.Getenv("API_ALLOW_IP"),
			SessionExpirationTime: sessionExp,
			SessionLimit:          sessionLimitInt,
			ProfileIDMinLength:    profileIDMinLen,
			PSK:                   os.Getenv("API_PSK"),
			SignupWebhookURL:      os.Getenv("API_SIGNUP_WEBHOOK_URL"),
			SignupWebhookPSK:      os.Getenv("API_SIGNUP_WEBHOOK_PSK"),
			DisableRateLimit:      parseBoolEnv("API_DISABLE_RATE_LIMIT"),
			PreauthURL:            os.Getenv("PREAUTH_URL"),
			PreauthPSK:            os.Getenv("PREAUTH_PSK"),
			PreauthTTL:            preauthTTL,
		},
		DB: &store.Config{
			DbURI:    os.Getenv("DB_URI"),
			Name:     os.Getenv("DB_NAME"),
			Username: os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
			AuthSource: func() string {
				v := os.Getenv("DB_AUTH_SOURCE")
				if v == "" {
					return "dns"
				}
				return v
			}(),
			MigrationsSource:      os.Getenv("DB_MIGRATIONS_SOURCE"),
			TLSEnabled:            parseBoolEnv("DB_TLS_ENABLED"),
			CertFile:              os.Getenv("DB_CERT_FILE"),
			KeyFile:               os.Getenv("DB_KEY_FILE"),
			CACertFile:            os.Getenv("DB_CA_CERT_FILE"),
			TLSInsecureSkipVerify: parseBoolEnv("DB_TLS_INSECURE_SKIP_VERIFY"),
		},
		Cache: &cache.Config{
			Address:               os.Getenv("CACHE_ADDRESS"),
			FailoverAddresses:     cacheAddrs,
			Username:              os.Getenv("CACHE_USERNAME"),
			Password:              os.Getenv("CACHE_PASSWORD"),
			FailoverPassword:      os.Getenv("CACHE_FAILOVER_PASSWORD"),
			FailoverUsername:      os.Getenv("CACHE_FAILOVER_USERNAME"),
			MasterName:            os.Getenv("CACHE_MASTER_NAME"),
			TLSEnabled:            parseBoolEnv("CACHE_TLS_ENABLED"),
			CertFile:              os.Getenv("CACHE_CERT_FILE"),
			KeyFile:               os.Getenv("CACHE_KEY_FILE"),
			CACertFile:            os.Getenv("CACHE_CA_CERT_FILE"),
			TLSInsecureSkipVerify: parseBoolEnv("CACHE_TLS_INSECURE_SKIP_VERIFY"),
		},
		Email: &EmailSenderConfig{
			SenderType: os.Getenv("EMAIL_SENDER_TYPE"),
			InboxId:    os.Getenv("EMAIL_SENDER_INBOX_ID"),
			AuthToken:  os.Getenv("EMAIL_SENDER_AUTH_TOKEN"),
		},
		Service: &ServiceConfig{
			OTPExpirationTime:              otpExp,
			MobileConfigPrivateKeyPath:     os.Getenv("MOBILECONFIG_PRIVATE_KEY_PATH"),
			MobileConfigCertPath:           os.Getenv("MOBILECONFIG_CERT_PATH"),
			IdLimiterMax:                   idLimiterMax,
			IdLimiterExpiration:            idLimiterExpiration,
			MaxProfiles:                    maxProfiles,
			MaxCredentials:                 maxCredentials,
			ServicesCatalogPath:            servicesCatalogPath,
			ServicesCatalogReloadEvery:     servicesCatalogReloadEvery,
			MigrateSubscriptionUUIDSubtype: parseBoolEnv("MIGRATE_SUBSCRIPTION_UUID_SUBTYPE"),
		},
		Sentry: &SentryConfig{
			DSN:         os.Getenv("SENTRY_DSN"),
			Environment: os.Getenv("SENTRY_ENVIRONMENT"),
			Release:     os.Getenv("SENTRY_RELEASE"),
		},
	}, nil
}
