package querylogs

// Env-gated benchmark for the top-domains and top-clients aggregations against
// a realistically sized 1-month retention collection. Not part of the regular
// suite — run with:
//
//	BENCH_QUERY_LOGS_TOP=1 go test ./service/query_logs/ -run TestBenchmarkQueryLogsTop -v
//	BENCH_DOCS=3000000 BENCH_CLIENTS=500 BENCH_QUERY_LOGS_TOP=1 go test ... (overrides)
//
// Reports p50/p95 over BENCH_RUNS runs (default 15) per aggregation and
// timespan. The first run of each series is cold.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ivpn/dns/api/db/mongodb"
	"github.com/ivpn/dns/api/model"
)

func TestBenchmarkQueryLogsTop(t *testing.T) {
	if os.Getenv("BENCH_QUERY_LOGS_TOP") != "1" {
		t.Skip("set BENCH_QUERY_LOGS_TOP=1 to run the top-lists benchmark")
	}
	ctx := context.Background()

	mongoImage := firstNonEmpty(os.Getenv("TEST_MONGO_IMAGE"), "mongo:8.0.9")
	username := firstNonEmpty(os.Getenv("TEST_MONGO_USERNAME"), "testuser")
	password := firstNonEmpty(os.Getenv("TEST_MONGO_PASSWORD"), "testpass")
	authSource := firstNonEmpty(os.Getenv("DB_AUTH_SOURCE"), "admin")
	req := testcontainers.ContainerRequest{
		Image: mongoImage,
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": username,
			"MONGO_INITDB_ROOT_PASSWORD": password,
		},
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	defer func() { _ = container.Terminate(ctx) }()

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "27017/tcp")
	require.NoError(t, err)
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource}))
	require.NoError(t, err)

	dbName := "dns_query_logs_top_bench"
	_ = client.Database(dbName).Drop(ctx)
	repo := mongodb.NewQueryLogsRepository(client, dbName, "query_logs")
	service := NewQueryLogsService(&repo)
	profileID := primitive.NewObjectID().Hex()

	// A REAL time-series collection with the proxy's exact options (see
	// devices_bench_test.go); the auto-created {profile_id, timestamp} index is
	// the only one, matching what the aggregations can rely on.
	tsOpts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("timestamp").
			SetMetaField("profile_id").
			SetGranularity("seconds"),
	).SetExpireAfterSeconds(2592000)
	require.NoError(t, client.Database(dbName).CreateCollection(ctx, "query_logs_1m", tsOpts))
	coll := client.Database(dbName).Collection("query_logs_1m")

	docCount := benchEnvInt("BENCH_DOCS", 1_000_000)
	clientCount := benchEnvInt("BENCH_CLIENTS", 200)
	runs := benchEnvInt("BENCH_RUNS", 15)

	// Seed: docs spread across 30 days; 20% blocked; 5000 distinct domains with
	// a skew (a few hot ones); clients skewed towards the first few IPs.
	const batchSize = 10_000
	now := time.Now()
	seedStart := time.Now()
	batch := make([]any, 0, batchSize)
	for i := 0; i < docCount; i++ {
		status := "processed"
		if i%5 == 0 {
			status = "blocked"
		}
		domainIdx := i % 5000
		if i%3 == 0 {
			domainIdx = i % 50 // hot domains
		}
		clientIdx := i % clientCount
		if i%2 == 0 {
			clientIdx = i % 5 // busy clients
		}
		ts := now.Add(-time.Duration(i%(30*24*60)) * time.Minute)
		batch = append(batch, bson.D{
			{Key: "timestamp", Value: ts},
			{Key: "profile_id", Value: profileID},
			{Key: "device_id", Value: "device-00"},
			{Key: "status", Value: status},
			{Key: "reasons", Value: bson.A{}},
			{Key: "dns_request", Value: bson.D{
				{Key: "domain", Value: fmt.Sprintf("host-%d.example.com", domainIdx)},
				{Key: "query_type", Value: "A"},
				{Key: "response_code", Value: "NOERROR"},
				{Key: "dnssec", Value: false},
			}},
			{Key: "client_ip", Value: fmt.Sprintf("10.%d.%d.%d", clientIdx/65536, (clientIdx/256)%256, clientIdx%256)},
			{Key: "protocol", Value: "udp"},
		})
		if len(batch) == batchSize {
			_, err := coll.InsertMany(ctx, batch)
			require.NoError(t, err)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		_, err := coll.InsertMany(ctx, batch)
		require.NoError(t, err)
	}
	t.Logf("seeded %d docs (5000 domains, %d clients, 20%% blocked) in %s", docCount, clientCount, time.Since(seedStart).Round(time.Millisecond))

	series := func(name string, fn func() error) {
		durs := make([]time.Duration, 0, runs)
		for i := 0; i < runs; i++ {
			start := time.Now()
			require.NoError(t, fn())
			durs = append(durs, time.Since(start))
		}
		cold := durs[0]
		sort.Slice(durs, func(a, b int) bool { return durs[a] < durs[b] })
		p := func(q float64) time.Duration { return durs[int(float64(len(durs)-1)*q+0.5)].Round(time.Millisecond) }
		t.Logf("%-48s cold=%8s p50=%8s p95=%8s", name, cold.Round(time.Millisecond), p(0.50), p(0.95))
	}

	for _, ts := range []string{model.LAST_1_DAY, model.LAST_7_DAYS, model.LAST_MONTH} {
		for _, kind := range []string{model.QueryLogTopKindBlocked, model.QueryLogTopKindResolved} {
			series(fmt.Sprintf("top domains %-8s %s (limit 50)", kind, ts), func() error {
				items, err := service.GetProfileQueryLogTopDomains(ctx, profileID, model.RetentionOneMonth, ts, kind, 50)
				if err == nil && len(items) != 50 {
					return fmt.Errorf("expected 50 domains, got %d", len(items))
				}
				return err
			})
		}
		series(fmt.Sprintf("top clients %s (limit 50)", ts), func() error {
			items, err := service.GetProfileQueryLogTopClients(ctx, profileID, model.RetentionOneMonth, ts, 50)
			if err == nil && len(items) == 0 {
				return fmt.Errorf("no clients")
			}
			return err
		})
	}
	series("reference: logs page1 created (hot path)", func() error {
		_, err := service.GetProfileQueryLogs(ctx, profileID, model.RetentionOneMonth, "all", model.LAST_MONTH, "", "", "created", 1, 100)
		return err
	})
}
