package model

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// specRef: proxy-statistics-behaviour.md #Y5
func TestServiceStatisticsAggregate_SumsCounters(t *testing.T) {
	stamp := time.Date(2026, 9, 15, 13, 47, 1, 0, time.UTC)
	doc := ServiceStatistics{Timestamp: stamp, Pop: "ams1"}

	doc.Aggregate(EventStatistics{Queries: Queries{Total: 1, Blocked: 1}})
	doc.Aggregate(EventStatistics{Queries: Queries{Total: 1, DNSSEC: 1}})

	assert.Equal(t, Queries{Total: 2, Blocked: 1, DNSSEC: 1}, doc.Queries)
	assert.True(t, doc.Timestamp.Equal(stamp), "events carry no time; the stamp is set at flush")
	assert.Equal(t, "ams1", doc.Pop)
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
