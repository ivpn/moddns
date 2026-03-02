package ratelimit

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/rs/zerolog"
)

func benchLimiter() *RateLimiter {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	return New(Config{
		PerIPEnabled:      true,
		PerIPRate:         100,
		PerIPBurst:        200,
		PerProfileEnabled: true,
		PerProfileRate:    300,
		PerProfileBurst:   500,
	}, nil)
}

func BenchmarkCheckIP(b *testing.B) {
	rl := benchLimiter()
	addr := netip.MustParseAddr("192.0.2.1")
	b.ResetTimer()
	for range b.N {
		rl.CheckIP(addr, "udp")
	}
}

func BenchmarkCheckProfile(b *testing.B) {
	rl := benchLimiter()
	b.ResetTimer()
	for range b.N {
		rl.CheckProfile("profile123", "tls")
	}
}

func BenchmarkCheckIP_ManyIPs(b *testing.B) {
	rl := benchLimiter()
	addrs := make([]netip.Addr, 1024)
	for i := range addrs {
		addrs[i] = netip.MustParseAddr(fmt.Sprintf("10.%d.%d.%d", (i>>16)&0xff, (i>>8)&0xff, i&0xff))
	}
	b.ResetTimer()
	for i := range b.N {
		rl.CheckIP(addrs[i%len(addrs)], "udp")
	}
}

func BenchmarkCheckIP_Parallel(b *testing.B) {
	rl := benchLimiter()
	addr := netip.MustParseAddr("192.0.2.1")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.CheckIP(addr, "udp")
		}
	})
}
