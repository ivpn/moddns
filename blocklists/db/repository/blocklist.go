package repository

import (
	"context"

	"github.com/ivpn/dns/blocklists/model"
)

// BlocklistRepository represents a blocklist repository
type BlocklistRepository interface {
	UpsertMetadata(ctx context.Context, blocklist model.BlocklistMetadata) error
	UpsertContent(ctx context.Context, blocklist model.BlocklistContent) error
	GetMetadata(ctx context.Context, filter map[string]any) ([]model.BlocklistMetadata, error)
	GetContent(ctx context.Context, filter map[string]any) ([]model.BlocklistContent, error)
	Delete(ctx context.Context, filter map[string]any) error
	DeleteMetadata(ctx context.Context, filter map[string]any) error
}
