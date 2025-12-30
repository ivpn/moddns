package querylogs

import (
	"context"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
)

const (
	STATUS_ALL       = "all"
	STATUS_BLOCKED   = "blocked"
	STATUS_PROCESSED = "processed"
)

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
	logs, err := q.QueryLogsRepository.GetQueryLogs(ctx, profileId, retention, STATUS_ALL, 0, "", "", "created", page, limit)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (q *QueryLogsService) DeleteProfileQueryLogs(ctx context.Context, profileId string) error {
	return q.QueryLogsRepository.DeleteQueryLogs(ctx, profileId)
}
