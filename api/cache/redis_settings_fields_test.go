package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/api/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md G23 — a per-field write leaves the hashes exactly as a full
// write of the resulting settings would, and does not touch the blocklists or services lists.
func TestSetProfileSettingsFields_MatchesFullWrite(t *testing.T) {
	ctx := context.Background()
	newClient := func() *redis.Client {
		mr := miniredis.RunT(t)
		c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	base := func() *model.ProfileSettings {
		s := model.NewSettings()
		s.ProfileId = "p1"
		s.Privacy.Blocklists = []string{"bl1"}
		s.Privacy.Services = []string{"svc1"}
		return s
	}

	got := newClient()
	require.NoError(t, NewRedisCacheFromClient(got).CreateOrUpdateProfileSettings(ctx, base(), false))
	require.NoError(t, NewRedisCacheFromClient(got).SetProfileSettingsFields(ctx, "p1", []SettingsField{
		{Hash: "statistics", Field: "enabled", Value: true},
		{Hash: "logs", Field: "enabled", Value: true},
		{Hash: "logs", Field: "log_clients_ips", Value: true},
		{Hash: "logs", Field: "log_domains", Value: false},
		{Hash: "logs", Field: "retention", Value: model.Retention("1w")},
		{Hash: "privacy", Field: "default_rule", Value: model.DEFAULT_RULE_BLOCK},
		{Hash: "privacy", Field: "blocklists_subdomains_rule", Value: model.ACTION_ALLOW},
		{Hash: "privacy", Field: "custom_rules_subdomains_rule", Value: model.CUSTOM_RULES_SUBDOMAINS_EXACT},
		{Hash: "security:dnssec", Field: "enabled", Value: false},
		{Hash: "security:dnssec", Field: "send_do_bit", Value: true},
		{Hash: "security:rebinding_protection", Field: "enabled", Value: true},
		{Hash: "advanced", Field: "recursor", Value: model.RECURSOR_SDNS},
	}))

	wantSettings := base()
	wantSettings.Statistics.Enabled = true
	wantSettings.Logs = &model.LogsSettings{Enabled: true, LogClientsIPs: true, LogDomains: false, Retention: "1w"}
	wantSettings.Privacy.DefaultRule = model.DEFAULT_RULE_BLOCK
	wantSettings.Privacy.BlocklistsSubdomainsRule = model.ACTION_ALLOW
	wantSettings.Privacy.CustomRulesSubdomainsRule = model.CUSTOM_RULES_SUBDOMAINS_EXACT
	wantSettings.Security.DNSSECSettings = model.DNSSECSettings{Enabled: false, SendDoBit: true}
	wantSettings.Security.RebindingProtection.Enabled = true
	wantSettings.Advanced.Recursor = model.RECURSOR_SDNS
	want := newClient()
	require.NoError(t, NewRedisCacheFromClient(want).CreateOrUpdateProfileSettings(ctx, wantSettings, false))

	for _, h := range []string{"logs", "statistics", "privacy", "security:dnssec", "security:rebinding_protection", "advanced"} {
		key := "settings:p1:" + h
		w, err := want.HGetAll(ctx, key).Result()
		require.NoError(t, err)
		g, err := got.HGetAll(ctx, key).Result()
		require.NoError(t, err)
		require.Equal(t, w, g, key)
	}
	for _, l := range []string{"blocklists", "services"} {
		g, err := got.LRange(ctx, "settings:p1:"+l, 0, -1).Result()
		require.NoError(t, err)
		require.Len(t, g, 1, l)
	}
}
