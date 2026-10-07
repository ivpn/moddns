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
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
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

// expectPersist makes the stored document before the update equal existing.
func (h *transitionsHarness) expectPersist(existing *model.Profile) {
	h.expectPersistWithStored(existing, cloneStatsProfile(existing))
}

// cloneStatsProfile copies what statsProfile sets, since the service edits the copy it read.
func cloneStatsProfile(p *model.Profile) *model.Profile {
	c := *p
	settings := *p.Settings
	stats, logs := *p.Settings.Statistics, *p.Settings.Logs
	settings.Statistics, settings.Logs = &stats, &logs
	c.Settings = &settings
	return &c
}

// expectPersistWithStored lets the read used for validation differ from the document the
// update replaces, as when another PATCH lands in between.
func (h *transitionsHarness) expectPersistWithStored(read, stored *model.Profile) {
	h.profiles.On("GetProfileById", mock.Anything, read.ProfileId).Return(read, nil)
	h.profiles.On("UpdateFields", mock.Anything, read.ProfileId, mock.Anything).Return(stored, stored, nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, read.ProfileId, mock.Anything).Return(nil).Maybe()
}

// sentUpdate returns the update passed to the repository.
func (h *transitionsHarness) sentUpdate(t *testing.T) repository.ProfileFieldsUpdate {
	t.Helper()
	for _, c := range h.profiles.Calls {
		if c.Method == "UpdateFields" {
			return c.Arguments.Get(2).(repository.ProfileFieldsUpdate)
		}
	}
	t.Fatal("UpdateFields was not called")
	return repository.ProfileFieldsUpdate{}
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

func statsFieldSet(v bool) repository.FieldSet {
	return repository.FieldSet{Field: "settings.statistics.enabled", Value: v}
}

// specRef: api-endpoint-behaviour.md J6, G7, G22 — disabling purges and hands the clock to the
// update, which clears enabled_at against the stored value.
func TestUpdateProfile_StatisticsDisablePurges(t *testing.T) {
	h := newTransitionsHarness(t)
	past := time.Now().Add(-time.Hour)
	h.expectPersist(statsProfile(true, &past))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.NoError(t, err)
	upd := h.sentUpdate(t)
	require.Equal(t, []repository.FieldSet{statsFieldSet(false)}, upd.Set)
	require.True(t, upd.EnabledAtNow.Equal(fixedNow))
}

// specRef: api-endpoint-behaviour.md J6, G20 — the transition runs once per PATCH, not once per path,
// and a repeated path is written once with its last value.
func TestUpdateProfile_StatisticsTransitionRunsOncePerPatch(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(statsProfile(true, nil))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	updates := []model.ProfileUpdate{
		statsToggle(false),
		{Operation: model.UpdateOperationReplace, Path: "/settings/logs/enabled", Value: true},
		statsToggle(false),
	}
	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", updates)
	require.NoError(t, err)
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
	require.Equal(t, []repository.FieldSet{statsFieldSet(false), {Field: "settings.logs.enabled", Value: true}}, h.sentUpdate(t).Set)
}

// specRef: api-endpoint-behaviour.md G7, G20 — a PATCH with no net change purges nothing.
func TestUpdateProfile_StatisticsNoNetChangePurgesNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	h.expectPersist(statsProfile(true, &at))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{statsToggle(false), statsToggle(true)})
	require.NoError(t, err)
	require.Equal(t, []repository.FieldSet{statsFieldSet(true)}, h.sentUpdate(t).Set)
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G7 — enabling purges nothing and passes the UTC clock for enabled_at.
func TestUpdateProfile_StatisticsEnablePassesClock(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(statsProfile(false, nil))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(true)})
	require.NoError(t, err)
	upd := h.sentUpdate(t)
	require.Equal(t, []repository.FieldSet{statsFieldSet(true)}, upd.Set)
	require.True(t, upd.EnabledAtNow.Equal(fixedNow))
	require.Equal(t, time.UTC, upd.EnabledAtNow.Location(), "enabled_at is stored in UTC")
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G7, G21 — an unrelated PATCH does not write statistics.
func TestUpdateProfile_UnrelatedPatchLeavesStatistics(t *testing.T) {
	h := newTransitionsHarness(t)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	h.expectPersist(statsProfile(true, &at))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{{Operation: model.UpdateOperationReplace, Path: "/settings/logs/retention", Value: "1d"}})
	require.NoError(t, err)
	require.Equal(t, []repository.FieldSet{{Field: "settings.logs.retention", Value: model.Retention("1d")}}, h.sentUpdate(t).Set)
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G22 — the transition compares with the stored document the
// update replaced, not with the copy read for validation.
func TestUpdateProfile_StatisticsTransitionUsesReplacedValue(t *testing.T) {
	t.Run("read off, stored on: purges", func(t *testing.T) {
		h := newTransitionsHarness(t)
		h.expectPersistWithStored(statsProfile(false, nil), statsProfile(true, nil))
		h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

		_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
		require.NoError(t, err)
		h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
	})
	t.Run("read on, stored off: no purge", func(t *testing.T) {
		h := newTransitionsHarness(t)
		h.expectPersistWithStored(statsProfile(true, nil), statsProfile(false, nil))

		_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
		require.NoError(t, err)
		h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
	})
}

