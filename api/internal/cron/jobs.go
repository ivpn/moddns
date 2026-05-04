package cron

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/rs/zerolog/log"
)

// NotifyExpiringSubscriptions resets the notified flag for active subscriptions,
// finds expired+unnotified ones, sends notification emails, and marks them as notified.
func NotifyExpiringSubscriptions(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, mailer email.Mailer) {
	ctx := context.Background()

	// 1. Reset notified for active subscriptions
	if err := subRepo.ResetNotifiedForActive(ctx); err != nil {
		log.Error().Err(err).Msg("Cron: failed to reset notified flag for active subscriptions")
	}

	// 2. Find expired+unnotified subscriptions
	subs, err := subRepo.FindExpiredUnnotified(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find expired unnotified subscriptions")
		return
	}

	if len(subs) == 0 {
		return
	}

	log.Info().Int("count", len(subs)).Msg("Cron: notifying expiring subscriptions")

	// 3. Send notification emails
	for _, sub := range subs {
		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for expiry notification")
			continue
		}

		if err := mailer.SendSubscriptionExpiryEmail(ctx, account.Email); err != nil {
			log.Error().Err(err).Str("email", account.Email).Msg("Cron: failed to send subscription expiry email")
			continue
		}
	}

	// 4. Mark as notified
	ids := make([]uuid.UUID, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}
	if err := subRepo.MarkNotified(ctx, ids); err != nil {
		log.Error().Err(err).Msg("Cron: failed to mark subscriptions as notified")
	}
}

// NotifyPendingDeleteSubscriptions resets the notified_pending_delete flag for active subscriptions,
// finds pending-delete+unnotified ones, deletes their Redis profile settings (DNS cutoff),
// sends notification emails, and marks them as notified.
func NotifyPendingDeleteSubscriptions(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, profileRepo repository.ProfileRepository, profileCache cache.Cache, mailer email.Mailer) {
	ctx := context.Background()

	// 1. Reset notified_pending_delete for active subscriptions
	if err := subRepo.ResetPendingDeleteNotifiedForActive(ctx); err != nil {
		log.Error().Err(err).Msg("Cron: failed to reset notified_pending_delete flag for active subscriptions")
	}

	// 2. Find pending-delete+unnotified subscriptions
	subs, err := subRepo.FindPendingDeleteUnnotified(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find pending-delete unnotified subscriptions")
		return
	}

	if len(subs) == 0 {
		return
	}

	log.Info().Int("count", len(subs)).Msg("Cron: notifying pending-delete subscriptions")

	// 3. DNS cutoff: delete Redis profile settings for each subscription's profiles
	for _, sub := range subs {
		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for DNS cutoff")
			continue
		}

		profiles, err := profileRepo.GetProfilesByAccountId(ctx, account.ID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get profiles for DNS cutoff")
			continue
		}

		for _, profile := range profiles {
			if err := profileCache.DeleteProfileSettings(ctx, profile.ProfileId); err != nil {
				log.Error().Err(err).Str("profile_id", profile.ProfileId).Msg("Cron: failed to delete profile settings from cache (DNS cutoff)")
			}
		}
	}

	// 4. Send notification emails
	for _, sub := range subs {
		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for pending-delete notification")
			continue
		}

		if err := mailer.SendPendingDeleteEmail(ctx, account.Email); err != nil {
			log.Error().Err(err).Str("email", account.Email).Msg("Cron: failed to send pending-delete email")
			continue
		}
	}

	// 5. Mark as notified
	ids := make([]uuid.UUID, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}
	if err := subRepo.MarkPendingDeleteNotified(ctx, ids); err != nil {
		log.Error().Err(err).Msg("Cron: failed to mark subscriptions as pending-delete notified")
	}
}
