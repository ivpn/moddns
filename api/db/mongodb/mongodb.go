package mongodb

import (
	"context"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/libs/store"
)

const (
	collNameAccounts           = "accounts"
	collNameProfiles           = "profiles"
	collNameSubscriptions      = "subscriptions"
	collNameBlocklistsMetadata = "blocklists_metadata"
	collNameQueryLogs          = "query_logs"
	collNameSessions           = "sessions"
)

// MongoDB is a MongoDB database instance
type MongoDB struct {
	store.Store
	AccountRepository
	ProfileRepository
	BlocklistRepository
	QueryLogsRepository
	SubscriptionRepository
	StatisticsRepository
	SessionRepository
	CredentialRepository
}

// New creates a new MongoDB instance
func New(ctx context.Context, storeI store.Store, config *config.Config) (*MongoDB, error) {
	client := storeI.GetClient()

	sessionRepo, err := NewSessionRepository(ctx, client, config.DB.Name, config.API.SessionExpirationTime, collNameSessions)
	if err != nil {
		return nil, err
	}

	return &MongoDB{
		Store:                  storeI,
		AccountRepository:      NewAccountRepository(client, config.DB.Name, collNameAccounts),
		ProfileRepository:      NewProfileRepository(client, config.DB.Name, collNameProfiles),
		QueryLogsRepository:    NewQueryLogsRepository(client, config.DB.Name, collNameQueryLogs),
		BlocklistRepository:    NewBlocklistRepository(client, config.DB.Name, collNameBlocklistsMetadata),
		SubscriptionRepository: NewSubscriptionRepository(client, config.DB.Name, collNameSubscriptions),
		StatisticsRepository:   NewStatisticsRepository(client, config.DB.Name),
		SessionRepository:      sessionRepo,
		CredentialRepository:   NewCredentialRepository(client, config.DB.Name, CredentialsCollection),
	}, nil
}
