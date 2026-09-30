package profile_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
	"github.com/ivpn/dns/api/service/statistics"
)

var transitionsTiming = statistics.PurgeTiming{ProxySettingsTTL: 30 * time.Second}

type transitionsHarness struct {
	svc       *profile.ProfileService
	profiles  *mocks.ProfileRepository
	statsRepo *mocks.StatisticsRepository
	cache     *mocks.Cachecache
	queue     *cache.RedisCache
}

func newTransitionsHarness(t *testing.T) *transitionsHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	h := &transitionsHarness{
		profiles:  mocks.NewProfileRepository(t),
		statsRepo: mocks.NewStatisticsRepository(t),
		cache:     mocks.NewCachecache(t),
		queue:     cache.NewRedisCacheFromClient(client),
	}
	statsSvc := statistics.NewStatisticsService(h.statsRepo, statistics.WithPurgeQueue(h.queue, transitionsTiming))
	h.svc = profile.NewProfileService(config.ServerConfig{}, config.ServiceConfig{}, h.profiles, mocks.NewAccountRepository(t),
		nil, nil, statsSvc, nil, h.cache, mocks.NewGeneratoridgen(t), validator.New())
	return h
}

// dueAll returns every queued job regardless of due time.
func (h *transitionsHarness) dueAll(t *testing.T) []statistics.PurgeJob {
	t.Helper()
	entries, err := h.queue.DueStatisticsPurges(context.Background(), time.Now().Add(24*time.Hour), 100)
	require.NoError(t, err)
	jobs := make([]statistics.PurgeJob, 0, len(entries))
	for _, e := range entries {
		job, ok := statistics.ParsePurgeMember(e.Member)
		require.True(t, ok, "member %q", e.Member)
		jobs = append(jobs, job)
	}
	return jobs
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

// specRef: api-endpoint-behaviour.md J6, J8, G7
func TestUpdateProfile_StatisticsDisablePurgesAndSchedulesDelayedPass(t *testing.T) {
	h := newTransitionsHarness(t)
	past := time.Now().Add(-time.Hour)
	existing := statsProfile(true, &past)
	h.expectPersist(existing)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	start := time.Now()
	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.NoError(t, err)
	require.False(t, got.Settings.Statistics.Enabled)
	require.Nil(t, got.Settings.Statistics.EnabledAt, "enabled_at is cleared on true->false")

	wantBefore, _ := statistics.PurgeSchedule(start, transitionsTiming.ProxySettingsTTL)
	jobs := h.dueAll(t)
	require.Len(t, jobs, 1)
	require.Equal(t, statistics.PurgeKindDisable, jobs[0].Kind)
	require.Equal(t, wantBefore.Unix(), jobs[0].Before.Unix())
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

	start := time.Now()
	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(true)})
	require.NoError(t, err)
	require.True(t, got.Settings.Statistics.Enabled)
	require.NotNil(t, got.Settings.Statistics.EnabledAt)
	require.False(t, got.Settings.Statistics.EnabledAt.Before(start.Add(-time.Second)))
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

// specRef: api-endpoint-behaviour.md J6 — a failed immediate purge is covered by the queued pass and the PATCH still succeeds.
func TestUpdateProfile_StatisticsPurgeFailureIsCoveredByQueuedPass(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectPersist(statsProfile(true, nil))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.NoError(t, err)

	jobs := h.dueAll(t)
	require.Len(t, jobs, 1)
	require.Equal(t, statistics.PurgeKindDisable, jobs[0].Kind)
	require.NotNil(t, jobs[0].Before)
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

// specRef: api-endpoint-behaviour.md J6 — a failed Redis write after the Mongo update still queues the purge jobs.
func TestUpdateProfile_StatisticsDisableRedisFailureStillQueuesPurge(t *testing.T) {
	h := newTransitionsHarness(t)
	existing := statsProfile(true, nil)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(existing, nil)
	h.profiles.On("Update", mock.Anything, "profile123", mock.Anything).Return(nil)
	h.cache.On("CreateOrUpdateProfileSettings", mock.Anything, mock.Anything, false).Return(errors.New("redis down"))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{statsToggle(false)})
	require.Error(t, err)

	jobs := h.dueAll(t)
	require.Len(t, jobs, 1, "the delayed pass is queued even though the Redis write failed")
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

// specRef: api-endpoint-behaviour.md J7 — profile deletion purges statistics and queues an unbounded delayed pass.
func TestDeleteProfile_PurgesStatisticsAndSchedulesDelayedPass(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), nil)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	require.NoError(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))

	jobs := h.dueAll(t)
	require.Len(t, jobs, 1)
	require.Equal(t, statistics.PurgeKindDelete, jobs[0].Kind)
	require.Nil(t, jobs[0].Before)
}

// specRef: api-endpoint-behaviour.md J7 — the delayed pass is queued before the fan-out, so a failed profile delete cannot orphan statistics.
func TestDeleteProfile_DelayedPassQueuedEvenWhenDeleteFails(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), errors.New("db down"))
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Maybe()

	require.Error(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))

	jobs := h.dueAll(t)
	require.Len(t, jobs, 1)
	require.Equal(t, statistics.PurgeKindDelete, jobs[0].Kind)
}

// specRef: api-endpoint-behaviour.md J7 — a failed purge on delete is covered by the queued pass and the deletion succeeds.
func TestDeleteProfile_StatisticsPurgeFailureIsCoveredByQueuedPass(t *testing.T) {
	h := newTransitionsHarness(t)
	h.expectDelete(t, statsProfile(true, nil), nil)
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(errors.New("boom")).Once()

	require.NoError(t, h.svc.DeleteProfile(context.Background(), "account123", "profile123", true))

	jobs := h.dueAll(t)
	require.Len(t, jobs, 1)
	require.Equal(t, statistics.PurgeKindDelete, jobs[0].Kind)
}

// specRef: api-endpoint-behaviour.md J7 — nothing is deleted when the delayed pass cannot be queued.
func TestDeleteProfile_NothingDeletedWhenQueueUnavailable(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	profiles := mocks.NewProfileRepository(t)
	statsRepo := mocks.NewStatisticsRepository(t)
	statsSvc := statistics.NewStatisticsService(statsRepo, statistics.WithPurgeQueue(cache.NewRedisCacheFromClient(client), transitionsTiming))
	svc := profile.NewProfileService(config.ServerConfig{}, config.ServiceConfig{}, profiles, mocks.NewAccountRepository(t),
		nil, nil, statsSvc, nil, mocks.NewCachecache(t), mocks.NewGeneratoridgen(t), validator.New())
	profiles.On("GetProfileById", mock.Anything, "profile123").Return(statsProfile(true, nil), nil)
	mr.Close()

	require.Error(t, svc.DeleteProfile(context.Background(), "account123", "profile123", true))
	profiles.AssertNotCalled(t, "DeleteProfileById", mock.Anything, mock.Anything)
}
