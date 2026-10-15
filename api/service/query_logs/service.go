package querylogs

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/libs/dislock"
	"github.com/rs/zerolog/log"
)

// ErrInvalidTopKind is returned for a kind other than blocked or resolved.
var ErrInvalidTopKind = errors.New("invalid top kind")

const (
	// RetentionMoveLockPrefix namespaces the per-profile retention-move locks in Redis.
	RetentionMoveLockPrefix = "query-logs:retention-move:"
	// RetentionMoveDeadline bounds the move a PATCH starts.
	RetentionMoveDeadline = 10 * time.Minute
	// RetentionMoveLockTTL outlives the longest move (RetentionMoveDeadline, the reconcile's per-profile
	// bound) plus the undo of its last batch, so no second mover can take the lock mid-batch.
	RetentionMoveLockTTL = 11 * time.Minute
)

type QueryLogsService struct {
	QueryLogsRepository repository.QueryLogsRepository

	moveLocker *dislock.Locker
	background func(func())
	now        func() time.Time
}

// Option configures optional QueryLogsService dependencies.
type Option func(*QueryLogsService)

// WithBackground replaces how ConformBestEffort starts its background move (a goroutine by default).
func WithBackground(run func(func())) Option {
	return func(q *QueryLogsService) { q.background = run }
}

// WithClock replaces the clock the retention move measures the retention from.
func WithClock(now func() time.Time) Option {
	return func(q *QueryLogsService) { q.now = now }
}

func NewQueryLogsService(db repository.QueryLogsRepository, opts ...Option) *QueryLogsService {
	q := &QueryLogsService{
		QueryLogsRepository: db,
		background:          func(f func()) { go f() },
		now:                 time.Now,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// SetRetentionMoveLocker serialises retention moves of one profile across instances
// (api-endpoint-behaviour.md J54). Without it moves run unlocked.
func (q *QueryLogsService) SetRetentionMoveLocker(l *dislock.Locker) { q.moveLocker = l }

// ConformToRetention leaves the profile's rows only in collections no longer than retention and no
// older than it (J54, J13). A move already running on another instance is skipped. It returns the
// rows moved and deleted.
func (q *QueryLogsService) ConformToRetention(ctx context.Context, profileId string, retention model.Retention) (int, int64, error) {
	if q.moveLocker != nil {
		lock, err := q.moveLocker.TryLock(ctx, profileId, RetentionMoveLockTTL)
		if errors.Is(err, dislock.ErrNotAcquired) {
			return 0, 0, nil
		}
		if err != nil {
			return 0, 0, err
		}
		defer func() { _ = lock.Unlock(context.WithoutCancel(ctx)) }()
	}
	return q.QueryLogsRepository.ConformQueryLogs(ctx, profileId, retention, q.now().UTC())
}

// ConformBestEffort starts the move on lowering the retention in the background, so the PATCH does
// not wait for it, then calls done; the next query-logs reconcile repeats it for anything left.
func (q *QueryLogsService) ConformBestEffort(ctx context.Context, profileId string, retention model.Retention, done func(context.Context)) {
	moveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), RetentionMoveDeadline)
	q.background(func() {
		defer cancel()
		if _, _, err := q.ConformToRetention(moveCtx, profileId, retention); err != nil {
			log.Ctx(moveCtx).Error().Err(err).Msg("query logs: retention move failed; the next query-logs reconcile retries it")
		}
		// The move may have ended on its deadline; the caller's follow-up must still run.
		done(context.WithoutCancel(moveCtx))
	})
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
