package statistics

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/rs/zerolog/log"
)

const (
	// Bucket width of the proxy's statistics writer.
	bucketWidth = 15 * time.Minute
	// Proxy statistics flush interval; a closed bucket lands within this long.
	proxyFlushInterval = 30 * time.Second
	// Slack for collector lag beyond the settings TTL and flush interval.
	purgeLagMargin = 2 * time.Minute

	sweepBatch      = 100
	purgeJobTimeout = 90 * time.Second
)

// PurgeKind names what queued a purge.
type PurgeKind string

const (
	// PurgeKindDisable is bounded: it deletes buckets starting before its cutoff.
	PurgeKindDisable PurgeKind = "disable"
	// PurgeKindDelete is unbounded: the profile no longer exists.
	PurgeKindDelete PurgeKind = "delete"
)

// PurgeJob deletes a profile's statistics. A nil Before is unbounded.
type PurgeJob struct {
	Kind      PurgeKind
	ProfileID string
	Before    *time.Time
}

// Member encodes the job as its queue member. The cutoff is part of the member,
// so jobs with different bounds never collide and re-queueing the same job is a
// no-op.
func (j PurgeJob) Member() string {
	if j.Kind == PurgeKindDisable && j.Before != nil {
		return fmt.Sprintf("%s:%s:%d", j.Kind, j.ProfileID, j.Before.Unix())
	}
	return string(j.Kind) + ":" + j.ProfileID
}

// ParsePurgeMember decodes a queue member; ok is false for anything malformed.
func ParsePurgeMember(member string) (PurgeJob, bool) {
	kind, rest, found := strings.Cut(member, ":")
	if !found || rest == "" {
		return PurgeJob{}, false
	}
	switch PurgeKind(kind) {
	case PurgeKindDelete:
		if strings.Contains(rest, ":") {
			return PurgeJob{}, false
		}
		return PurgeJob{Kind: PurgeKindDelete, ProfileID: rest}, true
	case PurgeKindDisable:
		i := strings.LastIndex(rest, ":")
		if i <= 0 {
			return PurgeJob{}, false
		}
		secs, err := strconv.ParseInt(rest[i+1:], 10, 64)
		if err != nil || secs < 0 {
			return PurgeJob{}, false
		}
		before := time.Unix(secs, 0).UTC()
		return PurgeJob{Kind: PurgeKindDisable, ProfileID: rest[:i], Before: &before}, true
	}
	return PurgeJob{}, false
}

// PurgeCutoff returns the end of the 15-minute UTC bucket containing now.
func PurgeCutoff(now time.Time) time.Time {
	return now.UTC().Truncate(bucketWidth).Add(bucketWidth)
}

// PurgeSchedule returns the cutoff and due time of the delayed pass for a
// purge decided at now. The proxy may keep counting into a bucket until its
// cached settings expire (proxyTTL) plus the collector lag margin, so the cutoff
// is the end of the bucket containing that instant; the pass is due once the
// last bucket below the cutoff has closed and been flushed. Both depend only on
// the 15-minute window of now, which is what makes re-queueing idempotent.
func PurgeSchedule(now time.Time, proxyTTL time.Duration) (before, due time.Time) {
	before = PurgeCutoff(now.Add(proxyTTL + purgeLagMargin))
	due = before.Add(proxyFlushInterval + purgeLagMargin)
	return before, due
}

// SweepResult counts what one Sweep pass did; it carries no identifiers.
type SweepResult struct {
	Done, Failed, Dropped int
}

// PurgeSweeper runs due purge jobs. Deletes are idempotent and the cron job
// already single-flights across instances, so no per-profile lock is taken.
type PurgeSweeper struct {
	queue      cache.StatisticsPurgeQueue
	repo       repository.StatisticsRepository
	jobTimeout time.Duration
}

func NewPurgeSweeper(queue cache.StatisticsPurgeQueue, repo repository.StatisticsRepository) *PurgeSweeper {
	return &PurgeSweeper{queue: queue, repo: repo, jobTimeout: purgeJobTimeout}
}

// SetJobTimeout overrides the per-job delete bound (default 90s).
func (s *PurgeSweeper) SetJobTimeout(d time.Duration) { s.jobTimeout = d }

// Sweep processes the jobs due at now. A failed job stays queued for the next tick.
func (s *PurgeSweeper) Sweep(ctx context.Context, now time.Time) (SweepResult, error) {
	var res SweepResult
	entries, err := s.queue.DueStatisticsPurges(ctx, now, sweepBatch)
	if err != nil {
		return res, fmt.Errorf("statistics purge: read queue: %w", err)
	}

	for _, e := range entries {
		job, ok := ParsePurgeMember(e.Member)
		if !ok {
			if err := s.queue.RemoveStatisticsPurge(ctx, e.Member); err != nil {
				res.Failed++
			} else {
				res.Dropped++
			}
			continue
		}

		jobCtx, cancel := context.WithTimeout(ctx, s.jobTimeout)
		delErr := s.repo.DeleteProfileStatistics(jobCtx, job.ProfileID, job.Before)
		cancel()
		if delErr != nil {
			res.Failed++
			continue
		}
		if err := s.queue.RemoveStatisticsPurge(ctx, e.Member); err != nil {
			res.Failed++
			continue
		}
		res.Done++
	}

	if res.Failed > 0 {
		log.Ctx(ctx).Warn().Int("failed", res.Failed).Msg("statistics purge: some jobs failed and stay queued")
	}
	return res, nil
}
