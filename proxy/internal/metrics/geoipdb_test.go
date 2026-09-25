package metrics

import (
	"testing"
	"time"

	"github.com/ivpn/dns/libs/geoipdb"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type fakeGeoIPDB struct{ stats geoipdb.Stats }

func (f fakeGeoIPDB) Stats() geoipdb.Stats { return f.stats }

func gaugeValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		m := mf.GetMetric()[0]
		if mf.GetType() == dto.MetricType_COUNTER {
			return m.GetCounter().GetValue()
		}
		return m.GetGauge().GetValue()
	}
	t.Fatalf("metric %s not registered", name)
	return 0
}

func TestObserveGeoIPDBExposesBuildAndReloadState(t *testing.T) {
	build := time.Date(2026, 9, 18, 13, 50, 42, 0, time.UTC)
	loaded := build.Add(2 * time.Hour)
	reg := prometheus.NewRegistry()
	ObserveGeoIPDB(reg, fakeGeoIPDB{stats: geoipdb.Stats{
		BuildTime: build,
		LoadedAt:  loaded,
		Reloads:   3,
		Failures:  1,
		LastError: "boom",
	}})

	if got := gaugeValue(t, reg, "proxy_dns_geoip_db_build_timestamp_seconds"); got != float64(build.Unix()) {
		t.Errorf("build timestamp gauge = %v, want %v", got, build.Unix())
	}
	if got := gaugeValue(t, reg, "proxy_dns_geoip_db_loaded_timestamp_seconds"); got != float64(loaded.Unix()) {
		t.Errorf("loaded timestamp gauge = %v, want %v", got, loaded.Unix())
	}
	if got := gaugeValue(t, reg, "proxy_dns_geoip_db_reloads_total"); got != 3 {
		t.Errorf("reloads counter = %v, want 3", got)
	}
	if got := gaugeValue(t, reg, "proxy_dns_geoip_db_reload_failures_total"); got != 1 {
		t.Errorf("failures counter = %v, want 1", got)
	}
	if got := gaugeValue(t, reg, "proxy_dns_geoip_db_reload_error"); got != 1 {
		t.Errorf("reload error gauge = %v, want 1", got)
	}
}
