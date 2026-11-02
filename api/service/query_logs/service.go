package querylogs

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
)

// ErrInvalidTopKind is returned for a kind other than blocked or resolved.
var ErrInvalidTopKind = errors.New("invalid top kind")

type QueryLogsService struct {
	QueryLogsRepository repository.QueryLogsRepository

	now func() time.Time
}

// Option configures optional QueryLogsService dependencies.
type Option func(*QueryLogsService)

// WithClock replaces the clock the retention trim measures the retention from.
func WithClock(now func() time.Time) Option {
	return func(q *QueryLogsService) { q.now = now }
}

func NewQueryLogsService(db repository.QueryLogsRepository, opts ...Option) *QueryLogsService {
	q := &QueryLogsService{
		QueryLogsRepository: db,
		now:                 time.Now,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// TrimToRetention deletes the profile's query logs older than the retention from the longer
// collections (J54, J13) and returns the number of rows deleted.
func (q *QueryLogsService) TrimToRetention(ctx context.Context, profileId string, retention model.Retention) (int64, error) {
	return q.QueryLogsRepository.TrimQueryLogs(ctx, profileId, retention, q.now())
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

// GetProfileQueryLogCoveredFrom returns when the profile's oldest log row inside timespan was written.
func (q *QueryLogsService) GetProfileQueryLogCoveredFrom(ctx context.Context, profileId string, retention model.Retention, timespan string) (*time.Time, error) {
	hours, err := model.NewTopTimespan(timespan)
	if err != nil {
		return nil, err
	}
	return q.QueryLogsRepository.GetQueryLogCoveredFrom(ctx, profileId, retention, hours)
}

func (q *QueryLogsService) DeleteProfileQueryLogs(ctx context.Context, profileId string) error {
	return q.QueryLogsRepository.DeleteQueryLogs(ctx, profileId)
}

// ListProfileIDs returns every profile id that has query logs in any retention collection.
func (q *QueryLogsService) ListProfileIDs(ctx context.Context) ([]string, error) {
	return q.QueryLogsRepository.ListQueryLogProfileIDs(ctx)
}
