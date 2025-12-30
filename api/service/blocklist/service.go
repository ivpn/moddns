package blocklist

import (
	"context"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
)

type BlocklistService struct {
	BlocklistRepository repository.BlocklistRepository
	Cache               cache.Cache
}

const defaultBlocklistSort = "updated"

// NewBlocklistService creates a new blocklist service
func NewBlocklistService(db repository.BlocklistRepository, cache cache.Cache) *BlocklistService {
	return &BlocklistService{
		BlocklistRepository: db,
		Cache:               cache,
	}
}

// Get returns blocklists based on filtering criteria
func (p *BlocklistService) GetBlocklist(ctx context.Context, filter map[string]any, sortBy string) ([]*model.Blocklist, error) {
	if sortBy == "" {
		sortBy = defaultBlocklistSort
	}

	blocklists, err := p.BlocklistRepository.Get(ctx, filter, sortBy)
	if err != nil {
		return nil, err
	}
	return blocklists, nil
}