// specRef: api-endpoint-behaviour.md J6 — a failed immediate purge is left to the unconsented-statistics purge and the PATCH still succeeds.
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
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(nil, nil, errors.New("db down"))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.Error(t, err)
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J6, G23 — a failed Redis write after the Mongo update still purges.
func TestUpdateProfile_StatisticsDisableRedisFailureStillPurges(t *testing.T) {
	h := newTransitionsHarness(t)
	existing := statsProfile(true, nil)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(existing, nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(cloneStatsProfile(existing), cloneStatsProfile(existing), nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(errors.New("redis down"))
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

// specRef: api-endpoint-behaviour.md J7 — a failed purge is logged and left to the unconsented-statistics purge; the deletion succeeds.
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

func retentionProfile(enabled bool, retention model.StatisticsRetention) *model.Profile {
	p := statsProfile(enabled, nil)
	p.Settings.Statistics.Retention = retention
	return p
}

func retentionOp(v string) model.ProfileUpdate {
	return model.ProfileUpdate{Operation: model.UpdateOperationReplace, Path: "/settings/statistics/retention", Value: v}
}

// specRef: api-endpoint-behaviour.md J50, J51, G22 — lowering the retention of an enabled profile
// moves its daily statistics once, compared with the replaced value; raising, a disabled profile
// and a PATCH that also turns statistics off move nothing.
func TestUpdateProfile_StatisticsRetentionTransitions(t *testing.T) {
	cases := []struct {
		name      string
		read      *model.Profile
		stored    *model.Profile
		ops       []model.ProfileUpdate
		wantMove  model.StatisticsRetention
		wantPurge bool
	}{
		{name: "1y to 30d moves", read: retentionProfile(true, "1y"), stored: retentionProfile(true, "1y"), ops: []model.ProfileUpdate{retentionOp("30d")}, wantMove: "30d"},
		{name: "1y to 90d moves", read: retentionProfile(true, "1y"), stored: retentionProfile(true, "1y"), ops: []model.ProfileUpdate{retentionOp("90d")}, wantMove: "90d"},
		{name: "empty stored reads as 30d: nothing lower", read: retentionProfile(true, ""), stored: retentionProfile(true, ""), ops: []model.ProfileUpdate{retentionOp("30d")}},
		{name: "raising moves nothing", read: retentionProfile(true, "30d"), stored: retentionProfile(true, "30d"), ops: []model.ProfileUpdate{retentionOp("1y")}},
		{name: "disabled profile moves nothing", read: retentionProfile(false, "1y"), stored: retentionProfile(false, "1y"), ops: []model.ProfileUpdate{retentionOp("30d")}},
		{name: "read 1y but stored 30d moves nothing", read: retentionProfile(true, "1y"), stored: retentionProfile(true, "30d"), ops: []model.ProfileUpdate{retentionOp("30d")}},
		{name: "lowering and turning off purges only", read: retentionProfile(true, "1y"), stored: retentionProfile(true, "1y"), ops: []model.ProfileUpdate{retentionOp("30d"), statsToggle(false)}, wantPurge: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTransitionsHarness(t)
			h.expectPersistWithStored(tc.read, tc.stored)
			if tc.wantMove != "" {
				h.statsRepo.On("MoveProfileDailyStatistics", mock.Anything, "profile123", tc.wantMove, mock.Anything).Return(2, nil).Once()
			}
			if tc.wantPurge {
				h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()
			}

			_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", tc.ops)
			require.NoError(t, err)
			if tc.wantMove == "" {
				h.statsRepo.AssertNotCalled(t, "MoveProfileDailyStatistics", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

// specRef: api-endpoint-behaviour.md J50 — a failed immediate move does not fail the PATCH.
func TestUpdateProfile_StatisticsRetentionMoveFailureIsBestEffort(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(retentionProfile(true, "1y"))
	h.statsRepo.On("MoveProfileDailyStatistics", mock.Anything, "profile123", model.StatisticsRetention30d, mock.Anything).Return(0, errors.New("boom")).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{retentionOp("30d")})
	require.NoError(t, err)
}

// specRef: api-endpoint-behaviour.md J52 — deleting history stamps keep-since on an enabled
// profile, deletes everything and leaves the settings alone.
func TestDeleteStatisticsHistory(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		h := newTransitionsHarness(t)
		h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(statsProfile(enabled, nil), nil)
		h.profiles.On("SetStatisticsHistoryDeletedAt", mock.Anything, "profile123", fixedNow).Return(enabled, nil).Once()
		h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

		require.NoError(t, h.svc.DeleteStatisticsHistory(context.Background(), "account123", "profile123"))
		h.profiles.AssertNotCalled(t, "UpdateFields", mock.Anything, mock.Anything, mock.Anything)
	}
}

// specRef: api-endpoint-behaviour.md J52 — a foreign profile is not found and nothing is deleted.
func TestDeleteStatisticsHistory_ForeignProfile(t *testing.T) {
	h := newTransitionsHarness(t)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(statsProfile(true, nil), nil)

	err := h.svc.DeleteStatisticsHistory(context.Background(), "other-account", "profile123")
	require.ErrorIs(t, err, dbErrors.ErrProfileNotFound)
	h.statsRepo.AssertNotCalled(t, "DeleteProfileStatistics", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J52 — a failed delete is reported; the stamp stays for the purge.
func TestDeleteStatisticsHistory_DeleteFailure(t *testing.T) {
	h := newTransitionsHarness(t)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(statsProfile(true, nil), nil)
	h.profiles.On("SetStatisticsHistoryDeletedAt", mock.Anything, "profile123", fixedNow).Return(true, nil).Once()
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	require.Error(t, h.svc.DeleteStatisticsHistory(context.Background(), "account123", "profile123"))
}
