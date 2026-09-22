package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// SettingsCacheSource is what the gauges read; *settingscache.Cache satisfies it.
type SettingsCacheSource interface {
	Len() int
	Bytes() int64
	StoreAvailable() bool
}

// SettingsCacheMetrics exposes the state of the in-process profile settings
// cache and the settings-store breaker. Gauges are read at scrape time from
// counters the cache maintains incrementally, so nothing runs per query.
type SettingsCacheMetrics struct {
	evictions *prometheus.CounterVec
}

// NewSettingsCacheMetrics registers the eviction counter. The gauges need the
// cache instance and are registered separately by ObserveSettingsCache.
func NewSettingsCacheMetrics(reg prometheus.Registerer) *SettingsCacheMetrics {
	m := &SettingsCacheMetrics{
		evictions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proxy_dns_profile_settings_cache_evictions_total",
			Help: "Profile settings cache entries removed, by reason: size (LRU capacity) or deleted (store reported the profile gone).",
		}, []string{"reason"}),
	}
	reg.MustRegister(m.evictions)
	return m
}

// RecordEviction counts one removed entry; matches settingscache's eviction hook.
func (m *SettingsCacheMetrics) RecordEviction(reason string) {
	m.evictions.WithLabelValues(reason).Inc()
}

// ObserveSettingsCache registers gauges that read src at scrape time.
func ObserveSettingsCache(reg prometheus.Registerer, src SettingsCacheSource) {
	reg.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_profile_settings_cache_entries",
			Help: "Profiles currently held in the in-process settings cache (fresh and stale).",
		}, func() float64 { return float64(src.Len()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_profile_settings_cache_bytes_estimate",
			Help: "Estimated bytes retained by the settings cache; a trend, not heap accounting.",
		}, func() float64 { return float64(src.Bytes()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_settings_store_available",
			Help: "1 when the settings store is believed reachable, 0 while the store breaker is open.",
		}, func() float64 {
			if src.StoreAvailable() {
				return 1
			}
			return 0
		}),
	)
}
