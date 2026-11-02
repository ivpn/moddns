package repository

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/model"
)

type QueryLogsRepository interface {
	GetQueryLogs(ctx context.Context, profileId string, retention model.Retention, status string, timespan int, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error)
	GetQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error)
	GetQueryLogTopDomains(ctx context.Context, profileId string, retention model.Retention, status string, timespanHours, limit int) ([]model.QueryLogTopDomain, error)
	GetQueryLogTopClients(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopClient, error)
	GetQueryLogTopBlocklists(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopBlocklist, error)
	// GetQueryLogCoveredFrom returns the timestamp of the profile's oldest row inside the window, or nil.
	GetQueryLogCoveredFrom(ctx context.Context, profileId string, retention model.Retention, timespanHours int) (*time.Time, error)
	DeleteQueryLogs(ctx context.Context, profileId string) error
	// TrimQueryLogs deletes the profile's rows older than now - retention from every collection
	// longer than retention and returns the number deleted.
	TrimQueryLogs(ctx context.Context, profileId string, retention model.Retention, now time.Time) (int64, error)
	// ListQueryLogProfileIDs returns every profile id present in any retention collection.
	ListQueryLogProfileIDs(ctx context.Context) ([]string, error)
}
