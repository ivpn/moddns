package profile

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	logsPurgeBatch      = 1000
	logsPurgeJobTimeout = 90 * time.Second
	// A run must finish well inside the cron lock TTL (10 min).
	logsPurgeDeadline = 5 * time.Minute
)

// UnconsentedQueryLogsPurgeResult counts what one sweep did; it carries no identifiers.
type UnconsentedQueryLogsPurgeResult struct {
	Checked, Purged, Failed int
}

// SetQueryLogsPurgeTimeouts overrides the per-profile delete bound and the whole-run bound
// of PurgeUnconsentedQueryLogs.
func (p *ProfileService) SetQueryLogsPurgeTimeouts(perProfile, wholeRun time.Duration) {
	p.logsPurgeJobTime, p.logsPurgeRunTime = perProfile, wholeRun
}

// purgeQueryLogsBestEffort deletes all of the profile's query logs; leftovers of a failure
// are removed by the next sweep (api-endpoint-behaviour.md J12).
func (p *ProfileService) purgeQueryLogsBestEffort(ctx context.Context, profileId string) {
	if err := p.QueryLogsService.DeleteProfileQueryLogs(ctx, profileId); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("query logs: immediate purge failed; the next unconsented query-logs sweep removes the leftovers")
	}
	p.invalidateQueryLogCaches(ctx, profileId)
}

// PurgeUnconsentedQueryLogs deletes the query logs of every profile that no longer exists or
// has logging off, as stored now (api-endpoint-behaviour.md J13, J14).
func (p *ProfileService) PurgeUnconsentedQueryLogs(ctx context.Context) (UnconsentedQueryLogsPurgeResult, error) {
	var res UnconsentedQueryLogsPurgeResult
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

	for start := 0; start < len(ids); start += logsPurgeBatch {
		batch := ids[start:min(start+logsPurgeBatch, len(ids))]
		enabled, err := p.ProfileRepository.GetProfilesLogsEnabled(runCtx, batch)
		if err != nil {
			return res, err
		}

		for _, id := range batch {
			if runCtx.Err() != nil {
				return res, nil
			}
			res.Checked++
			if enabled[id] {
				continue
			}

			jobCtx, cancelJob := context.WithTimeout(runCtx, jobTime)
			delErr := p.QueryLogsService.DeleteProfileQueryLogs(jobCtx, id)
			timedOut := errors.Is(jobCtx.Err(), context.DeadlineExceeded)
			cancelJob()
			p.invalidateQueryLogCaches(runCtx, id)
			if delErr != nil {
				res.Failed++
				if timedOut {
					return res, nil
				}
				continue
			}
			res.Purged++
		}
	}
	return res, nil
}
