package cache

import (
	"context"
	"sort"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/ivpn/dns/api/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func settingsWithRules(values ...string) *model.ProfileSettings {
	s := model.NewSettings()
	s.ProfileId = "p1"
	s.Privacy.Blocklists = []string{"bl1"}
	for _, v := range values {
		s.CustomRules = append(s.CustomRules, &model.CustomRule{ID: primitive.NewObjectID(), Action: model.ACTION_BLOCK, Value: v, Syntax: model.SYNTAX_FQDN})
	}
	return s
}

// dumpRules returns every key of the profile's custom-rule layout with its content.
func dumpRules(t *testing.T, ctx context.Context, c *redis.Client) (members []string, hashes map[string]map[string]string) {
	t.Helper()
	members, err := c.SMembers(ctx, "settings:p1:custom_rules").Result()
	require.NoError(t, err)
	sort.Strings(members)
	keys, err := c.Keys(ctx, "settings:p1:custom_rule:*").Result()
	require.NoError(t, err)
	hashes = map[string]map[string]string{}
	for _, k := range keys {
		h, err := c.HGetAll(ctx, k).Result()
		require.NoError(t, err)
		hashes[k] = h
	}
	return members, hashes
}

func newTestRedis(t *testing.T) (*RedisCache, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCacheFromClient(client), client
}

// specRef: api-endpoint-behaviour.md G25 — the full settings write stores the custom rules in
// exactly the AddCustomRules layout.
func TestCreateOrUpdateProfileSettings_WritesCustomRulesInAddCustomRulesLayout(t *testing.T) {
	ctx := context.Background()
	settings := settingsWithRules("a.example", "b.example")

	got, gotClient := newTestRedis(t)
	require.NoError(t, got.CreateOrUpdateProfileSettings(ctx, settings, false))

	want, wantClient := newTestRedis(t)
	require.NoError(t, want.AddCustomRules(ctx, "p1", settings.CustomRules))

	gotMembers, gotHashes := dumpRules(t, ctx, gotClient)
	wantMembers, wantHashes := dumpRules(t, ctx, wantClient)
	require.Len(t, gotMembers, 2)
	require.Equal(t, wantMembers, gotMembers)
	require.Equal(t, wantHashes, gotHashes)
}

// specRef: api-endpoint-behaviour.md G25 — a rewrite replaces the rules: stale ones disappear.
func TestCreateOrUpdateProfileSettings_ReplacesCustomRules(t *testing.T) {
	ctx := context.Background()
	c, client := newTestRedis(t)
	first := settingsWithRules("keep.example", "drop.example")
	require.NoError(t, c.CreateOrUpdateProfileSettings(ctx, first, false))

	second := settingsWithRules()
	second.CustomRules = []*model.CustomRule{first.CustomRules[0], {ID: primitive.NewObjectID(), Action: model.ACTION_ALLOW, Value: "new.example", Syntax: model.SYNTAX_FQDN}}
	require.NoError(t, c.CreateOrUpdateProfileSettings(ctx, second, false))

	members, hashes := dumpRules(t, ctx, client)
	require.ElementsMatch(t, []string{
		"settings:p1:custom_rule:" + second.CustomRules[0].ID.Hex(),
		"settings:p1:custom_rule:" + second.CustomRules[1].ID.Hex(),
	}, members)
	require.Len(t, hashes, 2)
	require.NotContains(t, hashes, "settings:p1:custom_rule:"+first.CustomRules[1].ID.Hex())
	require.Equal(t, "new.example", hashes["settings:p1:custom_rule:"+second.CustomRules[1].ID.Hex()]["value"])
}

// specRef: api-endpoint-behaviour.md G25 — no rules leave no set and no rule hashes; the other
// settings keys are written as before.
func TestCreateOrUpdateProfileSettings_NoCustomRules(t *testing.T) {
	ctx := context.Background()
	c, client := newTestRedis(t)
	require.NoError(t, c.CreateOrUpdateProfileSettings(ctx, settingsWithRules("x.example"), false))

	for _, rules := range [][]*model.CustomRule{nil, {}} {
		s := settingsWithRules()
		s.CustomRules = rules
		require.NoError(t, c.CreateOrUpdateProfileSettings(ctx, s, false))
		members, hashes := dumpRules(t, ctx, client)
		require.Empty(t, members)
		require.Empty(t, hashes)
		exists, err := client.Exists(ctx, "settings:p1:custom_rules").Result()
		require.NoError(t, err)
		require.Zero(t, exists)
	}

	for _, k := range []string{"logs", "statistics", "privacy", "security:dnssec", "security:rebinding_protection", "advanced"} {
		exists, err := client.Exists(ctx, "settings:p1:"+k).Result()
		require.NoError(t, err)
		require.EqualValues(t, 1, exists, k)
	}
	bl, err := client.LRange(ctx, "settings:p1:blocklists", 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, []string{"bl1"}, bl)
}

// specRef: api-endpoint-behaviour.md G25 — rules added after the full write (import) survive it.
func TestCreateOrUpdateProfileSettings_ThenAddCustomRulesKeepsThem(t *testing.T) {
	ctx := context.Background()
	c, client := newTestRedis(t)
	require.NoError(t, c.CreateOrUpdateProfileSettings(ctx, settingsWithRules(), false))
	rules := settingsWithRules("imported.example").CustomRules
	require.NoError(t, c.AddCustomRules(ctx, "p1", rules))

	members, hashes := dumpRules(t, ctx, client)
	require.Equal(t, []string{"settings:p1:custom_rule:" + rules[0].ID.Hex()}, members)
	require.Len(t, hashes, 1)
}
