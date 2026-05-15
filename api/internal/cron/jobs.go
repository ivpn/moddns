package cron

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/email"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// NotifyExpiringSubscriptions re-arms the `notified` flag for subs no longer
// in LimitedAccess, then finds LA candidates via the coarse Mongo pre-filter
// and sends the expiry email to those whose computed status is exactly
// StatusLimitedAccess. The model (sub.GetStatus()) is the single source of
// truth for the LA predicate; Mongo queries are correctness obligations only
// ("must not exclude any true candidate").
func NotifyExpiringSubscriptions(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, mailer email.Mailer) {
	ctx := context.Background()

	// 1. Re-arm: clear notified=false for any sub no longer in LimitedAccess.
	if err := rearmLANotified(ctx, subRepo); err != nil {
		log.Error().Err(err).Msg("Cron: failed to re-arm notified flag")
	}

	// 2. Find candidates (loose pre-filter; cron post-filters via GetStatus()).
	candidates, err := subRepo.FindExpiredUnnotified(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find expired unnotified subscriptions")
		return
	}

	if len(candidates) == 0 {
		return
	}

	log.Info().Int("candidates", len(candidates)).Msg("Cron: evaluating expiring-subscription candidates")

	// 3. Strict post-filter via the model; send the email; record IDs to mark notified.
	notifiedIDs := make([]uuid.UUID, 0, len(candidates))
	skippedNotLA := 0
	for _, sub := range candidates {
		if sub.GetStatus() != model.StatusLimitedAccess {
			skippedNotLA++
			log.Debug().Str("subscription_id", sub.ID.String()).Str("status", string(sub.GetStatus())).Msg("Cron: skipping non-LA candidate from expiry notification")
			continue
		}

		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for expiry notification")
			continue
		}

		if err := mailer.SendSubscriptionExpiryEmail(ctx, account.Email); err != nil {
			log.Error().Err(err).Str("email", account.Email).Msg("Cron: failed to send subscription expiry email")
			continue
		}

		notifiedIDs = append(notifiedIDs, sub.ID)
	}

	log.Info().Int("candidates", len(candidates)).Int("skipped_not_la", skippedNotLA).Int("sent", len(notifiedIDs)).Msg("Cron: expiring-subscription notifications complete")

	// 4. Mark only the actually-emailed subs as notified.
	if len(notifiedIDs) > 0 {
		if err := subRepo.SetNotified(ctx, notifiedIDs, true); err != nil {
			log.Error().Err(err).Msg("Cron: failed to mark subscriptions as notified")
		}
	}
}

// NotifyPendingDeleteSubscriptions re-arms the `notified_pending_delete` flag
// for subs no longer in PendingDelete, then finds PD candidates via the coarse
// Mongo pre-filter, cuts off DNS (Redis profile cache delete) and sends the
// pending-deletion email to those whose computed status is exactly
// StatusPendingDelete. The model (sub.GetStatus()) is the single source of
// truth for the PD predicate.
func NotifyPendingDeleteSubscriptions(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, profileRepo repository.ProfileRepository, profileCache cache.Cache, mailer email.Mailer) {
	ctx := context.Background()

	// 1. Re-arm: clear notified_pending_delete=false for any sub no longer in PD.
	if err := rearmPendingDeleteNotified(ctx, subRepo); err != nil {
		log.Error().Err(err).Msg("Cron: failed to re-arm notified_pending_delete flag")
	}

	// 2. Find candidates (loose pre-filter; cron post-filters via GetStatus()).
	candidates, err := subRepo.FindPendingDeleteUnnotified(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find pending-delete unnotified subscriptions")
		return
	}

	if len(candidates) == 0 {
		return
	}

	log.Info().Int("candidates", len(candidates)).Msg("Cron: evaluating pending-delete candidates")

	// 3. Strict post-filter via the model; cut off DNS; send email; record IDs to mark notified.
	notifiedIDs := make([]uuid.UUID, 0, len(candidates))
	skippedNotPD := 0
	for _, sub := range candidates {
		if sub.GetStatus() != model.StatusPendingDelete {
			skippedNotPD++
			log.Debug().Str("subscription_id", sub.ID.String()).Str("status", string(sub.GetStatus())).Msg("Cron: skipping non-PD candidate from pending-delete notification")
			continue
		}

		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for pending-delete")
			continue
		}

		// DNS cutoff: delete Redis profile settings for every profile of the account.
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

		if err := mailer.SendPendingDeleteEmail(ctx, account.Email); err != nil {
			log.Error().Err(err).Str("email", account.Email).Msg("Cron: failed to send pending-delete email")
			// Leave sub unnotified so the retry happens next tick.
			continue
		}

		notifiedIDs = append(notifiedIDs, sub.ID)
	}

	log.Info().Int("candidates", len(candidates)).Int("skipped_not_pd", skippedNotPD).Int("sent", len(notifiedIDs)).Msg("Cron: pending-delete notifications complete")

	// 4. Mark only the actually-cut-off-and-emailed subs as notified.
	if len(notifiedIDs) > 0 {
		if err := subRepo.SetPendingDeleteNotified(ctx, notifiedIDs, true); err != nil {
			log.Error().Err(err).Msg("Cron: failed to mark subscriptions as pending-delete notified")
		}
	}
}

// rearmLANotified loads every sub with notified=true and clears the flag
// for any sub that has returned to a genuinely Active state. Per the
// one-shot contract (docs/specs/subscription-lifecycle-enforcement.md
// "Idempotency and one-shot semantics"), the flag must NOT be cleared on
// transitions to Grace, LA-after-PD, or any non-Active state — only true
// recovery via resync re-arms the next email.
func rearmLANotified(ctx context.Context, subRepo repository.SubscriptionRepository) error {
	notified, err := subRepo.FindWithLANotified(ctx)
	if err != nil {
		return err
	}
	toReset := make([]uuid.UUID, 0, len(notified))
	for _, sub := range notified {
		if sub.GetStatus() == model.StatusActive {
			toReset = append(toReset, sub.ID)
		}
	}
	if len(toReset) == 0 {
		return nil
	}
	return subRepo.SetNotified(ctx, toReset, false)
}

// rearmPendingDeleteNotified loads every sub with notified_pending_delete=true
// and clears the flag for any sub that has returned to a genuinely Active
// state. Same one-shot contract as rearmLANotified: a Tier 1 sub or a sub in
// LA-after-PD partial recovery keeps the flag set, so the cron does not
// re-fire the cutoff/email until the user fully recovers.
func rearmPendingDeleteNotified(ctx context.Context, subRepo repository.SubscriptionRepository) error {
	notified, err := subRepo.FindWithPendingDeleteNotified(ctx)
	if err != nil {
		return err
	}
	toReset := make([]uuid.UUID, 0, len(notified))
	for _, sub := range notified {
		if sub.GetStatus() == model.StatusActive {
			toReset = append(toReset, sub.ID)
		}
	}
	if len(toReset) == 0 {
		return nil
	}
	return subRepo.SetPendingDeleteNotified(ctx, toReset, false)
}
