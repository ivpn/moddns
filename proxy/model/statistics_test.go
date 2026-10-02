package model

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// specRef: proxy-statistics-behaviour.md #Y12
func TestClassifyReasons(t *testing.T) {
	tests := []struct {
		name    string
		reasons []string
		want    ReasonClass
	}{
		{name: "blocklists token", reasons: []string{"blocklists"}, want: ReasonClassBlocklist},
		{name: "blocklist id token", reasons: []string{"blocklist: hagezi_pro"}, want: ReasonClassBlocklist},
		{name: "subdomains rule token", reasons: []string{"blocklists_subdomains_rule"}, want: ReasonClassBlocklist},
		{name: "services token", reasons: []string{"services"}, want: ReasonClassService},
		{name: "service id token", reasons: []string{"service: netflix"}, want: ReasonClassService},
		{name: "custom rules", reasons: []string{"custom_rules"}, want: ReasonClassCustomRule},
		{name: "rebinding", reasons: []string{"rebinding_protection"}, want: ReasonClassRebinding},
		{name: "default rule", reasons: []string{"default_rule"}, want: ReasonClassDefaultRule},
		{name: "cname uncloaking is a modifier of the class beside it", reasons: []string{"blocklists", "blocklist: x", "cname_uncloaking"}, want: ReasonClassBlocklist},
		{name: "cname uncloaking alone is not a class", reasons: []string{"cname_uncloaking"}, want: ReasonClassOther},
		{name: "no reasons", reasons: nil, want: ReasonClassOther},
		{name: "unknown token", reasons: []string{"something_new"}, want: ReasonClassOther},
		{name: "custom rule beats everything", reasons: []string{"blocklists", "services", "default_rule", "rebinding_protection", "custom_rules"}, want: ReasonClassCustomRule},
		{name: "rebinding beats blocklist", reasons: []string{"blocklists", "rebinding_protection"}, want: ReasonClassRebinding},
		{name: "blocklist beats service", reasons: []string{"services", "service: x", "blocklists"}, want: ReasonClassBlocklist},
		{name: "service beats default rule", reasons: []string{"default_rule", "services"}, want: ReasonClassService},
		{name: "recognised token beats unknown", reasons: []string{"something_new", "default_rule"}, want: ReasonClassDefaultRule},
		{name: "id lookalike is not a class", reasons: []string{"blocklistsX", "servicesX"}, want: ReasonClassOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ClassifyReasons(tt.reasons))
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestStatisticsBucketStart(t *testing.T) {
	assert.Equal(t, 15*time.Minute, StatisticsBucket)
	loc := time.FixedZone("CEST", 2*3600)
	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{name: "truncated to the quarter", in: time.Date(2026, 9, 17, 13, 44, 59, 9, time.UTC), want: time.Date(2026, 9, 17, 13, 30, 0, 0, time.UTC)},
		{name: "exact boundary is unchanged", in: time.Date(2026, 9, 17, 13, 45, 0, 0, time.UTC), want: time.Date(2026, 9, 17, 13, 45, 0, 0, time.UTC)},
		{name: "local time is normalised to UTC", in: time.Date(2026, 9, 17, 1, 20, 0, 0, loc), want: time.Date(2026, 9, 16, 23, 15, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatisticsBucketStart(tt.in)
			assert.True(t, got.Equal(tt.want), "got %s want %s", got, tt.want)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestStatistics_Count(t *testing.T) {
	doc := Statistics{}
	doc.Count(Queries{Total: 1, Blocked: 1}, &ConsentedStatistics{ReasonClass: ReasonClassBlocklist, Protocol: ProtocolDoH})
	doc.Count(Queries{Total: 1, DNSSEC: 1}, &ConsentedStatistics{Protocol: ProtocolDoT})
	doc.Count(Queries{Total: 1, Blocked: 1}, &ConsentedStatistics{ReasonClass: ReasonClassOther, Protocol: ProtocolDoQ})
	doc.Count(Queries{Total: 1}, &ConsentedStatistics{})

	assert.Equal(t, StatisticsQueries{Total: 4, Blocked: 2, DNSSEC: 1}, doc.Queries)
	assert.Equal(t, StatisticsReasons{Blocklist: 1, Other: 1}, doc.Reasons)
	assert.Equal(t, StatisticsProtocols{DoH: 1, DoT: 1, DoQ: 1}, doc.Protocols)
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestOtherDeviceID_CannotBeANormalisedDeviceID(t *testing.T) {
	assert.Equal(t, 64, MaxDevicesPerProfileBucket)
	assert.Equal(t, "_other", OtherDeviceID)
	assert.True(t, strings.Contains(OtherDeviceID, "_"), "underscore is outside [A-Za-z0-9 -]")
}

// specRef: proxy-statistics-behaviour.md #Y1 #Y19
func TestStatistics_SchemaShape(t *testing.T) {
	forbiddenAnywhere := map[string]bool{"timestamp": true, "client_ip": true, "created_at": true, "flushed_at": true}
	top := map[string]bool{"bucket_start": true, "meta": true, "queries": true, "reasons": true, "protocols": true}
	tagOf := func(f reflect.StructField) string { return strings.Split(f.Tag.Get("bson"), ",")[0] }

	typ := reflect.TypeOf(Statistics{})
	stored := 0
	for i := 0; i < typ.NumField(); i++ {
		tag := tagOf(typ.Field(i))
		if tag == "-" {
			assert.Equal(t, "Retention", typ.Field(i).Name, "only the routing key may be unstored")
			continue
		}
		stored++
		assert.True(t, top[tag], "unexpected stored field %q; extend the spec first", tag)
		assert.False(t, forbiddenAnywhere[tag])
	}
	assert.Equal(t, len(top), stored)

	wantCounters := map[string][]string{
		"queries":   {"total", "blocked", "dnssec"},
		"reasons":   {"blocklist", "service", "custom_rule", "rebinding", "default_rule", "other"},
		"protocols": {"doh", "dot", "doq"},
	}
	for name, tags := range wantCounters {
		f, ok := typ.FieldByNameFunc(func(n string) bool { f, _ := typ.FieldByName(n); return tagOf(f) == name })
		require.True(t, ok, name)
		var got []string
		for i := 0; i < f.Type.NumField(); i++ {
			assert.Equal(t, reflect.Int64, f.Type.Field(i).Type.Kind(), "%s.%s must be int64", name, f.Type.Field(i).Name)
			got = append(got, tagOf(f.Type.Field(i)))
		}
		assert.ElementsMatch(t, tags, got, name)
	}

	metaField, ok := typ.FieldByName("Meta")
	require.True(t, ok)
	var meta []string
	for i := 0; i < metaField.Type.NumField(); i++ {
		meta = append(meta, tagOf(metaField.Type.Field(i)))
	}
	assert.ElementsMatch(t, []string{"profile_id", "device_id"}, meta)
}

// specRef: proxy-statistics-behaviour.md #Y19
func TestStatistics_BSONWritesEveryCounterAsInt64(t *testing.T) {
	data, err := bson.Marshal(Statistics{
		BucketStart: time.Date(2026, 9, 17, 13, 30, 0, 0, time.UTC),
		Meta:        StatisticsMeta{ProfileID: "p1", DeviceID: ""},
		Retention:   "90d",
	})
	require.NoError(t, err)
	raw := bson.Raw(data)

	keys, err := raw.Elements()
	require.NoError(t, err)
	var names []string
	for _, e := range keys {
		names = append(names, e.Key())
	}
	assert.Equal(t, []string{"bucket_start", "meta", "queries", "reasons", "protocols"}, names)

	for path, want := range map[string][]string{
		"queries":   {"total", "blocked", "dnssec"},
		"reasons":   {"blocklist", "service", "custom_rule", "rebinding", "default_rule", "other"},
		"protocols": {"doh", "dot", "doq"},
	} {
		sub, ok := raw.Lookup(path).DocumentOK()
		require.True(t, ok, path)
		els, _ := sub.Elements()
		require.Len(t, els, len(want), path)
		for i, e := range els {
			assert.Equal(t, want[i], e.Key())
			_, isInt64 := e.Value().Int64OK()
			assert.True(t, isInt64, "%s.%s must be int64", path, e.Key())
		}
	}
	dev, ok := raw.Lookup("meta", "device_id").StringValueOK()
	require.True(t, ok, "an empty device id is stored, not omitted")
	assert.Empty(t, dev)
}
