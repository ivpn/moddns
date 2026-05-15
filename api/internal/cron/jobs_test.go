package cron

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// freshDates returns now-anchored timestamps that make a paid sub Active and a Tier 1 sub PD via L3.
func freshDates() (activeUntil, updatedAt time.Time) {
	now := time.Now()
	return now.Add(30 * 24 * time.Hour), now
}

// staleDates returns timestamps that make any sub PD via the date predicates.
func staleDates() (activeUntil, updatedAt time.Time) {
	now := time.Now()
	return now.Add(-20 * 24 * time.Hour), now.Add(-20 * 24 * time.Hour)
}

// newSub builds a model.Subscription with a synthetic ID and the given tier and dates.
func newSub(tier string, activeUntil, updatedAt time.Time) model.Subscription {
	return model.Subscription{
		ID:          uuid.New(),
		AccountID:   primitive.NewObjectID(),
		Tier:        tier,
		ActiveUntil: activeUntil,
		UpdatedAt:   updatedAt,
	}
}

// TestNotifyPendingDelete_FreshTier1 covers the headline behaviour: a Tier 1
// sub with fresh dates must be picked up, have DNS cut off, get emailed, and
// be marked notified.
//
// specRef: subscription-lifecycle-enforcement.md E9
func TestNotifyPendingDelete_FreshTier1(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	au, ua := freshDates()
	sub := newSub("IVPN Tier 1", au, ua)
	account := &model.Account{
		ID:    sub.AccountID,
		Email: "victim@example.com",
	}
	profile := model.Profile{ProfileId: "prof-123"}

	// Re-arm step: no flagged subs to consider.
	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{}, nil).Once()

	// Find step: returns our Tier 1 sub.
	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{sub}, nil).Once()

	// Cron resolves account, profiles, deletes Redis cache, sends email, marks notified.
	accountRepo.On("GetAccountById", mock.Anything, sub.AccountID.Hex()).Return(account, nil).Once()
	profileRepo.On("GetProfilesByAccountId", mock.Anything, account.ID.Hex()).Return([]model.Profile{profile}, nil).Once()
	profileCache.On("DeleteProfileSettings", mock.Anything, "prof-123").Return(nil).Once()
	mailer.On("SendPendingDeleteEmail", mock.Anything, "victim@example.com").Return(nil).Once()
	subRepo.On("SetPendingDeleteNotified", mock.Anything, []uuid.UUID{sub.ID}, true).Return(nil).Once()

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyPendingDelete_PostFilterSkipsPaidActive proves the loose Mongo
// pre-filter is safe: if a paid Active sub leaks through (e.g. a future
// filter widening introduces false positives), the Go-side GetStatus()
// check rejects it. No DNS cutoff, no email, no flag write.
func TestNotifyPendingDelete_PostFilterSkipsPaidActive(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	au, ua := freshDates()
	leakedActive := newSub("IVPN Tier 2", au, ua)

	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{}, nil).Once()
	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{leakedActive}, nil).Once()

	// Deliberately register no expectations on accountRepo, profileRepo, cache,
	// mailer, or SetPendingDeleteNotified — if the cron calls any of them, the
	// mock will fail the test.

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyPendingDelete_ReArmClearsFlagWhenNoLongerPD covers the recovery
// path: a flagged sub that has transitioned back to Active (e.g., user
// upgraded from Tier 1 to Tier 2 via resync) must have its flag cleared.
func TestNotifyPendingDelete_ReArmClearsFlagWhenNoLongerPD(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	au, ua := freshDates()
	recovered := newSub("IVPN Tier 2", au, ua) // Now Active

	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{recovered}, nil).Once()
	// Re-arm fires: GetStatus() != PD → bulk-clear the flag for this sub's ID.
	subRepo.On("SetPendingDeleteNotified", mock.Anything, []uuid.UUID{recovered.ID}, false).Return(nil).Once()

	// Then find step finds no candidates.
	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{}, nil).Once()

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyPendingDelete_ReArmKeepsFlagWhenInLA verifies the one-shot
// contract for partial recoveries: a paid sub that went Active→PD and then
// partially resyncs into LA must keep its PD-notified flag set, so a later
// slip back into PD does NOT re-fire the email and cutoff.
//
// specRef: subscription-lifecycle-enforcement.md "Idempotency and one-shot semantics"
func TestNotifyPendingDelete_ReArmKeepsFlagWhenInLA(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	now := time.Now()
	// active_until 5 days ago, updated_at recent → LimitedAccess (L4).
	partial := newSub("IVPN Tier 2", now.Add(-5*24*time.Hour), now)
	require.Equal(t, model.StatusLimitedAccess, partial.GetStatus(), "sanity: setup must yield LimitedAccess")

	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{partial}, nil).Once()
	// No SetPendingDeleteNotified call expected — re-arm must NOT clear the flag.

	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{}, nil).Once()

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyPendingDelete_ReArmKeepsFlagWhenStillPD covers the idempotency
// case: a flagged Tier 1 sub stays flagged tick after tick (no re-email).
func TestNotifyPendingDelete_ReArmKeepsFlagWhenStillPD(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	au, ua := freshDates()
	stillPD := newSub("IVPN Tier 1", au, ua) // Still Tier 1 → still PD

	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{stillPD}, nil).Once()
	// No SetPendingDeleteNotified call — re-arm finds nothing to clear.

	// Find step excludes already-notified subs by definition; returns empty.
	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{}, nil).Once()

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyPendingDelete_EmailFailureLeavesUnnotified verifies the retry
// contract: if SendPendingDeleteEmail fails, the sub must not be marked
// notified so the cron retries on the next tick.
//
// specRef: subscription-lifecycle-enforcement.md E6 (analogous LA contract)
func TestNotifyPendingDelete_EmailFailureLeavesUnnotified(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	profileRepo := mocks.NewProfileRepository(t)
	profileCache := mocks.NewCachecache(t)
	mailer := mocks.NewMaileremail(t)

	au, ua := staleDates()
	sub := newSub("IVPN Tier 2", au, ua) // PD via stale dates
	account := &model.Account{ID: sub.AccountID, Email: "fail@example.com"}

	subRepo.On("FindWithPendingDeleteNotified", mock.Anything).Return([]model.Subscription{}, nil).Once()
	subRepo.On("FindPendingDeleteUnnotified", mock.Anything).Return([]model.Subscription{sub}, nil).Once()
	accountRepo.On("GetAccountById", mock.Anything, sub.AccountID.Hex()).Return(account, nil).Once()
	profileRepo.On("GetProfilesByAccountId", mock.Anything, account.ID.Hex()).Return([]model.Profile{}, nil).Once()
	mailer.On("SendPendingDeleteEmail", mock.Anything, "fail@example.com").Return(errors.New("smtp boom")).Once()

	// DNS cutoff still happened (no profiles to delete in this case), but
	// SetPendingDeleteNotified must NOT be called because the email failed.
	// We register no expectation on SetPendingDeleteNotified; if it fires,
	// the strict mock fails.

	NotifyPendingDeleteSubscriptions(subRepo, accountRepo, profileRepo, profileCache, mailer)
}

