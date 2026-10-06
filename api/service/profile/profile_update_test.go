package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db/repository"
	intvldtr "github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
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
