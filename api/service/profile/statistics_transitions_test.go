package profile_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
)

// fixedNow keeps time-derived values deterministic.
var fixedNow = time.Date(2026, 9, 29, 10, 14, 50, 0, time.UTC)

type transitionsHarness struct {
	svc       *profile.ProfileService
	profiles  *mocks.ProfileRepository
	statsRepo *mocks.StatisticsRepository
	cache     *mocks.Cachecache
}

func newTransitionsHarness(t *testing.T) *transitionsHarness {
	t.Helper()
	h := &transitionsHarness{
		profiles:  mocks.NewProfileRepository(t),
		statsRepo: mocks.NewStatisticsRepository(t),
		cache:     mocks.NewCachecache(t),
	}
	statsSvc := statistics.NewStatisticsService(h.statsRepo)
	h.svc = profile.NewProfileService(config.ServerConfig{}, config.ServiceConfig{}, h.profiles, mocks.NewAccountRepository(t),
		nil, nil, statsSvc, nil, h.cache, mocks.NewGeneratoridgen(t), validator.New())
	h.svc.SetClock(func() time.Time { return fixedNow })
	return h
}

func (h *transitionsHarness) expectPersist(existing *model.Profile) {
	h.profiles.On("GetProfileById", mock.Anything, existing.ProfileId).Return(existing, nil)
	h.profiles.On("Update", mock.Anything, existing.ProfileId, mock.Anything).Return(nil)
	h.cache.On("CreateOrUpdateProfileSettings", mock.Anything, mock.Anything, false).Return(nil)
}

func statsProfile(enabled bool, enabledAt *time.Time) *model.Profile {
	return &model.Profile{
		ProfileId: "profile123",
		AccountId: "account123",
		Settings: &model.ProfileSettings{
			ProfileId:  "profile123",
			Statistics: &model.StatisticsSettings{Enabled: enabled, EnabledAt: enabledAt},
			Logs:       &model.LogsSettings{Enabled: true},
		},
	}
}

func statsToggle(v bool) model.ProfileUpdate {
	return model.ProfileUpdate{Operation: model.UpdateOperationReplace, Path: "/settings/statistics/enabled", Value: v}
}

// specRef: api-endpoint-behaviour.md J6, G7
func TestUpdateProfile_StatisticsDisablePurgesAndClearsEnabledAt(t *testing.T) {
	h := newTransitionsHarness(t)
	past := time.Now().Add(-time.Hour)
	existing := statsProfile(true, &past)
	h.expectPersist(existing)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.NoError(t, err)
	require.False(t, got.Settings.Statistics.Enabled)
	require.Nil(t, got.Settings.Statistics.EnabledAt, "enabled_at is cleared on true->false")
}

// specRef: api-endpoint-behaviour.md J6 — the transition runs once per PATCH, not once per path.
func TestUpdateProfile_StatisticsTransitionRunsOncePerPatch(t *testing.T) {
	h := newTransitionsHarness(t)
	existing := statsProfile(true, nil)
	h.expectPersist(existing)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	updates := []model.ProfileUpdate{
		statsToggle(false),
		{Operation: model.UpdateOperationReplace, Path: "/settings/logs/enabled", Value: true},
		statsToggle(false),
	}
	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", updates)
	require.NoError(t, err)
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
}

