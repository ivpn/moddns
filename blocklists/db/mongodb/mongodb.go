package mongodb

import (
	"github.com/ivpn/dns/libs/store"
)

const (
	collNameAccounts           = "accounts"
	collNameProfiles           = "profiles"
	collNameSettings           = "settings"
	collNameBlocklistsMetadata = "blocklists_metadata"
	collNameBlocklists         = "blocklists"
	collNameQueryLogs          = "query_logs"
)

// MongoDB is a MongoDB database instance
type MongoDB struct {
	store.Store
	BlocklistRepository
}

// New creates a new MongoDB instance
func New(storeI store.Store, config *store.Config) (*MongoDB, error) {
	client := storeI.GetClient()

	return &MongoDB{
		Store:               storeI,
		BlocklistRepository: NewBlocklistRepository(client, config.Name, collNameBlocklistsMetadata, collNameBlocklists),
	}, nil
}
