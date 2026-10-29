package profile_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
)

// withQueryLogs attaches a mocked query-logs repository and accepts any cache generation bump.
func withQueryLogs(t *testing.T, h *transitionsHarness) *mocks.QueryLogsRepository {
	t.Helper()
	ql := mocks.NewQueryLogsRepository(t)
	h.svc.QueryLogsService = querylogs.NewQueryLogsService(ql, querylogs.WithClock(func() time.Time { return fixedNow }))
	h.cache.On("Incr", mock.Anything, mock.Anything, cache.GenerationTTL).Return(int64(1), nil).Maybe()
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

// generationBumps counts the bumps of the profile's query-log cache generation.
func generationBumps(h *transitionsHarness, profileId string) int {
	n := 0
	for _, c := range h.cache.Calls {
		if c.Method == "Incr" && c.Arguments.String(1) == cache.GenerationKey("logs", profileId) {
			n++
		}
	}
	return n
}

// requireLogCachesInvalidated checks the profile's query-log caches moved to a new generation
// once, which retires the devices list and every top list at once (J23).
func requireLogCachesInvalidated(t *testing.T, h *transitionsHarness, profileId string) {
	t.Helper()
	require.Equal(t, 1, generationBumps(h, profileId))
	h.cache.AssertNotCalled(t, "Del", mock.Anything, mock.Anything)
}

func logsRetentionOp(v string) model.ProfileUpdate {
	return model.ProfileUpdate{Operation: model.UpdateOperationReplace, Path: "/settings/logs/retention", Value: v}
}

// specRef: api-endpoint-behaviour.md J23, G22 - a logs retention change retires the query-log caches
// once (they were computed with the previous retention's read window); an unchanged retention does
// not, and a PATCH that also turns logs off invalidates only through the purge.
func TestUpdateProfile_LogsRetentionChangeInvalidatesCaches(t *testing.T) {
	cases := []struct {
		name      string
		stored    model.Retention
		ops       []model.ProfileUpdate
		wantBumps int
		wantPurge bool
	}{
		{name: "changed", stored: model.RetentionOneDay, ops: []model.ProfileUpdate{logsRetentionOp("1w")}, wantBumps: 1},
		{name: "unchanged", stored: model.RetentionOneWeek, ops: []model.ProfileUpdate{logsRetentionOp("1w")}},
		{name: "changed and turned off", stored: model.RetentionOneDay, ops: []model.ProfileUpdate{logsRetentionOp("1w"), logsToggle(false)}, wantBumps: 1, wantPurge: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTransitionsHarness(t)
			ql := withQueryLogs(t, h)
			p := logsProfile(true)
			p.Settings.Logs.Retention = tc.stored
			h.expectPersist(p)
			if tc.wantPurge {
				ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()
			}

			_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", tc.ops)
			require.NoError(t, err)
			require.Equal(t, tc.wantBumps, generationBumps(h, "profile123"))
		})
	}
}

func logsSettings(enabled bool, retention model.Retention) *model.LogsSettings {
	return &model.LogsSettings{Enabled: enabled, Retention: retention}
}

