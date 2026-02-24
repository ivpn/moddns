package ratelimit

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLimiter(t *testing.T, cfg Config) (*RateLimiter, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	rl := New(cfg, reg)
	return rl, reg
}

func counterValue(t *testing.T, reg *prometheus.Registry, layer, proto string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "dns_ratelimited_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := m.GetLabel()
			lm := make(map[string]string, len(labels))
			for _, l := range labels {
				lm[l.GetName()] = l.GetValue()
			}
			if lm["layer"] == layer && lm["proto"] == proto {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func TestDisabled(t *testing.T) {
	rl, _ := newTestLimiter(t, Config{Enabled: false, PerIPRate: 1, PerIPBurst: 1, PerProfileRate: 1, PerProfileBurst: 1})
	addr := netip.MustParseAddr("192.0.2.1")

	for range 1000 {
		assert.True(t, rl.CheckIP(addr, "udp"))
		assert.True(t, rl.CheckProfile("prof1", "udp"))
	}
}

func TestCheckIP_UnderLimit(t *testing.T) {
	rl, _ := newTestLimiter(t, Config{Enabled: true, PerIPRate: 100, PerIPBurst: 100, PerProfileRate: 100, PerProfileBurst: 100})
	addr := netip.MustParseAddr("192.0.2.1")

	// First burst of requests up to burst size should all pass.
	for i := range 100 {
		assert.True(t, rl.CheckIP(addr, "udp"), "request %d should pass", i)
	}
}

func TestCheckIP_OverLimit(t *testing.T) {
	rl, reg := newTestLimiter(t, Config{Enabled: true, PerIPRate: 5, PerIPBurst: 5, PerProfileRate: 100, PerProfileBurst: 100})
	addr := netip.MustParseAddr("192.0.2.1")

	// Exhaust the burst.
	for range 5 {
		rl.CheckIP(addr, "udp")
	}

	// Next request should be rejected.
	assert.False(t, rl.CheckIP(addr, "udp"))
	assert.Equal(t, float64(1), counterValue(t, reg, "ip", "udp"))
}

func TestCheckProfile_OverLimit(t *testing.T) {
	rl, reg := newTestLimiter(t, Config{Enabled: true, PerIPRate: 100, PerIPBurst: 100, PerProfileRate: 3, PerProfileBurst: 3})

	for range 3 {
		rl.CheckProfile("prof1", "tls")
	}

	assert.False(t, rl.CheckProfile("prof1", "tls"))
	assert.Equal(t, float64(1), counterValue(t, reg, "profile", "tls"))
}

func TestIndependentBuckets(t *testing.T) {
	rl, _ := newTestLimiter(t, Config{Enabled: true, PerIPRate: 2, PerIPBurst: 2, PerProfileRate: 2, PerProfileBurst: 2})

	ip1 := netip.MustParseAddr("192.0.2.1")
	ip2 := netip.MustParseAddr("192.0.2.2")

	// Exhaust ip1's bucket.
	for range 2 {
		rl.CheckIP(ip1, "udp")
	}
	assert.False(t, rl.CheckIP(ip1, "udp"))

	// ip2 should be unaffected.
	assert.True(t, rl.CheckIP(ip2, "udp"))
}

func TestPrometheusLabels(t *testing.T) {
	rl, reg := newTestLimiter(t, Config{Enabled: true, PerIPRate: 1, PerIPBurst: 1, PerProfileRate: 1, PerProfileBurst: 1})

	addr := netip.MustParseAddr("192.0.2.1")
	rl.CheckIP(addr, "https")
	rl.CheckIP(addr, "https") // over limit

	rl.CheckProfile("p1", "quic")
	rl.CheckProfile("p1", "quic") // over limit

	assert.Equal(t, float64(1), counterValue(t, reg, "ip", "https"))
	assert.Equal(t, float64(1), counterValue(t, reg, "profile", "quic"))
	// Different proto should be zero.
	assert.Equal(t, float64(0), counterValue(t, reg, "ip", "udp"))
}

func TestBurstAllowance(t *testing.T) {
	// Rate=1/s but burst=10 — should allow 10 immediate requests.
	rl, _ := newTestLimiter(t, Config{Enabled: true, PerIPRate: 1, PerIPBurst: 10, PerProfileRate: 1, PerProfileBurst: 10})
	addr := netip.MustParseAddr("192.0.2.1")

	for i := range 10 {
		assert.True(t, rl.CheckIP(addr, "udp"), "burst request %d should pass", i)
	}
	assert.False(t, rl.CheckIP(addr, "udp"), "should reject after burst exhausted")
}

func TestCounterIncrements(t *testing.T) {
	rl, reg := newTestLimiter(t, Config{Enabled: true, PerIPRate: 1, PerIPBurst: 1, PerProfileRate: 1, PerProfileBurst: 1})
	addr := netip.MustParseAddr("10.0.0.1")

	// First passes, next 5 fail.
	for range 6 {
		rl.CheckIP(addr, "tcp")
	}

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range mfs {
		if mf.GetName() != "dns_ratelimited_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := labelMap(m.GetLabel())
			if labels["layer"] == "ip" && labels["proto"] == "tcp" {
				assert.Equal(t, float64(5), m.GetCounter().GetValue())
				found = true
			}
		}
	}
	assert.True(t, found, "expected prometheus metric not found")
}

func labelMap(labels []*dto.LabelPair) map[string]string {
	m := make(map[string]string, len(labels))
	for _, l := range labels {
		m[l.GetName()] = l.GetValue()
	}
	return m
}

func TestCheckIP_ManyIPs(t *testing.T) {
	rl, _ := newTestLimiter(t, Config{Enabled: true, PerIPRate: 10, PerIPBurst: 10, PerProfileRate: 100, PerProfileBurst: 100})

	for i := range 256 {
		addr := netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", i))
		assert.True(t, rl.CheckIP(addr, "udp"))
	}
}
