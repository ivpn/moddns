package cron

import (
	"context"
	"time"

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

// NotifyInactiveSubscriptions re-arms the `notified_inactive` flag for subs no
// longer Inactive, then finds Inactive candidates via the coarse Mongo
// pre-filter, cuts off DNS (Redis profile cache delete) and sends the inactive
// notification email to those whose computed status is exactly StatusInactive.
// The model (sub.GetStatus()) is the single source of truth.
//
// Signup-reset RETIRED accounts are pending_delete (not inactive), so the
// status post-filter naturally excludes them — they are owned by the
// DeleteRetiredAccounts cron, which performed their DNS cutoff at retirement.
func NotifyInactiveSubscriptions(subRepo repository.SubscriptionRepository, accountRepo repository.AccountRepository, profileRepo repository.ProfileRepository, profileCache cache.Cache, mailer email.Mailer) {
	ctx := context.Background()

	// 1. Re-arm: clear notified_inactive=false for any sub no longer Inactive.
	if err := rearmInactiveNotified(ctx, subRepo); err != nil {
		log.Error().Err(err).Msg("Cron: failed to re-arm notified_inactive flag")
	}

	// 2. Find candidates (loose pre-filter; cron post-filters via GetStatus()).
	candidates, err := subRepo.FindInactiveUnnotified(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find inactive unnotified subscriptions")
		return
	}

	if len(candidates) == 0 {
		return
	}

	log.Info().Int("candidates", len(candidates)).Msg("Cron: evaluating inactive-subscription candidates")

	// 3. Strict post-filter via the model; cut off DNS; send email; record IDs to mark notified.
	notifiedIDs := make([]uuid.UUID, 0, len(candidates))
	skippedNotInactive := 0
	for _, sub := range candidates {
		if sub.GetStatus() != model.StatusInactive {
			// Excludes retired (pending_delete) and any sub that recovered.
			skippedNotInactive++
			continue
		}

		account, err := accountRepo.GetAccountById(ctx, sub.AccountID.Hex())
		if err != nil {
			log.Error().Err(err).Str("account_id", sub.AccountID.Hex()).Msg("Cron: failed to get account for inactive notification")
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

		if err := mailer.SendInactiveEmail(ctx, account.Email); err != nil {
			log.Error().Err(err).Str("email", account.Email).Msg("Cron: failed to send inactive-account email")
			// Leave sub unnotified so the retry happens next tick.
			continue
		}

		notifiedIDs = append(notifiedIDs, sub.ID)
	}

	log.Info().Int("candidates", len(candidates)).Int("skipped_not_inactive", skippedNotInactive).Int("sent", len(notifiedIDs)).Msg("Cron: inactive-subscription notifications complete")

	// 4. Mark only the actually-cut-off-and-emailed subs as notified.
	if len(notifiedIDs) > 0 {
		if err := subRepo.SetInactiveNotified(ctx, notifiedIDs, true); err != nil {
			log.Error().Err(err).Msg("Cron: failed to mark subscriptions as inactive-notified")
		}
	}
}

// DeleteRetiredAccounts hard-deletes accounts retired by the signup-reset flow
// once their grace window has elapsed. It finds subscriptions whose
// deletion_scheduled_at is older than model.RetiredAccountRetention and purges
// each account via the (idempotent) purge path. See
// docs/specs/signup-reset-behaviour.md Phase 3.
func DeleteRetiredAccounts(subRepo repository.SubscriptionRepository, purger AccountPurger) {
	ctx := context.Background()

	cutoff := time.Now().Add(-model.RetiredAccountRetention)
	subs, err := subRepo.FindScheduledForDeletion(ctx, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to find subscriptions scheduled for deletion")
		return
	}
	if len(subs) == 0 {
		return
	}

	log.Info().Int("candidates", len(subs)).Msg("Cron: hard-deleting retired accounts")

	deleted := 0
	for _, sub := range subs {
		accountID := sub.AccountID.Hex()
		if err := purger.PurgeAccountData(ctx, accountID); err != nil {
			log.Error().Err(err).Str("account_id", accountID).Msg("Cron: failed to purge retired account")
			continue
		}
		deleted++
	}

	log.Info().Int("candidates", len(subs)).Int("deleted", deleted).Msg("Cron: retired-account deletion complete")
}

// ReportDuplicateTokenHashAccounts is a READ-ONLY diagnostic: it scans for
// token_hash values held by more than one non-retired subscription (the
// signup-reset invariant violated) and logs a report. It writes nothing — it
// only surfaces pre-existing duplicates and any created by a failed/raced
// retirement (R-E4/R-E7) so an operator can remediate. The headline field
// `duplicate_groups` is the value to alert on (expected: 0). See
// docs/specs/signup-reset-behaviour.md.
func ReportDuplicateTokenHashAccounts(subRepo repository.SubscriptionRepository) {
	ctx := context.Background()

	groups, err := subRepo.FindDuplicateTokenHashGroups(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Cron: failed to scan for duplicate token_hash accounts")
		return
	}

	if len(groups) == 0 {
		log.Info().Int("duplicate_groups", 0).Msg("Cron: signup-reset duplicate report — invariant holds (no duplicates)")
		return
	}

	excess := 0
	for _, g := range groups {
		ids := make([]string, 0, len(g.AccountIDs))
		for _, id := range g.AccountIDs {
			ids = append(ids, id.Hex())
		}
		excess += g.Count - 1
		log.Warn().
			Str("event", "signup_reset_duplicate_token_hash").
			Str("token_hash", g.TokenHash).
			Int("account_count", g.Count).
			Strs("account_ids", ids).
			Msg("Cron: duplicate token_hash — multiple non-retired accounts for one IVPN customer")
	}

	log.Warn().
		Str("event", "signup_reset_duplicate_report").
		Int("duplicate_groups", len(groups)).
		Int("excess_accounts", excess).
		Msg("Cron: signup-reset duplicate report — manual remediation may be required")
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

// rearmInactiveNotified loads every sub with notified_inactive=true and clears
// the flag for any sub that has returned to a genuinely Active state. Same
// one-shot contract as rearmLANotified: a Tier 1 sub or a sub in
// LA-after-Inactive partial recovery keeps the flag set, so the cron does not
// re-fire the cutoff/email until the user fully recovers.
func rearmInactiveNotified(ctx context.Context, subRepo repository.SubscriptionRepository) error {
	notified, err := subRepo.FindWithInactiveNotified(ctx)
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
	return subRepo.SetInactiveNotified(ctx, toReset, false)
}
