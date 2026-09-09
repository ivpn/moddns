package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	n     int
	bytes int64
	up    bool
}

func (f fakeSource) Len() int             { return f.n }
func (f fakeSource) Bytes() int64         { return f.bytes }
func (f fakeSource) StoreAvailable() bool { return f.up }

func TestSettingsCacheMetrics_Registration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewSettingsCacheMetrics(reg)
	src := &fakeSource{n: 3, bytes: 4096, up: true}
	ObserveSettingsCache(reg, src)

	m.RecordEviction("size")
	m.RecordEviction("size")
	m.RecordEviction("deleted")

	gathered, err := reg.Gather()
	require.NoError(t, err)
	got := map[string]float64{}
	for _, fam := range gathered {
		for _, mt := range fam.GetMetric() {
			switch fam.GetType() {
			case dto.MetricType_GAUGE:
				got[fam.GetName()] = mt.GetGauge().GetValue()
			case dto.MetricType_COUNTER:
				got[fam.GetName()+"{"+mt.GetLabel()[0].GetValue()+"}"] = mt.GetCounter().GetValue()
			}
		}
	}
	assert.Equal(t, 2.0, got["proxy_dns_profile_settings_cache_evictions_total{size}"])
	assert.Equal(t, 1.0, got["proxy_dns_profile_settings_cache_evictions_total{deleted}"])
	assert.Equal(t, 3.0, got["proxy_dns_profile_settings_cache_entries"])
	assert.Equal(t, 4096.0, got["proxy_dns_profile_settings_cache_bytes_estimate"])
	assert.Equal(t, 1.0, got["proxy_dns_settings_store_available"])

	src.up = false
	gathered, err = reg.Gather()
	require.NoError(t, err)
	for _, fam := range gathered {
		if fam.GetName() == "proxy_dns_settings_store_available" {
			assert.Equal(t, 0.0, fam.GetMetric()[0].GetGauge().GetValue(), "gauge reads live state at scrape")
		}
	}
}
