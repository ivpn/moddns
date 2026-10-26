package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/model"
)

// BenchmarkStatisticsCollector_AddFleetOnly is the collector's per-event cost for a
// non-consenting profile.
func BenchmarkStatisticsCollector_AddFleetOnly(b *testing.B) {
	c := &StatisticsCollector{Pop: "ams1", Now: func() time.Time { return t1300 }}
	ev := model.EventStatistics{Queries: model.Queries{Total: 1, Blocked: 1}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.add(ev)
	}
}

// BenchmarkStatisticsCollector_AddConsented is the per-event cost for a consenting
// profile with 1000 profiles and 8 devices each (two entries per pair, one per tier).
func BenchmarkStatisticsCollector_AddConsented(b *testing.B) {
	c := &StatisticsCollector{Pop: "ams1", MaxOpenEntries: 1_000_000, Now: func() time.Time { return t1300 }}
	events := make([]model.EventStatistics, 0, 1000*8)
	for p := 0; p < 1000; p++ {
		for d := 0; d < 8; d++ {
			events = append(events, cevt(fmt.Sprintf("profile%04d", p), fmt.Sprintf("device-%d", d),
				model.Queries{Total: 1, Blocked: 1}, model.ReasonClassBlocklist, model.ProtocolDoH))
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.add(events[i%len(events)])
	}
}
