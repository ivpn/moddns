package profile

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

const (
	logsReconcileBatch      = 1000
	logsReconcileJobTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	logsReconcileDeadline = 5 * time.Minute
)

// QueryLogsReconcileResult counts what one run did; it carries no identifiers. Purged counts
// profiles whose logs were all deleted; Deleted and Moved count rows of profiles that keep logs.
type QueryLogsReconcileResult struct {
	Checked, Purged, Deleted, Moved, Failed int
}

// SetQueryLogsReconcileTimeouts overrides the per-profile bound and the whole-run bound of
// ReconcileQueryLogs.
func (p *ProfileService) SetQueryLogsReconcileTimeouts(perProfile, wholeRun time.Duration) {
	p.logsReconcileJobTime, p.logsReconcileRunTime = perProfile, wholeRun
}

// purgeQueryLogsBestEffort deletes all of the profile's query logs; leftovers of a failure
// are removed by the next reconcile (api-endpoint-behaviour.md J12).
func (p *ProfileService) purgeQueryLogsBestEffort(ctx context.Context, profileId string) {
	if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("query logs: immediate purge failed; the next query-logs reconcile removes the leftovers")
	}
	p.invalidateQueryLogCaches(ctx, profileId)
}

// conformQueryLogsBestEffort moves the profile's logs to a lowered retention in the background,
// then retires the caches computed while the rows were moving (J54).
func (p *ProfileService) conformQueryLogsBestEffort(ctx context.Context, profileId string, retention model.Retention) {
	p.QueryLogsService.ConformBestEffort(ctx, profileId, retention, func(ctx context.Context) {
		p.invalidateQueryLogCaches(ctx, profileId)
	})
}

// ReconcileQueryLogs makes the stored query logs conform to each profile's current settings
// (api-endpoint-behaviour.md J13, J14): a profile that no longer exists or has logging off loses
// all of its logs; one with logging on keeps only what its retention covers, in the collections
// it reads. Everything is derived from Mongo, so a run needs no memory of earlier ones. A
// profile's caches are retired only when its stored rows changed. The first per-profile timeout
// ends the run (the database is struggling) and so does the whole-run deadline; profiles not
// reached are picked up by the next run.
func (p *ProfileService) ReconcileQueryLogs(ctx context.Context) (QueryLogsReconcileResult, error) {
	var res QueryLogsReconcileResult
	jobTime, runTime := p.logsReconcileJobTime, p.logsReconcileRunTime
	if jobTime <= 0 {
		jobTime = logsReconcileJobTimeout
	}
	if runTime <= 0 {
		runTime = logsReconcileDeadline
	}

	runCtx, cancel := context.WithTimeout(ctx, runTime)
	defer cancel()

	ids, err := p.QueryLogsService.ListProfileIDs(runCtx)
	if err != nil {
		return res, err
	}

	for start := 0; start < len(ids); start += logsReconcileBatch {
		batch := ids[start:min(start+logsReconcileBatch, len(ids))]
		settings, err := p.ProfileRepository.GetProfilesLogsSettings(runCtx, batch)
		if err != nil {
			return res, err
		}

		for _, id := range batch {
			if runCtx.Err() != nil {
				return res, nil
			}
			res.Checked++
			logs := settings[id]

			jobCtx, cancelJob := context.WithTimeout(runCtx, jobTime)
			changed, jobErr := p.reconcileProfileQueryLogs(jobCtx, id, logs, &res)
			timedOut := errors.Is(jobCtx.Err(), context.DeadlineExceeded)
			cancelJob()
			if changed {
				p.invalidateQueryLogCaches(runCtx, id)
			}
			if jobErr != nil {
				res.Failed++
				if timedOut {
					return res, nil
				}
			}
		}
	}
	return res, nil
}

// reconcileProfileQueryLogs applies J13 to one profile and reports whether its rows may have changed.
func (p *ProfileService) reconcileProfileQueryLogs(ctx context.Context, profileId string, logs *model.LogsSettings, res *QueryLogsReconcileResult) (bool, error) {
	if logs == nil || !logs.Enabled {
		if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
			return true, err
		}
		res.Purged++
		return true, nil
	}
	moved, deleted, err := p.QueryLogsService.ConformToRetention(ctx, profileId, logs.Retention)
	res.Moved += moved
	res.Deleted += int(deleted)
	return moved > 0 || deleted > 0, err
}
