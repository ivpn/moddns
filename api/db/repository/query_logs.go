package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

type QueryLogsRepository interface {
	GetQueryLogs(ctx context.Context, profileId string, retention model.Retention, status string, timespan int, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error)
	DeleteQueryLogs(ctx context.Context, profileId string) error
}
