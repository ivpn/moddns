package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

type QueryLogsRepository interface {
	GetQueryLogs(ctx context.Context, profileId string, retention model.Retention, status string, timespan int, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error)
	GetQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error)
	GetQueryLogTopDomains(ctx context.Context, profileId string, retention model.Retention, status string, timespanHours, limit int) ([]model.QueryLogTopDomain, error)
	GetQueryLogTopClients(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopClient, error)
	GetQueryLogTopBlocklists(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopBlocklist, error)
	DeleteQueryLogs(ctx context.Context, profileId string) error
	// ListQueryLogProfileIDs returns every profile id present in any retention collection.
	ListQueryLogProfileIDs(ctx context.Context) ([]string, error)
}
