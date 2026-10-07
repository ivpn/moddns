package profile_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db/repository"
	intvldtr "github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
)

func defaultsProfile() *model.Profile {
	settings := model.NewSettings()
	settings.ProfileId = "profile123"
	return &model.Profile{ProfileId: "profile123", AccountId: "account123", Name: "one", Settings: settings}
}

// withAPIValidator installs the validator with the API's custom tags, which /name needs.
func withAPIValidator(t *testing.T, h *transitionsHarness) {
	t.Helper()
	v, err := intvldtr.NewAPIValidator()
	require.NoError(t, err)
	h.svc.Validate = v.Validator
}

func replaceOp(path string, value any) model.ProfileUpdate {
	return model.ProfileUpdate{Operation: model.UpdateOperationReplace, Path: path, Value: value}
}

// specRef: api-endpoint-behaviour.md G20 — one invalid operation fails the PATCH before anything is written.
func TestUpdateProfile_InvalidOperationWritesNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{
		statsToggle(false),
		replaceOp("/settings/advanced/recursor", "nope"),
	})
	require.Error(t, err)
	h.profiles.AssertNotCalled(t, "UpdateFields", mock.Anything, mock.Anything, mock.Anything)
	h.cache.AssertNotCalled(t, "SetProfileSettingsFields", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G20 — operations other than replace change nothing and write nothing.
func TestUpdateProfile_NonReplaceOperationWritesNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{
		{Operation: model.UpdateOperationAdd, Path: "/settings/logs/enabled", Value: true},
	})
	require.NoError(t, err)
	require.False(t, got.Settings.Logs.Enabled)
	h.profiles.AssertNotCalled(t, "UpdateFields", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G21, G23 — only touched fields go to Mongo and Redis; the
// response is the profile as re-read after the update.
func TestUpdateProfile_WritesOnlyTouchedFields(t *testing.T) {
	h := newTransitionsHarness(t)
	withAPIValidator(t, h)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	h.profiles.On("GetProfilesByAccountId", mock.Anything, "account123").Return([]model.Profile{}, nil)
	reread := defaultsProfile()
	reread.Name = "re-read"
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(defaultsProfile(), reread, nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", []cache.SettingsField{
		{Hash: "security:dnssec", Field: "send_do_bit", Value: true},
		{Hash: "logs", Field: "retention", Value: model.Retention("1w")},
	}).Return(nil).Once()

	got, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{
		replaceOp("/settings/security/dnssec/send_do_bit", true),
		replaceOp("/name", "two"),
		replaceOp("/settings/logs/retention", "1w"),
	})
	require.NoError(t, err)
	require.Equal(t, "re-read", got.Name)
	require.Equal(t, []repository.FieldSet{
		{Field: "settings.security.dnssec.send_do_bit", Value: true},
		{Field: "name", Value: "two"},
		{Field: "settings.logs.retention", Value: model.Retention("1w")},
	}, h.sentUpdate(t).Set)
}

// specRef: api-endpoint-behaviour.md G23 — a name-only PATCH writes nothing to Redis.
func TestUpdateProfile_NameOnlySkipsRedis(t *testing.T) {
	h := newTransitionsHarness(t)
	withAPIValidator(t, h)
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	h.profiles.On("GetProfilesByAccountId", mock.Anything, "account123").Return([]model.Profile{}, nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(defaultsProfile(), defaultsProfile(), nil)

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{replaceOp("/name", "two")})
	require.NoError(t, err)
	h.cache.AssertNotCalled(t, "SetProfileSettingsFields", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G12, G21 — default_rule = block appends an allow rule per
// allowed domain in the same update; the repository skips values already stored.
func TestUpdateProfile_DefaultRuleBlockAppendsAllowedDomains(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"a.example", "b.example"}}
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(defaultsProfile(), defaultsProfile(), nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_BLOCK)})
	require.NoError(t, err)
	rules := h.sentUpdate(t).AppendCustomRules
	require.Len(t, rules, 2)
	for i, domain := range []string{"a.example", "b.example"} {
		require.Equal(t, domain, rules[i].Value)
		require.EqualValues(t, model.ACTION_ALLOW, rules[i].Action)
		require.False(t, rules[i].ID.IsZero())
	}
}

// specRef: api-endpoint-behaviour.md G12, G21 — default_rule = allow appends nothing.
func TestUpdateProfile_DefaultRuleAllowAppendsNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"a.example"}}
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(defaultsProfile(), defaultsProfile(), nil)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_ALLOW)})
	require.NoError(t, err)
	require.Empty(t, h.sentUpdate(t).AppendCustomRules)
}

// appendingRepo makes UpdateFields behave like the store for the custom-rule append: rules
// whose value is in stored are skipped, the others land in the re-read profile.
func appendingRepo(h *transitionsHarness, stored ...string) {
	h.profiles.EXPECT().UpdateFields(mock.Anything, "profile123", mock.Anything).RunAndReturn(
		func(_ context.Context, _ string, upd repository.ProfileFieldsUpdate) (*model.Profile, *model.Profile, error) {
			before, after := defaultsProfile(), defaultsProfile()
			for _, v := range stored {
				r := &model.CustomRule{ID: primitive.NewObjectID(), Action: model.ACTION_ALLOW, Value: v}
				before.Settings.CustomRules = append(before.Settings.CustomRules, r)
				after.Settings.CustomRules = append(after.Settings.CustomRules, r)
			}
			for _, r := range upd.AppendCustomRules {
				if !slices.Contains(stored, r.Value) {
					c := *r
					after.Settings.CustomRules = append(after.Settings.CustomRules, &c)
				}
			}
			return before, after, nil
		})
}

