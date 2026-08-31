package querylogs

// Env-gated benchmark for the device-list aggregation against a realistically
// sized 1-month retention collection. Not part of the regular suite — run with:
//
//	BENCH_QUERY_LOG_DEVICES=1 go test ./service/query_logs/ -run TestBenchmarkQueryLogDevices -v
//	BENCH_DOCS=3000000 BENCH_DEVICES=25 BENCH_QUERY_LOG_DEVICES=1 go test ... (overrides)
//
// Baselines measured alongside: the paged logs fetch (default sort, the hottest
// existing path) and the unindexed domain sort (the most expensive existing path)
// over the same data, so the device aggregation's cost has context.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
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

func benchEnvInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func TestBenchmarkQueryLogDevices(t *testing.T) {
	if os.Getenv("BENCH_QUERY_LOG_DEVICES") != "1" {
		t.Skip("set BENCH_QUERY_LOG_DEVICES=1 to run the device-list benchmark")
	}
	ctx := context.Background()

	// Container boot mirrors QueryLogsServiceSuite.SetupSuite.
	mongoImage := firstNonEmpty(os.Getenv("TEST_MONGO_IMAGE"), "mongo:7.0.8")
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

	dbName := "dns_query_logs_bench"
	_ = client.Database(dbName).Drop(ctx)
	repo := mongodb.NewQueryLogsRepository(client, dbName, "query_logs")
	service := NewQueryLogsService(&repo)
	profileID := primitive.NewObjectID().Hex()

	// A REAL time-series collection with the proxy's exact options — a plain
	// InsertMany would auto-create a regular collection and the numbers would
	// not reflect bucket packing/unpacking at all. MongoDB also auto-creates
	// the {profile_id, timestamp} meta+time index on creation (6.3+).
	tsOpts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("timestamp").
			SetMetaField("profile_id").
			SetGranularity("seconds"),
	).SetExpireAfterSeconds(2592000)
	require.NoError(t, client.Database(dbName).CreateCollection(ctx, "query_logs_1m", tsOpts))
	coll := client.Database(dbName).Collection("query_logs_1m")
	// Mirror the hand-created domain index observed on the dev/prod-like DB
	// (present on 6h/1d/1w/1m there; owner unknown — see moddns-shadow#688).
	_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "profile_id", Value: 1},
			{Key: "dns_request.domain", Value: 1},
			{Key: "timestamp", Value: -1},
		},
		Options: options.Index().SetName("profile_domain_timestamp"),
	})
	require.NoError(t, err)

	docCount := benchEnvInt("BENCH_DOCS", 1_000_000)
	deviceCount := benchEnvInt("BENCH_DEVICES", 20)

	// Seed: docs spread across 30 days, devices assigned with a skew (device-00
	// gets ~half the traffic, like a busy router), ~5% with no device id.
	// Distinct ids produced: device-00 .. device-<deviceCount> = deviceCount+1.
	const batchSize = 10_000
	now := time.Now()
	seedStart := time.Now()
	batch := make([]any, 0, batchSize)
	for i := 0; i < docCount; i++ {
		device := ""
		if i%20 != 0 { // 5% device-less
			if i%2 == 0 {
				device = "device-00"
			} else {
				// i/2 cycles through all residues (odd i alone hits only half).
				device = fmt.Sprintf("device-%02d", 1+((i/2)%deviceCount))
			}
		}
		ts := now.Add(-time.Duration(i%(30*24*60)) * time.Minute)
		batch = append(batch, bson.D{
			{Key: "timestamp", Value: ts},
			{Key: "profile_id", Value: profileID},
			{Key: "device_id", Value: device},
			{Key: "status", Value: "processed"},
			{Key: "reasons", Value: bson.A{}},
			{Key: "dns_request", Value: bson.D{
				{Key: "domain", Value: fmt.Sprintf("host-%d.example.com", i%5000)},
				{Key: "query_type", Value: "A"},
				{Key: "response_code", Value: "NOERROR"},
				{Key: "dnssec", Value: false},
			}},
			{Key: "client_ip", Value: "10.0.0.1"},
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
	t.Logf("seeded %d docs (%d distinct devices, 5%% device-less) in %s", docCount, deviceCount+1, time.Since(seedStart).Round(time.Millisecond))

	timeIt := func(name string, fn func() error) {
		cold := time.Now()
		require.NoError(t, fn())
		coldDur := time.Since(cold)
		const warmRuns = 5
		var warmTotal time.Duration
		for i := 0; i < warmRuns; i++ {
			start := time.Now()
			require.NoError(t, fn())
			warmTotal += time.Since(start)
		}
		t.Logf("%-42s cold=%8s warm-avg=%8s", name, coldDur.Round(time.Millisecond), (warmTotal / warmRuns).Round(time.Millisecond))
	}

	timeIt("GetQueryLogDevices (new endpoint)", func() error {
		devices, err := service.GetProfileQueryLogDevices(ctx, profileID, model.RetentionOneMonth)
		if err == nil && len(devices) != deviceCount+1 {
			return fmt.Errorf("expected %d devices, got %d", deviceCount+1, len(devices))
		}
		return err
	})
	timeIt("GetQueryLogs page1 created (hot path)", func() error {
		_, err := service.GetProfileQueryLogs(ctx, profileID, model.RetentionOneMonth, "all", "LAST_MONTH", "", "", "created", 1, 100)
		return err
	})
	timeIt("GetQueryLogs sort=domain (worst path)", func() error {
		_, err := service.GetProfileQueryLogs(ctx, profileID, model.RetentionOneMonth, "all", "LAST_MONTH", "", "", "domain", 1, 100)
		return err
	})
}
