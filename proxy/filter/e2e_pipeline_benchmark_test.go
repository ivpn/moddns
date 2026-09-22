package filter

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	libscache "github.com/ivpn/dns/libs/cache"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const benchE2EProfile = "benche2eprofile1"

// startBenchRedis runs a throwaway Redis seeded with one realistic profile:
// 3 blocklists, 5 custom rules, subdomain blocking on, default rule allow.
func startBenchRedis(b *testing.B) (cache.Cache, *goredis.Client) {
	b.Helper()
	ctx := context.Background()
	redisC, err := tcredis.Run(ctx, "redis:7")
	if err != nil {
		b.Skipf("redis container unavailable: %v", err)
	}
	b.Cleanup(func() { _ = redisC.Terminate(ctx) })
	host, err := redisC.Host(ctx)
	require.NoError(b, err)
	port, err := redisC.MappedPort(ctx, "6379")
	require.NoError(b, err)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	b.Cleanup(func() { _ = rdb.Close() })
	k := "settings:" + benchE2EProfile
	require.NoError(b, rdb.HSet(ctx, k+":privacy", map[string]string{"default_rule": "allow", "blocklists_subdomains_rule": "block"}).Err())
	require.NoError(b, rdb.HSet(ctx, k+":logs", map[string]string{"enabled": "true"}).Err())
	require.NoError(b, rdb.HSet(ctx, k+":security:dnssec", map[string]string{"enabled": "true", "send_do_bit": "true"}).Err())
	require.NoError(b, rdb.HSet(ctx, k+":advanced", map[string]string{"recursor": "default"}).Err())
	require.NoError(b, rdb.HSet(ctx, k+":statistics", map[string]string{"enabled": "false"}).Err())
	require.NoError(b, rdb.RPush(ctx, k+":blocklists", "bl_ads", "bl_malware", "bl_tracking").Err())
	for _, bl := range []string{"bl_ads", "bl_malware", "bl_tracking"} {
		require.NoError(b, rdb.SAdd(ctx, "blocklist:"+bl, "blocked.example", "ads.tracker.example").Err())
	}
	rules := []map[string]string{
		{"value": "ads.example", "action": "block", "syntax": "domain"},
		{"value": "*.cdn.example", "action": "allow", "syntax": "domain"},
		{"value": "203.0.113.9", "action": "block", "syntax": "ip"},
		{"value": "AS64496", "action": "block", "syntax": "asn"},
		{"value": "*tracker*", "action": "block", "syntax": "domain"},
	}
	for i, r := range rules {
		h := fmt.Sprintf("%s:custom_rule:%d", k, i)
		require.NoError(b, rdb.HSet(ctx, h, r).Err())
		require.NoError(b, rdb.SAdd(ctx, k+":custom_rules", h).Err())
	}

	c, err := cache.NewRedisCache(&libscache.Config{Address: addr})
	require.NoError(b, err)
	b.Cleanup(c.Close)
	return c, rdb
}

// redisCalls sums cmdstat call counters from INFO commandstats.
func redisCalls(b *testing.B, rdb *goredis.Client) (total int64) {
	b.Helper()
	info, err := rdb.Info(context.Background(), "commandstats").Result()
	require.NoError(b, err)
	for _, line := range strings.Split(info, "\n") {
		if !strings.HasPrefix(line, "cmdstat_") {
			continue
		}
		for _, kv := range strings.Split(strings.SplitN(line, ":", 2)[1], ",") {
			if strings.HasPrefix(kv, "calls=") {
				n, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(kv, "calls=")), 10, 64)
				total += n
			}
		}
	}
	return total
}

func benchLogger() logging.LoggerInterface {
	return logging.NewDefaultFactory().ForRequest(logging.LoggingConfig{Enabled: false, ProfileID: benchE2EProfile})
}

// benchQuery is a no-match query: every stage runs to completion, blocklist
// membership walks 3 lists × 3 labels.
func benchQuery() *proxy.DNSContext {
	req := new(dns.Msg)
	req.SetQuestion("www.news.example.", dns.TypeA)
	res := new(dns.Msg)
	res.SetReply(req)
	res.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("198.51.100.7")}}
	return &proxy.DNSContext{Req: req, Res: res, Addr: netip.MustParseAddrPort("192.0.2.1:53"), Proto: proxy.ProtoUDP}
}

// BenchmarkE2EPipeline_RealRedis measures one full query through both filter
// phases against a real Redis. "warm" = settings already cached (the common
// case); "cold" = settings batch fetched first (cache miss). Reports Redis
// commands per query alongside ns/op.
func BenchmarkE2EPipeline_RealRedis(b *testing.B) {
	c, rdb := startBenchRedis(b)
	ctx := context.Background()
	domainF := NewDomainFilter(nil, c, nil)
	ipF := NewIPFilter(nil, c, nil, nil, nil, nil)
	settings, err := c.GetProfileSettingsBatch(ctx, benchE2EProfile)
	require.NoError(b, err)
	require.NoError(b, settings.StoreError())
	logger := benchLogger()

	b.Run("warm", func(b *testing.B) {
		require.NoError(b, rdb.ConfigResetStat(ctx).Err())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reqCtx := requestcontext.NewRequestContext(ctx, nil, benchE2EProfile, "", settings, logger)
			dctx := benchQuery()
			_ = domainF.Execute(ctx, reqCtx, dctx)
			_ = ipF.Execute(ctx, reqCtx, dctx)
		}
		b.StopTimer()
		b.ReportMetric(float64(redisCalls(b, rdb)-1)/float64(b.N), "redis-cmds/op")
	})
	b.Run("cold", func(b *testing.B) {
		require.NoError(b, rdb.ConfigResetStat(ctx).Err())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s, err := c.GetProfileSettingsBatch(ctx, benchE2EProfile)
			if err != nil {
				b.Fatal(err)
			}
			reqCtx := requestcontext.NewRequestContext(ctx, nil, benchE2EProfile, "", s, logger)
			dctx := benchQuery()
			_ = domainF.Execute(ctx, reqCtx, dctx)
			_ = ipF.Execute(ctx, reqCtx, dctx)
		}
		b.StopTimer()
		b.ReportMetric(float64(redisCalls(b, rdb)-1)/float64(b.N), "redis-cmds/op")
	})
}
