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
func TestStatisticsTier_BucketStart(t *testing.T) {
	assert.Equal(t, 15*time.Minute, StatisticsBucket15Min)
	assert.Equal(t, time.Hour, StatisticsBucket1Hour)
	loc := time.FixedZone("CEST", 2*3600)
	at := time.Date(2026, 9, 17, 13, 44, 59, 9, time.UTC)
	tests := []struct {
		name string
		tier StatisticsTier
		in   time.Time
		want time.Time
	}{
		{name: "15min truncated to the quarter", tier: StatisticsTier15Min, in: at, want: time.Date(2026, 9, 17, 13, 30, 0, 0, time.UTC)},
		{name: "15min exact boundary", tier: StatisticsTier15Min, in: time.Date(2026, 9, 17, 13, 45, 0, 0, time.UTC), want: time.Date(2026, 9, 17, 13, 45, 0, 0, time.UTC)},
		{name: "15min local time normalised to UTC", tier: StatisticsTier15Min, in: time.Date(2026, 9, 17, 1, 20, 0, 0, loc), want: time.Date(2026, 9, 16, 23, 15, 0, 0, time.UTC)},
		{name: "1h truncated to the hour", tier: StatisticsTier1Hour, in: at, want: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)},
		{name: "1d truncated to the UTC day", tier: StatisticsTier1Day, in: at, want: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)},
		{name: "1d local time normalised to UTC", tier: StatisticsTier1Day, in: time.Date(2026, 9, 17, 1, 20, 0, 0, loc), want: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tier.BucketStart(tt.in)
			assert.True(t, got.Equal(tt.want), "got %s want %s", got, tt.want)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y17 #Y20
func TestStatisticsTier_StoredValuesAreStable(t *testing.T) {
	assert.Equal(t, StatisticsTier("15min"), StatisticsTier15Min)
	assert.Equal(t, StatisticsTier("1h"), StatisticsTier1Hour)
	assert.Equal(t, StatisticsTier("1d"), StatisticsTier1Day)
}

// specRef: proxy-statistics-behaviour.md #Y17
func TestStatistics_ForDayRestampsTheHourDocument(t *testing.T) {
	hour := Statistics{
		BucketStart: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC),
		Meta:        StatisticsMeta{ProfileID: "p1", DeviceID: "d1"},
		Tier:        StatisticsTier1Hour,
		Retention:   StatisticsRetention90d,
		Total:       7, Blocked: 2, ReasonBlocklist: 2, ProtoDoH: 7,
	}

	day := hour.ForDay()

	assert.Equal(t, StatisticsTier1Day, day.Tier)
	assert.True(t, day.BucketStart.Equal(time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, hour.Meta, day.Meta)
	assert.Equal(t, StatisticsRetention90d, day.Retention)
	assert.Equal(t, int64(7), day.Total)
	assert.Equal(t, int64(2), day.ReasonBlocklist)
	assert.Equal(t, StatisticsTier1Hour, hour.Tier, "the hour document is untouched")
}

// specRef: proxy-statistics-behaviour.md #Y14
func TestStatistics_Count(t *testing.T) {
	doc := Statistics{}
	doc.Count(Queries{Total: 1, Blocked: 1}, &ConsentedStatistics{ReasonClass: ReasonClassBlocklist, Protocol: ProtocolDoH})
	doc.Count(Queries{Total: 1, DNSSEC: 1}, &ConsentedStatistics{Protocol: ProtocolDoT})
	doc.Count(Queries{Total: 1, Blocked: 1}, &ConsentedStatistics{ReasonClass: ReasonClassOther, Protocol: ProtocolDoQ})
	doc.Count(Queries{Total: 1}, &ConsentedStatistics{})

	assert.Equal(t, int64(4), doc.Total)
	assert.Equal(t, int64(2), doc.Blocked)
	assert.Equal(t, int64(1), doc.DNSSEC)
	assert.Equal(t, int64(1), doc.ReasonBlocklist)
	assert.Equal(t, int64(1), doc.ReasonOther)
	assert.Zero(t, doc.ReasonService+doc.ReasonCustomRule+doc.ReasonRebinding+doc.ReasonDefaultRule)
	assert.Equal(t, int64(1), doc.ProtoDoH)
	assert.Equal(t, int64(1), doc.ProtoDoT)
	assert.Equal(t, int64(1), doc.ProtoDoQ)
}

// specRef: proxy-statistics-behaviour.md #Y15
func TestOtherDeviceID_CannotBeANormalisedDeviceID(t *testing.T) {
	assert.Equal(t, 64, MaxDevicesPerProfileBucket)
	assert.Equal(t, "_other", OtherDeviceID)
	assert.True(t, strings.Contains(OtherDeviceID, "_"), "underscore is outside [A-Za-z0-9 -]")
}

var statisticsCounterKeys = []string{"total", "blocked", "dnssec", "reason_blocklist", "reason_service", "reason_custom_rule", "reason_rebinding", "reason_default_rule", "reason_other", "proto_doh", "proto_dot", "proto_doq"}

// specRef: proxy-statistics-behaviour.md #Y1 #Y19
func TestStatistics_SchemaShape(t *testing.T) {
	forbidden := map[string]bool{"timestamp": true, "client_ip": true, "created_at": true, "flushed_at": true}
	tagOf := func(f reflect.StructField) string { return strings.Split(f.Tag.Get("bson"), ",")[0] }

	want := map[string]bool{"bucket_start": true, "meta": true}
	for _, k := range statisticsCounterKeys {
		want[k] = true
	}
	unstored := map[string]bool{"Retention": true, "Tier": true}

	typ := reflect.TypeOf(Statistics{})
	stored := 0
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := tagOf(f)
		if tag == "-" {
			assert.True(t, unstored[f.Name], "only routing keys may be unstored, got %s", f.Name)
			continue
		}
		stored++
		assert.True(t, want[tag], "unexpected stored field %q; extend the spec first", tag)
		assert.False(t, forbidden[tag])
		if tag != "bucket_start" && tag != "meta" {
			assert.Equal(t, reflect.Int64, f.Type.Kind(), "%s must be int64", tag)
		}
	}
	assert.Equal(t, len(want), stored)

	metaField, ok := typ.FieldByName("Meta")
	require.True(t, ok)
	var meta []string
	for i := 0; i < metaField.Type.NumField(); i++ {
		meta = append(meta, tagOf(metaField.Type.Field(i)))
	}
	assert.ElementsMatch(t, []string{"profile_id", "device_id"}, meta)
}

