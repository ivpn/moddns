package ratelimit

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingMetrics captures rejection counts per (layer, proto) pair.
type recordingMetrics struct {
	rejections map[string]int
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{rejections: make(map[string]int)}
}

func (m *recordingMetrics) RecordRejection(layer, proto string) {
	m.rejections[layer+"/"+proto]++
}

func (m *recordingMetrics) count(layer, proto string) int {
	return m.rejections[layer+"/"+proto]
}

func newTestLimiter(cfg Config) (*RateLimiter, *recordingMetrics) {
	m := newRecordingMetrics()
	rl := New(cfg, m)
	return rl, m
}

func TestDisabled(t *testing.T) {
	rl, _ := newTestLimiter(Config{PerIPEnabled: false, PerIPRate: 1, PerIPBurst: 1, PerProfileEnabled: false, PerProfileRate: 1, PerProfileBurst: 1})
	addr := netip.MustParseAddr("192.0.2.1")

	for range 1000 {
		assert.True(t, rl.CheckIP(addr, "udp"))
		assert.True(t, rl.CheckProfile("prof1", "udp"))
	}
}

func TestIPDisabledProfileEnabled(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: false, PerIPRate: 1, PerIPBurst: 1, PerProfileEnabled: true, PerProfileRate: 3, PerProfileBurst: 3})
	addr := netip.MustParseAddr("192.0.2.1")

	// IP checks always pass when disabled.
	for range 100 {
		assert.True(t, rl.CheckIP(addr, "udp"))
	}

	// Profile checks still enforce limits.
	for range 3 {
		rl.CheckProfile("prof1", "udp")
	}
	assert.False(t, rl.CheckProfile("prof1", "udp"))
	assert.Equal(t, 1, m.count("profile", "udp"))
	assert.Equal(t, 0, m.count("ip", "udp"))
}

func TestIPEnabledProfileDisabled(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: true, PerIPRate: 3, PerIPBurst: 3, PerProfileEnabled: false, PerProfileRate: 1, PerProfileBurst: 1})
	addr := netip.MustParseAddr("192.0.2.1")

	// Profile checks always pass when disabled.
	for range 100 {
		assert.True(t, rl.CheckProfile("prof1", "udp"))
	}

	// IP checks still enforce limits.
	for range 3 {
		rl.CheckIP(addr, "udp")
	}
	assert.False(t, rl.CheckIP(addr, "udp"))
	assert.Equal(t, 1, m.count("ip", "udp"))
	assert.Equal(t, 0, m.count("profile", "udp"))
}

func TestCheckIP_UnderLimit(t *testing.T) {
	rl, _ := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 100, PerIPBurst: 100, PerProfileRate: 100, PerProfileBurst: 100})
	addr := netip.MustParseAddr("192.0.2.1")

	// First burst of requests up to burst size should all pass.
	for i := range 100 {
		assert.True(t, rl.CheckIP(addr, "udp"), "request %d should pass", i)
	}
}

func TestCheckIP_OverLimit(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 5, PerIPBurst: 5, PerProfileRate: 100, PerProfileBurst: 100})
	addr := netip.MustParseAddr("192.0.2.1")

	// Exhaust the burst.
	for range 5 {
		rl.CheckIP(addr, "udp")
	}

	// Next request should be rejected.
	assert.False(t, rl.CheckIP(addr, "udp"))
	assert.Equal(t, 1, m.count("ip", "udp"))
}

func TestCheckProfile_OverLimit(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 100, PerIPBurst: 100, PerProfileRate: 3, PerProfileBurst: 3})

	for range 3 {
		rl.CheckProfile("prof1", "tls")
	}

	assert.False(t, rl.CheckProfile("prof1", "tls"))
	assert.Equal(t, 1, m.count("profile", "tls"))
}

func TestIndependentBuckets(t *testing.T) {
	rl, _ := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 2, PerIPBurst: 2, PerProfileRate: 2, PerProfileBurst: 2})

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

func TestMetricsLabels(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 1, PerIPBurst: 1, PerProfileRate: 1, PerProfileBurst: 1})

	addr := netip.MustParseAddr("192.0.2.1")
	rl.CheckIP(addr, "https")
	rl.CheckIP(addr, "https") // over limit

	rl.CheckProfile("p1", "quic")
	rl.CheckProfile("p1", "quic") // over limit

	assert.Equal(t, 1, m.count("ip", "https"))
	assert.Equal(t, 1, m.count("profile", "quic"))
	// Different proto should be zero.
	assert.Equal(t, 0, m.count("ip", "udp"))
}

func TestBurstAllowance(t *testing.T) {
	// Rate=1/s but burst=10 — should allow 10 immediate requests.
	rl, _ := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 1, PerIPBurst: 10, PerProfileRate: 1, PerProfileBurst: 10})
	addr := netip.MustParseAddr("192.0.2.1")

	for i := range 10 {
		assert.True(t, rl.CheckIP(addr, "udp"), "burst request %d should pass", i)
	}
	assert.False(t, rl.CheckIP(addr, "udp"), "should reject after burst exhausted")
}

func TestCounterIncrements(t *testing.T) {
	rl, m := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 1, PerIPBurst: 1, PerProfileRate: 1, PerProfileBurst: 1})
	addr := netip.MustParseAddr("10.0.0.1")

	// First passes, next 5 fail.
	for range 6 {
		rl.CheckIP(addr, "tcp")
	}

	assert.Equal(t, 5, m.count("ip", "tcp"))
}

func TestCheckIP_ManyIPs(t *testing.T) {
	rl, _ := newTestLimiter(Config{PerIPEnabled: true, PerProfileEnabled: true, PerIPRate: 10, PerIPBurst: 10, PerProfileRate: 100, PerProfileBurst: 100})

	for i := range 256 {
		addr := netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", i))
		assert.True(t, rl.CheckIP(addr, "udp"))
	}
}
