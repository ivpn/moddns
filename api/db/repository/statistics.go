package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

type StatisticsRepository interface {
	GetProfileStatistics(ctx context.Context, profileId string, timespan int) ([]model.StatisticsAggregated, error)
	// DeleteProfileStatistics removes the profile's statistics from every retention
	// collection; a non-nil before limits it to buckets starting before that instant.
	DeleteProfileStatistics(ctx context.Context, profileId string, before *time.Time) error
}
