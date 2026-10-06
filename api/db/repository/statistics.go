package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

type StatisticsRepository interface {
	GetProfileStatistics(ctx context.Context, profileId string, timespan int) ([]model.StatisticsAggregated, error)
	// ListStatisticsProfileIDs returns the distinct profile ids present in any retention collection.
	ListStatisticsProfileIDs(ctx context.Context) ([]string, error)
	// DeleteProfileStatistics removes the profile's statistics from every tier. A non-nil
	// before is an instant; each tier deletes the buckets starting before the bucket of its
	// own width (15 min, 1 h, 1 d, UTC) that contains it.
	DeleteProfileStatistics(ctx context.Context, profileId string, before *time.Time) error
}
