package cron

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/api/service/statistics"
	"github.com/rs/zerolog/log"
)

// AccountPurger hard-deletes an account and all of its connected data without
// authorization checks. Implemented by service.Service (the method is promoted
// from AccountService.PurgeAccountData). Used by the DeleteRetiredAccounts job.
type AccountPurger interface {
	PurgeAccountData(ctx context.Context, accountId string) error
}

// UnconsentedStatisticsPurger removes statistics that no longer have consent.
type UnconsentedStatisticsPurger interface {
	PurgeUnconsentedStatistics(ctx context.Context) (statistics.UnconsentedPurgeResult, error)
}

// UnconsentedQueryLogsPurger removes query logs of profiles that have logging off or no longer exist.
type UnconsentedQueryLogsPurger interface {
	PurgeUnconsentedQueryLogs(ctx context.Context) (profile.UnconsentedQueryLogsPurgeResult, error)
}

// Start initializes the gocron scheduler with all periodic jobs.
//
// The locker enforces single-flight execution across load-balanced API
// instances: only the instance that acquires the per-job Redis lock for
// a given tick runs the job body; the others silently skip. The MongoDB
// notified flags remain the durable dedup safety net for the rare cases
// where the lock cannot serialise (e.g. Redis failover mid-tick).
func Start(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, profileRepo repository.ProfileRepository, profileCache cache.Cache, mailer email.Mailer, purger AccountPurger, statsPurger UnconsentedStatisticsPurger, purgeInterval time.Duration, logsPurger UnconsentedQueryLogsPurger, logsPurgeInterval time.Duration, locker gocron.Locker) {
	s, err := gocron.NewScheduler(gocron.WithDistributedLocker(locker))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create cron scheduler")
		return
	}

	_, err = s.NewJob(
		gocron.CronJob("0 * * * *", false), // every hour at minute 0
		gocron.NewTask(NotifyExpiringSubscriptions, subRepo, accountRepo, mailer),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule subscription expiry notification job")
		return
	}

	_, err = s.NewJob(
		gocron.CronJob("30 * * * *", false), // every hour at minute 30
		gocron.NewTask(NotifyInactiveSubscriptions, subRepo, accountRepo, profileRepo, profileCache, mailer),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule inactive-subscription notification job")
		return
	}

	_, err = s.NewJob(
		gocron.CronJob("15 * * * *", false), // every hour at minute 15 (offset from the :00 LA and :30 PD jobs)
		gocron.NewTask(DeleteRetiredAccounts, subRepo, purger),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule retired-account deletion job")
		return
	}

	_, err = s.NewJob(
		gocron.CronJob("45 3 * * *", false), // daily at 03:45 — read-only duplicate report
		gocron.NewTask(ReportDuplicateTokenHashAccounts, subRepo),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule duplicate token_hash report job")
		return
	}

	_, err = s.NewJob(
		gocron.DurationJob(purgeInterval),
		gocron.NewTask(PurgeUnconsentedStatistics, statsPurger),
		// A slow run must not overlap the next tick on this instance; the cron
		// lock covers other instances.
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule unconsented-statistics purge job")
		return
	}

	_, err = s.NewJob(
		gocron.DurationJob(logsPurgeInterval),
		gocron.NewTask(PurgeUnconsentedQueryLogs, logsPurger),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule unconsented query-logs purge job")
		return
	}

	s.Start()
	log.Info().Msg("Cron scheduler started")
}