// specRef: api-endpoint-behaviour.md J54, J55, G22 — lowering the logs retention while logs stay on trims the
// profile's older rows once, inline, then retires the query-log caches once; raising, an unchanged value or
// turning logs off in the same PATCH trims nothing, and a failed trim does not fail the PATCH.
func TestUpdateProfile_LogsRetentionLoweringTrims(t *testing.T) {
	cases := []struct {
		name      string
		stored    model.Retention
		ops       []model.ProfileUpdate
		wantTrim  bool
		trimErr   error
		wantPurge bool
	}{
		{name: "lowered", stored: model.RetentionOneMonth, ops: []model.ProfileUpdate{logsRetentionOp("1d")}, wantTrim: true},
		{name: "lowered, trim fails", stored: model.RetentionOneMonth, ops: []model.ProfileUpdate{logsRetentionOp("1d")}, wantTrim: true, trimErr: errors.New("boom")},
		{name: "raised", stored: model.RetentionOneDay, ops: []model.ProfileUpdate{logsRetentionOp("1m")}},
		{name: "lowered and turned off", stored: model.RetentionOneMonth, ops: []model.ProfileUpdate{logsRetentionOp("1h"), logsToggle(false)}, wantPurge: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTransitionsHarness(t)
			ql := withQueryLogs(t, h)
			p := logsProfile(true)
			p.Settings.Logs.Retention = tc.stored
			h.expectPersist(p)
			if tc.wantTrim {
				ql.On("TrimQueryLogs", mock.Anything, "profile123", model.RetentionOneDay, fixedNow).Return(int64(2), tc.trimErr).Once()
			}
			if tc.wantPurge {
				ql.On("DeleteQueryLogs", mock.Anything, "profile123").Return(nil).Once()
			}

			_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", tc.ops)
			require.NoError(t, err)
			if !tc.wantTrim {
				ql.AssertNotCalled(t, "TrimQueryLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			require.Equal(t, 1, generationBumps(h, "profile123"))
		})
	}
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
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(int64(0), nil).Once()

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

// specRef: api-endpoint-behaviour.md J12 — a failed purge is left to the next purge run; the PATCH succeeds and caches are still dropped.
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

// specRef: api-endpoint-behaviour.md J13, J14 — missing, logs-off and no-logs-block profiles lose all their
// logs; enabled ones lose the rows older than their retention, and only a profile whose rows changed has its
// caches retired.
func TestPurgeQueryLogs_AppliesRulePerProfile(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"gone", "off", "no-block", "on", "on-noop"}, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, []string{"gone", "off", "no-block", "on", "on-noop"}).Return(map[string]*model.LogsSettings{
		"off":      logsSettings(false, model.RetentionOneWeek),
		"no-block": nil,
		"on":       logsSettings(true, model.RetentionOneDay),
		"on-noop":  logsSettings(true, model.RetentionOneMonth),
	}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "gone").Return(nil).Once()
	ql.On("DeleteQueryLogs", mock.Anything, "off").Return(nil).Once()
	ql.On("DeleteQueryLogs", mock.Anything, "no-block").Return(nil).Once()
	ql.On("TrimQueryLogs", mock.Anything, "on", model.RetentionOneDay, fixedNow).Return(int64(7), nil).Once()
	ql.On("TrimQueryLogs", mock.Anything, "on-noop", model.RetentionOneMonth, fixedNow).Return(int64(0), nil).Once()

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.QueryLogsPurgeResult{Checked: 5, Total: 5, Purged: 3, Deleted: 7}, res)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "on")
	requireLogCachesInvalidated(t, h, "off")
	requireLogCachesInvalidated(t, h, "on")
	require.Zero(t, generationBumps(h, "on-noop"))
}

// specRef: api-endpoint-behaviour.md J14 — profile lookups are batched.
func TestPurgeQueryLogs_BatchesProfileLookups(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ids := make([]string, 2500)
	settings := map[string]*model.LogsSettings{}
	for i := range ids {
		ids[i] = fmt.Sprintf("p%d", i)
		settings[ids[i]] = logsSettings(true, model.RetentionOneMonth)
	}
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return(ids, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.MatchedBy(func(b []string) bool { return len(b) <= 1000 })).
		Return(settings, nil).Times(3)
	ql.On("TrimQueryLogs", mock.Anything, mock.Anything, model.RetentionOneMonth, fixedNow).Return(int64(0), nil).Times(2500)

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.QueryLogsPurgeResult{Checked: 2500, Total: 2500}, res)
}

