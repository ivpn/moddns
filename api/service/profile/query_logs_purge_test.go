package profile_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
)

// withQueryLogs attaches a mocked query-logs repository and accepts any cache deletion.
func withQueryLogs(t *testing.T, h *transitionsHarness) *mocks.QueryLogsRepository {
	t.Helper()
	ql := mocks.NewQueryLogsRepository(t)
	h.svc.QueryLogsService = querylogs.NewQueryLogsService(ql)
	h.cache.On("Del", mock.Anything, mock.Anything).Return(nil).Maybe()
	return ql
}

func logsProfile(enabled bool) *model.Profile {
	p := statsProfile(false, nil)
	p.Settings.Logs.Enabled = enabled
	return p
}

func logsToggle(v bool) model.ProfileUpdate {
	return model.ProfileUpdate{Operation: model.UpdateOperationReplace, Path: "/settings/logs/enabled", Value: v}
}

// deletedKeys returns the cache keys deleted for the profile.
func deletedKeys(h *transitionsHarness, profileId string) []string {
	var keys []string
	for _, c := range h.cache.Calls {
		if c.Method == "Del" {
			if k := c.Arguments.String(1); strings.HasSuffix(k, ":"+profileId) || strings.Contains(k, ":"+profileId+":") {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

func requireLogCachesInvalidated(t *testing.T, h *transitionsHarness, profileId string) {
	t.Helper()
	keys := deletedKeys(h, profileId)
	require.Contains(t, keys, "query_log_devices:"+profileId)
	require.Contains(t, keys, "logs:blocked:"+profileId+":LAST_1_DAY")
	require.Contains(t, keys, "logs:clients:"+profileId+":LAST_1_DAY")
}

// specRef: api-endpoint-behaviour.md J12, G8 — turning logs off deletes them from every retention
// collection and drops the logs caches.
func TestUpdateProfile_LogsDisablePurgesAndInvalidates(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.expectPersist(logsProfile(true))
	ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
	require.NoError(t, err)
	requireLogCachesInvalidated(t, h, "profile123")
}

// specRef: api-endpoint-behaviour.md J12, J6 — Off from "statistics + logs" purges each once.
func TestUpdateProfile_LogsAndStatisticsOffPurgeEachOnce(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	existing := statsProfile(true, nil)
	existing.Settings.Logs.Enabled = true
	h.expectPersist(existing)
	ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{logsToggle(false), statsToggle(false), logsToggle(false)})
	require.NoError(t, err)
	ql.AssertNumberOfCalls(t, "DeleteQueryLogs", 1)
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
}

// specRef: api-endpoint-behaviour.md J12, G22 — the logs transition compares with the replaced stored value.
func TestUpdateProfile_LogsTransitionUsesReplacedValue(t *testing.T) {
	t.Run("read off, stored on: purges", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		h.expectPersistWithStored(logsProfile(false), logsProfile(true))
		ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()

		_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
		require.NoError(t, err)
		ql.AssertNumberOfCalls(t, "DeleteQueryLogs", 1)
	})
	t.Run("read on, stored off: no purge", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		h.expectPersistWithStored(logsProfile(true), logsProfile(false))

		_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
		require.NoError(t, err)
		ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, mock.Anything)
	})
}

// specRef: api-endpoint-behaviour.md J12 — enabling or an unchanged value purges nothing.
func TestUpdateProfile_LogsEnableOrUnchangedPurgesNothing(t *testing.T) {
	for _, tc := range []struct {
		stored, set bool
	}{{false, true}, {true, true}, {false, false}} {
		t.Run(fmt.Sprintf("%v->%v", tc.stored, tc.set), func(t *testing.T) {
			h := newTransitionsHarness(t)
			ql := withQueryLogs(t, h)
			h.expectPersist(logsProfile(tc.stored))

			_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(tc.set)})
			require.NoError(t, err)
			ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, mock.Anything)
		})
	}
}

// specRef: api-endpoint-behaviour.md J12 — a failed purge is left to the sweep; the PATCH succeeds and caches are still dropped.
func TestUpdateProfile_LogsPurgeFailureDoesNotFailThePatch(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.expectPersist(logsProfile(true))
	ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(errors.New("boom")).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
	require.NoError(t, err)
	requireLogCachesInvalidated(t, h, "profile123")
}

