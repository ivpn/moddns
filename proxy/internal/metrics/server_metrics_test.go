package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// counterValues returns every counter sample as "name{label=value,...}" -> value.
func counterValues(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	gathered, err := reg.Gather()
	require.NoError(t, err)

	got := map[string]float64{}
	for _, fam := range gathered {
		for _, mt := range fam.GetMetric() {
			if mt.GetCounter() == nil {
				continue
			}
			key := fam.GetName()
			for _, l := range mt.GetLabel() {
				key += "{" + l.GetName() + "=" + l.GetValue() + "}"
			}
			got[key] = mt.GetCounter().GetValue()
		}
	}
	return got
}

// specRef: proxy-request-admission-behaviour.md #Q16 #Q17
func TestServerMetrics_DoHCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewServerMetrics(reg)

	m.RecordDoHRequest("2")
	m.RecordDoHRequest("3")
	m.RecordDoHRequest("3")
	m.RecordDoH3ConnRejected("source")
	m.RecordDoH3Busy()

	got := counterValues(t, reg)
	assert.InDelta(t, 1, got["proxy_doh_requests_total{http_version=2}"], 0)
	assert.InDelta(t, 2, got["proxy_doh_requests_total{http_version=3}"], 0)
	assert.InDelta(t, 1, got["proxy_doh3_connections_rejected_total{reason=source}"], 0)
	assert.InDelta(t, 1, got["proxy_doh3_requests_busy_total"], 0)
}

// specRef: proxy-request-admission-behaviour.md #Q16
func TestServerMetrics_DoH3ListenerUp(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewServerMetrics(reg)

	listenerUp := func() (float64, bool) {
		gathered, err := reg.Gather()
		require.NoError(t, err)
		for _, fam := range gathered {
			if fam.GetName() == "proxy_doh3_listener_up" {
				return fam.GetMetric()[0].GetGauge().GetValue(), true
			}
		}
		return 0, false
	}

	_, exported := listenerUp()
	assert.False(t, exported, "absent until a DoH3 listener reports, so DoT/DoQ and h3-off containers do not export 0")

	m.SetDoH3ListenerUp(false)
	v, exported := listenerUp()
	require.True(t, exported)
	assert.InDelta(t, 0, v, 0)

	m.SetDoH3ListenerUp(true)
	v, _ = listenerUp()
	assert.InDelta(t, 1, v, 0)
}
