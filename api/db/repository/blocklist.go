package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

// BlocklistRepository represents a blocklist repository
type BlocklistRepository interface {
	Get(ctx context.Context, filter map[string]any, sortBy string) ([]*model.Blocklist, error)
}
