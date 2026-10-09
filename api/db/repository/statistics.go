package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

type StatisticsRepository interface {
	// GetProfileStatistics aggregates the profile's buckets of one tier with
	// bucket_start in [from, to); series points are `bucket` wide.
	GetProfileStatistics(ctx context.Context, profileId string, tier model.StatisticsTier, from, to time.Time, bucket time.Duration) (*model.StatisticsAggregate, error)
	// ListStatisticsProfileIDs returns the distinct profile ids present in any retention collection.
	ListStatisticsProfileIDs(ctx context.Context) ([]string, error)
	// DeleteProfileStatistics removes the profile's statistics from every tier. A non-nil
	// before is an instant; each tier deletes the buckets starting before the bucket of its
	// own width (15 min, 1 h, 1 d, UTC) that contains it.
	// It returns the number of documents deleted across the collections.
	DeleteProfileStatistics(ctx context.Context, profileId string, before *time.Time) (int64, error)
	// DeleteProfileStatisticsThrough removes the profile's buckets up to and including the one
	// of each tier's width that contains at (api-endpoint-behaviour.md J53).
	DeleteProfileStatisticsThrough(ctx context.Context, profileId string, at time.Time) (int64, error)
	// MoveProfileDailyStatistics leaves the profile's daily statistics only in collections no
	// longer than to: documents of longer daily collections with bucket_start >= since are
	// copied unchanged into to's collection, then removed there with the older ones. It
	// returns the number of documents copied.
	MoveProfileDailyStatistics(ctx context.Context, profileId string, to model.StatisticsRetention, since time.Time) (int, error)
}
