package querylogs

import (
	"context"
	"errors"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
)

// ErrInvalidTopKind is returned for a kind other than blocked or resolved.
var ErrInvalidTopKind = errors.New("invalid top kind")

type QueryLogsService struct {
	QueryLogsRepository repository.QueryLogsRepository
}

func NewQueryLogsService(db repository.QueryLogsRepository) *QueryLogsService {
	return &QueryLogsService{
		QueryLogsRepository: db,
	}
}

func (q *QueryLogsService) GetProfileQueryLogs(ctx context.Context, profileId string, retention model.Retention, status, timespan, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error) {
	timespanHours, err := model.NewTimespan(timespan)
	if err != nil {
		return nil, err
	}

	logs, err := q.QueryLogsRepository.GetQueryLogs(ctx, profileId, retention, status, timespanHours, deviceId, search, sortBy, page, limit)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (q *QueryLogsService) DownloadProfileQueryLogs(ctx context.Context, profileId string, retention model.Retention, page, limit int) ([]model.QueryLog, error) {
	logs, err := q.QueryLogsRepository.GetQueryLogs(ctx, profileId, retention, model.QueryLogStatusAll, 0, "", "", "created", page, limit)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (q *QueryLogsService) GetProfileQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error) {
	return q.QueryLogsRepository.GetQueryLogDevices(ctx, profileId, retention)
}

// GetProfileQueryLogTopDomains returns the most frequent domains for kind
// (model.QueryLogTopKindBlocked or QueryLogTopKindResolved) inside timespan.
func (q *QueryLogsService) GetProfileQueryLogTopDomains(ctx context.Context, profileId string, retention model.Retention, timespan, kind string, limit int) ([]model.QueryLogTopDomain, error) {
	hours, err := model.NewTopTimespan(timespan)
	if err != nil {
		return nil, err
	}
	var status string
	switch kind {
	case model.QueryLogTopKindBlocked:
		status = model.QueryLogStatusBlocked
	case model.QueryLogTopKindResolved:
		status = model.QueryLogStatusProcessed
	default:
		return nil, ErrInvalidTopKind
	}
	return q.QueryLogsRepository.GetQueryLogTopDomains(ctx, profileId, retention, status, hours, limit)
}

// GetProfileQueryLogTopClients returns the most frequent client IPs inside timespan.
func (q *QueryLogsService) GetProfileQueryLogTopClients(ctx context.Context, profileId string, retention model.Retention, timespan string, limit int) ([]model.QueryLogTopClient, error) {
	hours, err := model.NewTopTimespan(timespan)
	if err != nil {
		return nil, err
	}
	return q.QueryLogsRepository.GetQueryLogTopClients(ctx, profileId, retention, hours, limit)
}

// GetProfileQueryLogTopBlocklists returns the blocklists that blocked the most queries inside timespan.
func (q *QueryLogsService) GetProfileQueryLogTopBlocklists(ctx context.Context, profileId string, retention model.Retention, timespan string, limit int) ([]model.QueryLogTopBlocklist, error) {
	hours, err := model.NewTopTimespan(timespan)
	if err != nil {
		return nil, err
	}
	return q.QueryLogsRepository.GetQueryLogTopBlocklists(ctx, profileId, retention, hours, limit)
}

func (q *QueryLogsService) DeleteProfileQueryLogs(ctx context.Context, profileId string) error {
	return q.QueryLogsRepository.DeleteQueryLogs(ctx, profileId)
}

// ListProfileIDs returns every profile id that has query logs in any retention collection.
func (q *QueryLogsService) ListProfileIDs(ctx context.Context) ([]string, error) {
	return q.QueryLogsRepository.ListQueryLogProfileIDs(ctx)
}
