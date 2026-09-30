package metrics

import (
	"github.com/ivpn/dns/libs/geoipdb"
	"github.com/prometheus/client_golang/prometheus"
)

// GeoIPDBSource is what the GeoIP gauges read; *asnlookup.Lookup satisfies it.
type GeoIPDBSource interface {
	Stats() geoipdb.Stats
}

// ObserveGeoIPDB registers gauges describing the GeoLite2-ASN database in use.
// The build timestamp is the end-to-end freshness signal: it only moves when
// the on-disk file was refreshed and the process picked it up.
func ObserveGeoIPDB(reg prometheus.Registerer, src GeoIPDBSource) {
	reg.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_geoip_db_build_timestamp_seconds",
			Help: "Build time (Unix seconds) of the GeoLite2-ASN database currently serving lookups.",
		}, func() float64 { return float64(src.Stats().BuildTime.Unix()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_geoip_db_loaded_timestamp_seconds",
			Help: "Time (Unix seconds) the current GeoLite2-ASN file was opened by this process.",
		}, func() float64 { return float64(src.Stats().LoadedAt.Unix()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "proxy_dns_geoip_db_reloads_total",
			Help: "Successful in-process reloads of the GeoLite2-ASN database.",
		}, func() float64 { return float64(src.Stats().Reloads) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "proxy_dns_geoip_db_reload_failures_total",
			Help: "Replacement GeoLite2-ASN files rejected (unreadable, corrupt or wrong edition); the previous database kept serving.",
		}, func() float64 { return float64(src.Stats().Failures) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "proxy_dns_geoip_db_reload_error",
			Help: "1 while the most recent reload attempt failed, 0 after a successful load.",
		}, func() float64 {
			if src.Stats().LastError != "" {
				return 1
			}
			return 0
		}),
	)
}
