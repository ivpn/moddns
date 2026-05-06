package cron

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/rs/zerolog/log"
)

// Start initializes the gocron scheduler with all periodic jobs.
//
// The locker enforces single-flight execution across load-balanced API
// instances: only the instance that acquires the per-job Redis lock for
// a given tick runs the job body; the others silently skip. The MongoDB
// notified flags remain the durable dedup safety net for the rare cases
// where the lock cannot serialise (e.g. Redis failover mid-tick).
func Start(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, profileRepo repository.ProfileRepository, profileCache cache.Cache, mailer email.Mailer, locker gocron.Locker) {
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
		gocron.NewTask(NotifyPendingDeleteSubscriptions, subRepo, accountRepo, profileRepo, profileCache, mailer),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule pending-delete notification job")
		return
	}

	s.Start()
	log.Info().Msg("Cron scheduler started")
}
