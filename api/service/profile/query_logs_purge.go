package profile

import (
	"context"
	"errors"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

const (
	logsPurgeBatch      = 1000
	logsPurgeJobTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	logsPurgeDeadline = 5 * time.Minute
)

// QueryLogsPurgeResult counts what one purge run did; it carries no identifiers. Purged counts
// profiles whose logs were all deleted; Deleted counts rows older than the retention of profiles
// that keep logs; Total is the number of profiles found, so Checked < Total means the run ended
// before reaching all of them.
type QueryLogsPurgeResult struct {
	Checked, Total, Purged, Deleted, Failed int
}

// SetQueryLogsPurgeTimeouts overrides the per-profile delete bound and the whole-run bound
// of PurgeQueryLogs.
func (p *ProfileService) SetQueryLogsPurgeTimeouts(perProfile, wholeRun time.Duration) {
	p.logsPurgeJobTime, p.logsPurgeRunTime = perProfile, wholeRun
}

// trimQueryLogsBestEffort deletes the profile's logs older than a lowered retention, then retires
// the caches computed from them (J54). A failure is only logged: reads never go back further than
// the retention, and the next query-logs purge removes whatever is left.
func (p *ProfileService) trimQueryLogsBestEffort(ctx context.Context, profileId string, retention model.Retention) {
	if _, err := p.QueryLogsService.TrimToRetention(ctx, profileId, retention); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("query logs: retention trim failed; the next query-logs purge removes the leftovers")
	}
	p.invalidateQueryLogCaches(ctx, profileId)
}

// purgeQueryLogsBestEffort deletes all of the profile's query logs; leftovers of a failure
// are removed by the next purge run (api-endpoint-behaviour.md J12).
func (p *ProfileService) purgeQueryLogsBestEffort(ctx context.Context, profileId string) {
	if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("query logs: immediate purge failed; the next query-logs purge removes the leftovers")
	}
	p.invalidateQueryLogCaches(ctx, profileId)
}

// PurgeQueryLogs makes the stored query logs conform to each profile's settings as stored now
// (api-endpoint-behaviour.md J13, J14): a profile that no longer exists or has logging off loses
// all of its logs; one with logging on loses the rows older than its retention. A profile's
// caches are retired only when its rows changed. The first per-profile timeout ends the run (the
// database is struggling) and so does the whole-run deadline; profiles not reached are picked up
// by the next run.
func (p *ProfileService) PurgeQueryLogs(ctx context.Context) (QueryLogsPurgeResult, error) {
	var res QueryLogsPurgeResult
	jobTime, runTime := p.logsPurgeJobTime, p.logsPurgeRunTime
	if jobTime <= 0 {
		jobTime = logsPurgeJobTimeout
	}
	if runTime <= 0 {
		runTime = logsPurgeDeadline
	}

	runCtx, cancel := context.WithTimeout(ctx, runTime)
	defer cancel()

	ids, err := p.QueryLogsService.ListProfileIDs(runCtx)
	if err != nil {
		return res, err
	}
	res.Total = len(ids)

	for start := 0; start < len(ids); start += logsPurgeBatch {
		batch := ids[start:min(start+logsPurgeBatch, len(ids))]
		settings, err := p.ProfileRepository.GetProfilesLogsSettings(runCtx, batch)
		if err != nil {
			if runCtx.Err() != nil {
				return res, nil
			}
			return res, err
		}

		for _, id := range batch {
			if runCtx.Err() != nil {
				return res, nil
			}
			res.Checked++

			jobCtx, cancelJob := context.WithTimeout(runCtx, jobTime)
			changed, jobErr := p.purgeProfileQueryLogs(jobCtx, id, settings[id], &res)
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

// purgeProfileQueryLogs applies J13 to one profile and reports whether its rows may have changed.
func (p *ProfileService) purgeProfileQueryLogs(ctx context.Context, profileId string, logs *model.LogsSettings, res *QueryLogsPurgeResult) (bool, error) {
	if logs == nil || !logs.Enabled {
		if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
			return true, err
		}
		res.Purged++
		return true, nil
	}
	deleted, err := p.QueryLogsService.TrimToRetention(ctx, profileId, logs.Retention)
	res.Deleted += int(deleted)
	return deleted > 0, err
}