// specRef: api-endpoint-behaviour.md J12 — a failed Redis write after the Mongo update still purges.
func TestUpdateProfile_LogsDisableRedisFailureStillPurges(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	existing := logsProfile(true)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(existing, nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(cloneStatsProfile(existing), cloneStatsProfile(existing), nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(errors.New("redis down"))
	ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
	require.Error(t, err)
	ql.AssertNumberOfCalls(t, "DeleteQueryLogs", 1)
}

// specRef: api-endpoint-behaviour.md J12 — no purge when the Mongo update fails.
func TestUpdateProfile_LogsNoPurgeWhenPersistFails(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(logsProfile(true), nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(nil, nil, errors.New("db down"))

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{logsToggle(false)})
	require.Error(t, err)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md J13, J14 — missing and logs-off profiles lose their logs; enabled ones keep them.
func TestPurgeUnconsentedQueryLogs_AppliesRulePerProfile(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"gone", "off", "on"}, nil)
	h.profiles.On("GetProfilesLogsEnabled", mock.Anything, []string{"gone", "off", "on"}).Return(map[string]bool{"off": false, "on": true}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "gone").Return(nil).Once()
	ql.On("DeleteQueryLogs", mock.Anything, "off").Return(nil).Once()

	res, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.UnconsentedQueryLogsPurgeResult{Checked: 3, Purged: 2}, res)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "on")
	requireLogCachesInvalidated(t, h, "off")
	require.Empty(t, deletedKeys(h, "on"))
}

// specRef: api-endpoint-behaviour.md J14 — profile lookups are batched.
func TestPurgeUnconsentedQueryLogs_BatchesProfileLookups(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ids := make([]string, 2500)
	enabled := map[string]bool{}
	for i := range ids {
		ids[i] = fmt.Sprintf("p%d", i)
		enabled[ids[i]] = true
	}
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return(ids, nil)
	h.profiles.On("GetProfilesLogsEnabled", mock.Anything, mock.MatchedBy(func(b []string) bool { return len(b) <= 1000 })).
		Return(enabled, nil).Times(3)

	res, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.UnconsentedQueryLogsPurgeResult{Checked: 2500}, res)
}

// specRef: api-endpoint-behaviour.md J14 — a failed delete is counted and the run continues.
func TestPurgeUnconsentedQueryLogs_DeleteFailureDoesNotStopTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesLogsEnabled", mock.Anything, mock.Anything).Return(map[string]bool{}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Return(errors.New("boom")).Once()
	ql.On("DeleteQueryLogs", mock.Anything, "b").Return(nil).Once()

	res, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.UnconsentedQueryLogsPurgeResult{Checked: 2, Purged: 1, Failed: 1}, res)
}

// specRef: api-endpoint-behaviour.md J14 — the first per-profile timeout ends the run.
func TestPurgeUnconsentedQueryLogs_JobTimeoutStopsTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.svc.SetQueryLogsPurgeTimeouts(20*time.Millisecond, time.Minute)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesLogsEnabled", mock.Anything, mock.Anything).Return(map[string]bool{}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return(context.DeadlineExceeded).Once()

	res, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.UnconsentedQueryLogsPurgeResult{Checked: 1, Failed: 1}, res)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "b")
}

// specRef: api-endpoint-behaviour.md J14 — past the whole-run deadline no further profile is processed.
func TestPurgeUnconsentedQueryLogs_RunDeadlineStopsTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.svc.SetQueryLogsPurgeTimeouts(time.Minute, 50*time.Millisecond)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesLogsEnabled", mock.Anything, mock.Anything).Return(map[string]bool{}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Run(func(mock.Arguments) {
		time.Sleep(120 * time.Millisecond)
	}).Return(nil).Once()

	res, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Purged)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "b")
}

// specRef: api-endpoint-behaviour.md J14 — read failures abort the run before any delete.
func TestPurgeUnconsentedQueryLogs_ReadFailuresAbort(t *testing.T) {
	t.Run("listing ids", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		ql.On("ListQueryLogProfileIDs", mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
		require.Error(t, err)
	})
	t.Run("profile lookup", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a"}, nil)
		h.profiles.On("GetProfilesLogsEnabled", mock.Anything, mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeUnconsentedQueryLogs(context.Background())
		require.Error(t, err)
		ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, mock.Anything)
	})
}