// specRef: api-endpoint-behaviour.md G7 — a PATCH with no net change purges nothing and keeps enabled_at.
func TestUpdateProfile_StatisticsNoNetChangeKeepsStateAndPurgesNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	existing := statsProfile(true, &at)
	h.expectPersist(existing)

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{statsToggle(false), statsToggle(true)})
	require.NoError(t, err)
	require.True(t, got.Settings.Statistics.Enabled)
	require.NotNil(t, got.Settings.Statistics.EnabledAt)
	require.True(t, got.Settings.Statistics.EnabledAt.Equal(at))
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G7 — enabled_at is set on false->true and nothing is purged.
func TestUpdateProfile_StatisticsEnableSetsEnabledAt(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(statsProfile(false, nil))

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(true)})
	require.NoError(t, err)
	require.True(t, got.Settings.Statistics.Enabled)
	require.NotNil(t, got.Settings.Statistics.EnabledAt)
	require.True(t, got.Settings.Statistics.EnabledAt.Equal(fixedNow))
	require.Equal(t, time.UTC, got.Settings.Statistics.EnabledAt.Location(), "enabled_at is stored in UTC")
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G7 — an unrelated PATCH leaves statistics.enabled_at untouched.
func TestUpdateProfile_UnrelatedPatchLeavesEnabledAt(t *testing.T) {
	h := newTransitionsHarness(t)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	h.expectPersist(statsProfile(true, &at))

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{{Operation: model.UpdateOperationReplace, Path: "/settings/logs/enabled", Value: false}})
	require.NoError(t, err)
	require.True(t, got.Settings.Statistics.EnabledAt.Equal(at))
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J6 — a failed immediate purge is left to the reconciler and the PATCH still succeeds.
func TestUpdateProfile_StatisticsPurgeFailureDoesNotFailThePatch(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(statsProfile(true, nil))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.NoError(t, err)
}

// specRef: api-endpoint-behaviour.md J6 — no purge runs when persisting the change fails.
func TestUpdateProfile_StatisticsNoPurgeWhenPersistFails(t *testing.T) {
	h := newTransitionsHarness(t)
	existing := statsProfile(true, nil)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(existing, nil)
	h.profiles.On("Update", mock.Anything, "profile123", mock.Anything).Return(errors.New("db down"))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.Error(t, err)
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J6 — a failed Redis write after the Mongo update still purges.
func TestUpdateProfile_StatisticsDisableRedisFailureStillPurges(t *testing.T) {
	h := newTransitionsHarness(t)
	existing := statsProfile(true, nil)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(existing, nil)
	h.profiles.On("Update", mock.Anything, "profile123", mock.Anything).Return(nil)
	h.cache.On("CreateOrUpdateProfileSettings", mock.Anything, mock.Anything, false).Return(errors.New("redis down"))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.Error(t, err)
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
}

func (h *transitionsHarness) expectDelete(t *testing.T, existing *model.Profile, deleteErr error) {
	t.Helper()
	qlRepo := mocks.NewQueryLogsRepository(t)
	h.svc.QueryLogsService = querylogs.NewQueryLogsService(qlRepo)
	h.profiles.On("GetProfileById", mock.Anything, existing.ProfileId).Return(existing, nil)
	h.profiles.On("DeleteProfileById", mock.Anything, existing.ProfileId).Return(deleteErr)
	qlRepo.On("DeleteQueryLogs", mock.Anything, existing.ProfileId).Return(nil).Maybe()
	h.cache.On("DeleteProfileSettings", mock.Anything, existing.ProfileId).Return(nil).Maybe()
}

// specRef: api-endpoint-behaviour.md J7 — profile deletion purges the profile's statistics.
func TestDeleteProfile_PurgesStatistics(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), nil)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	require.NoError(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
}

// specRef: api-endpoint-behaviour.md J7 — a failed purge is logged and left to the reconciler; the deletion succeeds.
func TestDeleteProfile_StatisticsPurgeFailureDoesNotFailTheDeletion(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), nil)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	require.NoError(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))
}

// specRef: api-endpoint-behaviour.md J7 — the statistics purge is not cancelled by a failing sibling leg.
func TestDeleteProfile_StatisticsPurgeRunsEvenWhenAnotherLegFails(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), errors.New("db down"))
	var ctxErr error
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond) // let the failing sibling leg cancel the errgroup context first
		ctxErr = args.Get(0).(context.Context).Err()
	}).Return(nil).Once()

	require.Error(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))
	require.NoError(t, ctxErr)
}
