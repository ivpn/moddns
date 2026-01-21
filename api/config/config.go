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
	OTPExpirationTime           time.Duration
	MobileConfigPrivateKeyPath  string
	MobileConfigCertPath        string
	IdLimiterMax                int
	IdLimiterExpiration         time.Duration
	MaxProfiles                 int
	MaxCredentials              int
	SubscriptionCacheExpiration time.Duration
	ServicesCatalogPath         string
	ServicesCatalogReloadEvery  time.Duration
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
}

type EmailSenderConfig struct {
	SenderType string
	InboxId    string
	AuthToken  string
}

// New creates a new Config instance
func New() (*Config, error) {
	cacheAddrs := strings.Split(os.Getenv("CACHE_ADDRESSES"), ",")

	serverName := os.Getenv("SERVER_NAME")
	if serverName == "" {
		serverName = "modDNS API"
	}
	serverFQDN := os.Getenv("SERVER_FQDN")
	if serverFQDN == "" {
		serverFQDN = "app.moddns.net"
	}

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

	otpExpStr := os.Getenv("OTP_EXPIRATION")
	if otpExpStr == "" {
		otpExpStr = "5m"
	}
	otpExp, err := time.ParseDuration(otpExpStr)
	if err != nil {
		return nil, err
	}

	// subscription cache expiration (used for AddSubscription endpoint)
	subCacheExpStr := os.Getenv("SUBSCRIPTION_CACHE_EXPIRATION")
	if subCacheExpStr == "" {
		subCacheExpStr = "15m"
	}
	subCacheExp, err := time.ParseDuration(subCacheExpStr)
	if err != nil {
		return nil, err
	}

	sessionLimit := os.Getenv("API_SESSION_LIMIT")
	if sessionLimit == "" {
		sessionLimit = "10"
	}
	sessionLimitInt, err := strconv.ParseInt(sessionLimit, 10, 64)
	if err != nil {
		return nil, err
	}

	sessionExpStr := os.Getenv("API_SESSION_EXPIRATION")
	if sessionExpStr == "" {
		sessionExpStr = "1h"
	}
	sessionExp, err := time.ParseDuration(sessionExpStr)
	if err != nil {
		return nil, err
	}

	maxProfilesStr := os.Getenv("MAX_PROFILES")
	if maxProfilesStr == "" {
		maxProfilesStr = "100"
	}
	maxProfiles, err := strconv.Atoi(maxProfilesStr)
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
	profileIDMinLenStr := os.Getenv("PROFILE_ID_MIN_LENGTH")
	if profileIDMinLenStr == "" {
		profileIDMinLenStr = "10"
	}
	profileIDMinLen, err := strconv.Atoi(profileIDMinLenStr)
	if err != nil || profileIDMinLen <= 0 {
		profileIDMinLen = 10
	}
	if err != nil {
		return nil, err
	}

	idLimiterEnv := os.Getenv("ID_LIMITER_MAX")
	if idLimiterEnv == "" {
		idLimiterEnv = "5"
	}
	idLimiterMax, err := strconv.Atoi(idLimiterEnv)
	if err != nil {
		return nil, err
	}
	idLimiterExpirationStr := os.Getenv("ID_LIMITER_EXPIRATION")
	if idLimiterExpirationStr == "" {
		idLimiterExpirationStr = "1h"
	}
	idLimiterExpiration, err := time.ParseDuration(idLimiterExpirationStr)
	if err != nil {
		return nil, err
	}

	servicesCatalogPath := os.Getenv("SERVICES_CATALOG_PATH")
	if servicesCatalogPath == "" {
		servicesCatalogPath = "./services/catalog.yml"
	}
	servicesCatalogReloadEveryStr := os.Getenv("SERVICES_CATALOG_RELOAD")
	if servicesCatalogReloadEveryStr == "" {
		servicesCatalogReloadEveryStr = "5m"
	}
	servicesCatalogReloadEvery, err := time.ParseDuration(servicesCatalogReloadEveryStr)
	if err != nil {
		return nil, err
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
			OTPExpirationTime:           otpExp,
			MobileConfigPrivateKeyPath:  os.Getenv("MOBILECONFIG_PRIVATE_KEY_PATH"),
			MobileConfigCertPath:        os.Getenv("MOBILECONFIG_CERT_PATH"),
			IdLimiterMax:                idLimiterMax,
			IdLimiterExpiration:         idLimiterExpiration,
			MaxProfiles:                 maxProfiles,
			MaxCredentials:              maxCredentials,
			SubscriptionCacheExpiration: subCacheExp,
			ServicesCatalogPath:         servicesCatalogPath,
			ServicesCatalogReloadEvery:  servicesCatalogReloadEvery,
		},
		Sentry: &SentryConfig{
			DSN:         os.Getenv("SENTRY_DSN"),
			Environment: os.Getenv("SENTRY_ENVIRONMENT"),
			Release:     os.Getenv("SENTRY_RELEASE"),
		},
	}, nil
}
