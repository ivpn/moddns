package model

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// specRef: proxy-statistics-behaviour.md #Y5
func TestBucketStart(t *testing.T) {
	loc := time.FixedZone("CEST", 2*3600)
	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{name: "mid-hour is truncated to the hour", in: time.Date(2026, 9, 17, 13, 47, 59, 123456789, time.UTC), want: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)},
		{name: "exact hour is unchanged", in: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC), want: time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)},
		{name: "local time is normalised to UTC first", in: time.Date(2026, 9, 17, 1, 30, 0, 0, loc), want: time.Date(2026, 9, 16, 23, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BucketStart(tt.in)
			assert.True(t, got.Equal(tt.want), "got %s want %s", got, tt.want)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y5 #Y6
func TestNewServiceStatistics_KeyedByPopAndHour(t *testing.T) {
	doc := NewServiceStatistics("ams1", time.Date(2026, 9, 17, 13, 47, 1, 0, time.UTC))

	assert.Equal(t, "ams1:2026-09-17T13", doc.ID)
	assert.True(t, doc.Timestamp.Equal(time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)))
	assert.Equal(t, "ams1", doc.Pop)
	assert.Equal(t, Queries{}, doc.Queries)

	doc.Aggregate(EventStatistics{Queries: Queries{Total: 1, Blocked: 1}})
	doc.Aggregate(EventStatistics{Queries: Queries{Total: 1, DNSSEC: 1}})
	assert.Equal(t, Queries{Total: 2, Blocked: 1, DNSSEC: 1}, doc.Queries)
	assert.Equal(t, "ams1:2026-09-17T13", doc.ID, "aggregation never moves the document")
}

// specRef: proxy-statistics-behaviour.md #Y1 #Y6
func TestServiceStatistics_SchemaCarriesNoIdentifier(t *testing.T) {
	forbidden := map[string]bool{"profile_id": true, "device_id": true, "client_ip": true}
	allowed := map[string]bool{"_id": true, "timestamp": true, "pop": true, "queries": true}

	typ := reflect.TypeOf(ServiceStatistics{})
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.Split(typ.Field(i).Tag.Get("bson"), ",")[0]
		assert.False(t, forbidden[tag], "field %s must not be stored", tag)
		assert.True(t, allowed[tag], "unexpected stored field %q; extend the spec first", tag)
	}
	assert.Equal(t, len(allowed), typ.NumField())

	evt := reflect.TypeOf(EventStatistics{})
	assert.Equal(t, 1, evt.NumField(), "the per-query event carries counters only")
	assert.Equal(t, "Queries", evt.Field(0).Name)
}