// TestNotifyExpiring_PostFilterSkipsPDAndGrace verifies the LA cron's
// strict GetStatus()==LA post-filter still rejects PD and GracePeriod subs
// that may leak through the loose Mongo pre-filter.
func TestNotifyExpiring_PostFilterSkipsPDAndGrace(t *testing.T) {
	subRepo := mocks.NewSubscriptionRepository(t)
	accountRepo := mocks.NewAccountRepository(t)
	mailer := mocks.NewMaileremail(t)

	// PD candidate that would match the loose Mongo pre-filter.
	pd := newSub("IVPN Tier 2", time.Now().Add(-20*24*time.Hour), time.Now().Add(-20*24*time.Hour))
	// Grace period candidate.
	graceAU, graceUA := time.Now().Add(-1*24*time.Hour), time.Now().Add(-50*time.Hour)
	grace := newSub("IVPN Tier 2", graceAU, graceUA)
	require.Equal(t, model.StatusGracePeriod, grace.GetStatus(), "sanity: setup yields GracePeriod")

	subRepo.On("FindWithLANotified", mock.Anything).Return([]model.Subscription{}, nil).Once()
	subRepo.On("FindExpiredUnnotified", mock.Anything).Return([]model.Subscription{pd, grace}, nil).Once()
	// No SetNotified call expected — post-filter rejects both.
	// No mailer call expected.

	NotifyExpiringSubscriptions(subRepo, accountRepo, mailer)
}