// specRef: proxy-statistics-behaviour.md #Y19
func TestStatistics_BSONWritesFlatInt64Counters(t *testing.T) {
	data, err := bson.Marshal(Statistics{
		BucketStart: time.Date(2026, 9, 17, 13, 30, 0, 0, time.UTC),
		Meta:        StatisticsMeta{ProfileID: "p1", DeviceID: ""},
		Tier:        StatisticsTier1Hour,
		Retention:   StatisticsRetention90d,
	})
	require.NoError(t, err)
	raw := bson.Raw(data)

	els, err := raw.Elements()
	require.NoError(t, err)
	var names []string
	for _, e := range els {
		names = append(names, e.Key())
	}
	assert.Equal(t, append([]string{"bucket_start", "meta"}, statisticsCounterKeys...), names)

	for _, k := range statisticsCounterKeys {
		_, isInt64 := raw.Lookup(k).Int64OK()
		assert.True(t, isInt64, "%s must be int64", k)
	}
	dev, ok := raw.Lookup("meta", "device_id").StringValueOK()
	require.True(t, ok, "an empty device id is stored, not omitted")
	assert.Empty(t, dev)
}

// specRef: proxy-statistics-behaviour.md #Y20
func TestStatisticsRetention_StoredValuesAreStable(t *testing.T) {
	assert.Equal(t, StatisticsRetention("30d"), StatisticsRetention30d)
	assert.Equal(t, StatisticsRetention("90d"), StatisticsRetention90d)
	assert.Equal(t, StatisticsRetention("1y"), StatisticsRetention1y)
}

// specRef: proxy-statistics-behaviour.md #Y12
func TestReasonClass_StoredValuesAreStable(t *testing.T) {
	assert.Equal(t, ReasonClass("blocklist"), ReasonClassBlocklist)
	assert.Equal(t, ReasonClass("service"), ReasonClassService)
	assert.Equal(t, ReasonClass("custom_rule"), ReasonClassCustomRule)
	assert.Equal(t, ReasonClass("rebinding"), ReasonClassRebinding)
	assert.Equal(t, ReasonClass("default_rule"), ReasonClassDefaultRule)
	assert.Equal(t, ReasonClass("other"), ReasonClassOther)
}
