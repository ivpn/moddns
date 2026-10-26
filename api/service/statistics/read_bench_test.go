package statistics_test

// Env-gated benchmark of the statistics read aggregation (spec row J47) on real
// time-series collections created by migration 027, written the way the proxy
// writes them: one additive measurement per (profile, device, writer) every 15
// minutes, stamped at the 15-minute, hour or day start of its tier. Run with:
//
//	BENCH_STATISTICS=1 go test ./service/statistics/ -run TestBenchmarkStatisticsRead -v
//	TEST_MONGO_IMAGE=mongo:7.0.8 BENCH_STATISTICS=1 go test ... (other server version; default is prod's 8.0.9)
//
// BENCH_NOISE=<n> other profiles share the collections (default 5); BENCH_WRITERS=<n> proxy instances (default 2).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ivpn/dns/api/db/mongodb"
	"github.com/ivpn/dns/api/model"
)

func benchEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func flatDoc(profile, device string, at time.Time, total int) bson.D {
	return bson.D{
		{Key: "bucket_start", Value: at},
		{Key: "meta", Value: bson.D{{Key: "profile_id", Value: profile}, {Key: "device_id", Value: device}}},
		{Key: "total", Value: total}, {Key: "blocked", Value: total / 5}, {Key: "dnssec", Value: total / 3},
		{Key: "reason_blocklist", Value: total / 10}, {Key: "reason_service", Value: total / 20}, {Key: "reason_custom_rule", Value: 1},
		{Key: "reason_rebinding", Value: 0}, {Key: "reason_default_rule", Value: 0}, {Key: "reason_other", Value: 0},
		{Key: "proto_doh", Value: total / 2}, {Key: "proto_dot", Value: total / 4}, {Key: "proto_doq", Value: total / 8},
	}
}

// specRef: api-endpoint-behaviour.md J47
func TestBenchmarkStatisticsRead(t *testing.T) {
	if os.Getenv("BENCH_STATISTICS") != "1" {
		t.Skip("set BENCH_STATISTICS=1 to run the statistics read benchmark")
	}
	ctx := context.Background()
	noise, _ := strconv.Atoi(benchEnv("BENCH_NOISE", "5"))
	writers, _ := strconv.Atoi(benchEnv("BENCH_WRITERS", "2"))

	username, password, authSource := benchEnv("TEST_MONGO_USERNAME", "testuser"), benchEnv("TEST_MONGO_PASSWORD", "testpass"), benchEnv("DB_AUTH_SOURCE", "admin")
	image := benchEnv("TEST_MONGO_IMAGE", "mongo:8.0.9")
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			Env:          map[string]string{"MONGO_INITDB_ROOT_USERNAME": username, "MONGO_INITDB_ROOT_PASSWORD": password},
			ExposedPorts: []string{"27017/tcp"},
			WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer func() { _ = container.Terminate(ctx) }()
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "27017/tcp")
	require.NoError(t, err)
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource}))
	require.NoError(t, err)

	dbName := "dns_statistics_bench"
	_ = client.Database(dbName).Drop(ctx)
	raw, err := os.ReadFile("../../db/mongodb/migrations/027_statistics_retention_collections.up.json")
	require.NoError(t, err)
	var cmds []json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &cmds))
	for _, c := range cmds {
		var cmd bson.D
		require.NoError(t, bson.UnmarshalExtJSON(c, false, &cmd))
		require.NoError(t, client.Database(dbName).RunCommand(ctx, cmd).Err())
	}
	db := client.Database(dbName)

	const target = "benchprofile"
	profiles := []string{target}
	for i := 0; i < noise; i++ {
		profiles = append(profiles, fmt.Sprintf("noise%03d", i))
	}
	devices := []string{"laptop", "phone", "tv", "router", "tablet"}
	now := time.Now().UTC().Truncate(time.Hour).Add(13 * time.Minute) // mid-hour, like a request

	// seed writes one measurement per profile, device and writer at every closed
	// step of period over the last `days` days into coll, stamped at its tier bucket.
	// The 1h and 1d tiers get a partial measurement per quarter (proxy-statistics-behaviour.md Y28).
	seed := func(coll string, days int, period time.Duration, stamp func(time.Time) time.Time, ps []string) int {
		c := db.Collection(coll)
		batch := make([]any, 0, 10_000)
		n := 0
		flush := func() {
			if len(batch) > 0 {
				_, err := c.InsertMany(ctx, batch)
				require.NoError(t, err)
				batch = batch[:0]
			}
		}
		end := now.Truncate(period)
		for i := 0; i < int(time.Duration(days)*24*time.Hour/period); i++ {
			closed := end.Add(-time.Duration(i+1) * period)
			for _, p := range ps {
				for d, dev := range devices {
					for w := 0; w < writers; w++ {
						batch = append(batch, flatDoc(p, dev, stamp(closed), 5+(i*7+d*13+w)%40))
						n++
						if len(batch) == cap(batch) {
							flush()
						}
					}
				}
			}
		}
		flush()
		return n
	}
	same := func(t time.Time) time.Time { return t }
	hour := func(t time.Time) time.Time { return t.Truncate(time.Hour) }
	day := func(t time.Time) time.Time { return t.Truncate(24 * time.Hour) }
	const quarter = 15 * time.Minute
	start := time.Now()
	n15 := seed("statistics_15min", 1, quarter, same, profiles)
	n1h := seed("statistics_1h", 8, quarter, hour, profiles)
	n1d := seed("statistics_1d_1y", 365, quarter, day, profiles[:1]) // the target holds a full year
	n1d += seed("statistics_1d_90d", 90, quarter, day, profiles[1:])
	n1d += seed("statistics_1d_30d", 30, quarter, day, profiles[1:])
	t.Logf("%s: seeded %d (15min) + %d (1h) + %d (1d) measurements, %d profiles, %d writers in %s", image, n15, n1h, n1d, len(profiles), writers, time.Since(start).Round(time.Millisecond))

	repo := mongodb.NewStatisticsRepository(client, dbName)
	views := []string{model.StatisticsLast3Hours, model.StatisticsLast6Hours, model.LAST_1_DAY, model.LAST_7_DAYS, model.LAST_MONTH, model.StatisticsLast3Months, model.StatisticsLastYear}
	for _, view := range views {
		spec, err := model.NewStatisticsTimespan(view)
		require.NoError(t, err)
		to := now
		from := to.Add(-spec.Window).Truncate(spec.Bucket) // no retention clamp: the repository is called with the full window
		var durs []time.Duration
		points := 0
		for i := 0; i < 21; i++ {
			st := time.Now()
			agg, err := repo.GetProfileStatistics(ctx, target, spec.Tier, from, to, spec.Bucket)
			d := time.Since(st)
			require.NoError(t, err)
			require.NotZero(t, agg.Totals.Total, view)
			points = len(agg.Series)
			if i > 0 { // run 0 is the cold run
				durs = append(durs, d)
			}
		}
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
		t.Logf("RESULT %s %-14s tier=%-5s points=%-3d p50=%s p95=%s", image, view, spec.Tier, points, durs[len(durs)/2].Round(time.Millisecond), durs[len(durs)*95/100].Round(time.Millisecond))
	}
}
