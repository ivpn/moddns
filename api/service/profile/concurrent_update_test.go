package profile_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/db/mongodb"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/api/service/statistics"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func startMongo(t *testing.T) *mongo.Client {
	t.Helper()
	ctx := context.Background()
	username, password := envOr("TEST_MONGO_USERNAME", "testuser"), envOr("TEST_MONGO_PASSWORD", "testpass")
	authSource := envOr("DB_AUTH_SOURCE", "admin")
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: envOr("TEST_MONGO_IMAGE", "mongo:8.0.9"),
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": username,
				"MONGO_INITDB_ROOT_PASSWORD": password,
			},
			ExposedPorts: []string{"27017/tcp"},
			WaitingFor:   wait.ForLog("Waiting for connections").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "27017/tcp")
	require.NoError(t, err)
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource}))
	require.NoError(t, err)
	require.NoError(t, client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err())
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

// readBarrierRepo holds every GetProfileById until n callers have read, so
// overlapping PATCHes all start from the same stored state.
type readBarrierRepo struct {
	repository.ProfileRepository
	wg *sync.WaitGroup
}

func (r readBarrierRepo) GetProfileById(ctx context.Context, profileId string) (*model.Profile, error) {
	p, err := r.ProfileRepository.GetProfileById(ctx, profileId)
	r.wg.Done()
	r.wg.Wait()
	return p, err
}

func settingsHashes(t *testing.T, rdb *redis.Client, profileId string) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, h := range []string{"logs", "statistics", "privacy", "security:dnssec", "security:rebinding_protection", "advanced"} {
		v, err := rdb.HGetAll(context.Background(), fmt.Sprintf("settings:%s:%s", profileId, h)).Result()
		require.NoError(t, err)
		out[h] = v
	}
	return out
}

// specRef: api-endpoint-behaviour.md G21, G22, G23 — two overlapping PATCHes of
// different paths both persist, the statistics purge runs once and Redis matches Mongo.
func TestUpdateProfile_ConcurrentPatchesOnDifferentPathsBothPersist(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Docker")
	}
	ctx := context.Background()
	client := startMongo(t)
	repo := mongodb.NewProfileRepository(client, "dns_test_profile_concurrency", "profiles")

	const profileId, accountId = "profileconc1", "accountconc1"
	enabledAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	settings := model.NewSettings()
	settings.ProfileId = profileId
	settings.Statistics = &model.StatisticsSettings{Enabled: true, EnabledAt: &enabledAt}
	settings.Advanced.Recursor = model.RECURSOR_SDNS
	require.NoError(t, repo.CreateProfile(ctx, &model.Profile{ProfileId: profileId, AccountId: accountId, Name: "conc", Settings: settings}))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisCache := cache.NewRedisCacheFromClient(rdb)
	require.NoError(t, redisCache.CreateOrUpdateProfileSettings(ctx, settings))

	statsRepo := mocks.NewStatisticsRepository(t)
	statsRepo.On("DeleteProfileStatistics", mock.Anything, profileId, (*time.Time)(nil)).Return(nil).Once()

	var wg sync.WaitGroup
	wg.Add(2)
	svc := profile.NewProfileService(config.ServerConfig{}, config.ServiceConfig{}, readBarrierRepo{ProfileRepository: &repo, wg: &wg},
		mocks.NewAccountRepository(t), nil, nil, statistics.NewStatisticsService(statsRepo), nil, redisCache, mocks.NewGeneratoridgen(t), validator.New())

	patches := [][]model.ProfileUpdate{
		{{Operation: model.UpdateOperationReplace, Path: "/settings/statistics/enabled", Value: false}},
		{{Operation: model.UpdateOperationReplace, Path: "/settings/advanced/recursor", Value: model.RECURSOR_KNOT}},
	}
	var run sync.WaitGroup
	errs := make([]error, len(patches))
	for i := range patches {
		run.Add(1)
		go func(i int) {
			defer run.Done()
			_, errs[i] = svc.UpdateProfile(ctx, accountId, profileId, patches[i])
		}(i)
	}
	run.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	stored, err := repo.GetProfileById(ctx, profileId)
	require.NoError(t, err)
	require.False(t, stored.Settings.Statistics.Enabled, "statistics.enabled = false persists")
	require.Nil(t, stored.Settings.Statistics.EnabledAt, "enabled_at is cleared with the disable")
	require.Equal(t, model.RECURSOR_KNOT, stored.Settings.Advanced.Recursor, "recursor change persists")
	statsRepo.AssertNumberOfCalls(t, "DeleteProfileStatistics", 1)

	want := miniredis.RunT(t)
	wantRdb := redis.NewClient(&redis.Options{Addr: want.Addr()})
	require.NoError(t, cache.NewRedisCacheFromClient(wantRdb).CreateOrUpdateProfileSettings(ctx, stored.Settings))
	require.Equal(t, settingsHashes(t, wantRdb, profileId), settingsHashes(t, rdb, profileId), "Redis settings hashes match Mongo")
}