// specRef: api-endpoint-behaviour.md J14 — a failed delete or trim is counted and the run continues.
func TestPurgeQueryLogs_FailureDoesNotStopTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b", "c"}, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.Anything).Return(map[string]*model.LogsSettings{
		"c": logsSettings(true, model.RetentionOneDay),
	}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Return(errors.New("boom")).Once()
	ql.On("DeleteQueryLogs", mock.Anything, "b").Return(nil).Once()
	ql.On("TrimQueryLogs", mock.Anything, "c", model.RetentionOneDay, fixedNow).Return(int64(0), errors.New("boom")).Once()

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.QueryLogsPurgeResult{Checked: 3, Total: 3, Purged: 1, Failed: 2}, res)
}

// specRef: api-endpoint-behaviour.md J14 — the first per-profile timeout ends the run.
func TestPurgeQueryLogs_JobTimeoutStopsTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.svc.SetQueryLogsPurgeTimeouts(20*time.Millisecond, time.Minute)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.Anything).Return(map[string]*model.LogsSettings{}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return(context.DeadlineExceeded).Once()

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.QueryLogsPurgeResult{Checked: 1, Total: 2, Failed: 1}, res)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "b")
}

// specRef: api-endpoint-behaviour.md J14 — past the whole-run deadline no further profile is processed.
func TestPurgeQueryLogs_RunDeadlineStopsTheRun(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.svc.SetQueryLogsPurgeTimeouts(time.Minute, 50*time.Millisecond)
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a", "b"}, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.Anything).Return(map[string]*model.LogsSettings{}, nil)
	ql.On("DeleteQueryLogs", mock.Anything, "a").Run(func(mock.Arguments) {
		time.Sleep(120 * time.Millisecond)
	}).Return(nil).Once()

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Purged)
	ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, "b")
}

// specRef: api-endpoint-behaviour.md J14 — read failures abort the run before any delete.
func TestPurgeQueryLogs_ReadFailuresAbort(t *testing.T) {
	t.Run("listing ids", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		ql.On("ListQueryLogProfileIDs", mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeQueryLogs(context.Background())
		require.Error(t, err)
	})
	t.Run("profile lookup", func(t *testing.T) {
		h := newTransitionsHarness(t)
		ql := withQueryLogs(t, h)
		ql.On("ListQueryLogProfileIDs", mock.Anything).Return([]string{"a"}, nil)
		h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.Anything).Return(nil, errors.New("down"))
		_, err := h.svc.PurgeQueryLogs(context.Background())
		require.Error(t, err)
		ql.AssertNotCalled(t, "DeleteQueryLogs", mock.Anything, mock.Anything)
	})
}

// specRef: api-endpoint-behaviour.md J14 — a run that reaches its deadline while loading a later batch's settings
// ends with its counts, like any other early stop, instead of failing.
func TestPurgeQueryLogs_DeadlineDuringALaterLookupKeepsTheCounts(t *testing.T) {
	h := newTransitionsHarness(t)
	ql := withQueryLogs(t, h)
	h.svc.SetQueryLogsPurgeTimeouts(time.Minute, 200*time.Millisecond)
	ids := make([]string, 1001)
	first := map[string]*model.LogsSettings{}
	for i := range ids {
		ids[i] = fmt.Sprintf("p%d", i)
		first[ids[i]] = logsSettings(true, model.RetentionOneMonth)
	}
	ql.On("ListQueryLogProfileIDs", mock.Anything).Return(ids, nil)
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, mock.MatchedBy(func(b []string) bool { return len(b) == 1000 })).Return(first, nil).Once()
	h.profiles.On("GetProfilesLogsSettings", mock.Anything, []string{"p1000"}).Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return(nil, context.DeadlineExceeded).Once()
	ql.On("TrimQueryLogs", mock.Anything, mock.Anything, model.RetentionOneMonth, fixedNow).Return(int64(0), nil).Times(1000)

	res, err := h.svc.PurgeQueryLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, profile.QueryLogsPurgeResult{Checked: 1000, Total: 1001}, res)
}