func ruleValues(rules []*model.CustomRule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Value)
	}
	return out
}

// specRef: api-endpoint-behaviour.md G24 — the appended allow rules reach Redis, where the proxy reads them.
func TestUpdateProfile_DefaultRuleBlockWritesAppendedRulesToRedis(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"app.example", "api.example"}}
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	appendingRepo(h)
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)
	var written []*model.CustomRule
	h.cache.On("AddCustomRules", mock.Anything, "profile123", mock.Anything).Run(func(args mock.Arguments) {
		written = args.Get(2).([]*model.CustomRule)
	}).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_BLOCK)})
	require.NoError(t, err)
	require.Equal(t, []string{"app.example", "api.example"}, ruleValues(written))
	sent := h.sentUpdate(t).AppendCustomRules
	for i := range written {
		require.Equal(t, sent[i].ID, written[i].ID, "Redis gets the ids stored in Mongo")
		require.EqualValues(t, model.ACTION_ALLOW, written[i].Action)
	}
}

// specRef: api-endpoint-behaviour.md G21, G24 — a value already stored is not appended and not rewritten.
func TestUpdateProfile_DefaultRuleBlockWritesOnlyNewlyAppendedRules(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"app.example", "api.example"}}
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	appendingRepo(h, "app.example")
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)
	h.cache.On("AddCustomRules", mock.Anything, "profile123", mock.MatchedBy(func(r []*model.CustomRule) bool {
		return slices.Equal(ruleValues(r), []string{"api.example"})
	})).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_BLOCK)})
	require.NoError(t, err)
}

// specRef: api-endpoint-behaviour.md G24 — nothing appended, nothing written.
func TestUpdateProfile_DefaultRuleBlockAllStoredWritesNothing(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"app.example"}}
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
	appendingRepo(h, "app.example")
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
		[]model.ProfileUpdate{replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_BLOCK)})
	require.NoError(t, err)
	h.cache.AssertNotCalled(t, "AddCustomRules", mock.Anything, mock.Anything, mock.Anything)
}

// specRef: api-endpoint-behaviour.md G24, J6 — a Redis error on the rules fails the PATCH after transitions ran.
func TestUpdateProfile_DefaultRuleBlockRedisFailure(t *testing.T) {
	h := newTransitionsHarness(t)
	h.svc.ServerConfig = config.ServerConfig{AllowedDomains: []string{"app.example"}}
	read := defaultsProfile()
	read.Settings.Statistics.Enabled = true
	h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(read, nil)
	h.profiles.EXPECT().UpdateFields(mock.Anything, "profile123", mock.Anything).RunAndReturn(
		func(_ context.Context, _ string, upd repository.ProfileFieldsUpdate) (*model.Profile, *model.Profile, error) {
			before, after := defaultsProfile(), defaultsProfile()
			before.Settings.Statistics.Enabled = true
			after.Settings.CustomRules = append(after.Settings.CustomRules, upd.AppendCustomRules...)
			return before, after, nil
		})
	h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", mock.Anything).Return(nil)
	h.cache.On("AddCustomRules", mock.Anything, "profile123", mock.Anything).Return(errors.New("redis down")).Once()
	h.statsRepo.On("DeleteProfileStatistics", mock.Anything, "profile123", (*time.Time)(nil)).Return(nil).Once()

	_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123", []model.ProfileUpdate{
		replaceOp("/settings/privacy/default_rule", model.DEFAULT_RULE_BLOCK),
		statsToggle(false),
	})
	require.Error(t, err)
	h.statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)
}

// specRef: api-endpoint-behaviour.md G26, J48 — the retention path writes Mongo and the Redis statistics hash.
func TestUpdateProfile_StatisticsRetention(t *testing.T) {
	for _, value := range []string{"30d", "90d", "1y"} {
		t.Run(value, func(t *testing.T) {
			h := newTransitionsHarness(t)
			h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)
			h.profiles.On("UpdateFields", mock.Anything, "profile123", mock.Anything).Return(defaultsProfile(), defaultsProfile(), nil)
			h.cache.On("SetProfileSettingsFields", mock.Anything, "profile123", []cache.SettingsField{
				{Hash: "statistics", Field: "retention", Value: model.StatisticsRetention(value)},
			}).Return(nil).Once()

			_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
				[]model.ProfileUpdate{replaceOp("/settings/statistics/retention", value)})
			require.NoError(t, err)
			require.Equal(t, []repository.FieldSet{{Field: "settings.statistics.retention", Value: model.StatisticsRetention(value)}}, h.sentUpdate(t).Set)
		})
	}
}

// specRef: api-endpoint-behaviour.md G26, G20 — an unknown retention fails the PATCH before any write.
func TestUpdateProfile_StatisticsRetentionInvalid(t *testing.T) {
	for _, value := range []any{"1m", "", "365d", 30} {
		h := newTransitionsHarness(t)
		h.profiles.On("GetProfileById", mock.Anything, "profile123").Return(defaultsProfile(), nil)

		_, err := h.svc.UpdateProfile(context.Background(), "account123", "profile123",
			[]model.ProfileUpdate{replaceOp("/settings/statistics/retention", value)})
		require.ErrorIs(t, err, profile.ErrStatisticsRetentionInvalid, "%v", value)
		h.profiles.AssertNotCalled(t, "UpdateFields", mock.Anything, mock.Anything, mock.Anything)
	}
}

// specRef: api-endpoint-behaviour.md J48 — new profiles start at 30d.
func TestNewSettings_StatisticsRetentionDefault(t *testing.T) {
	require.Equal(t, model.StatisticsRetention30d, model.NewSettings().Statistics.Retention)
}
