package mongodb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
)

type ProfileRepositorySuite struct {
	suite.Suite
	client    *mongo.Client
	repo      ProfileRepository
	container testcontainers.Container
}

// startTestMongo starts a throwaway MongoDB (TEST_MONGO_IMAGE, default the production version).
func startTestMongo(t *testing.T) (*mongo.Client, testcontainers.Container) {
	t.Helper()
	ctx := context.Background()
	username := firstNonEmpty(os.Getenv("TEST_MONGO_USERNAME"), "testuser")
	password := firstNonEmpty(os.Getenv("TEST_MONGO_PASSWORD"), "testpass")
	authSource := firstNonEmpty(os.Getenv("DB_AUTH_SOURCE"), "admin")
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: firstNonEmpty(os.Getenv("TEST_MONGO_IMAGE"), "mongo:8.0.9"),
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
	return client, container
}

func (s *ProfileRepositorySuite) SetupSuite() {
	s.client, s.container = startTestMongo(s.T())
	s.repo = NewProfileRepository(s.client, firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_test")+"_profiles", "profiles")
}

func (s *ProfileRepositorySuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Disconnect(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

func (s *ProfileRepositorySuite) SetupTest() {
	_, err := s.repo.profilesCollection.DeleteMany(context.Background(), bson.D{})
	s.Require().NoError(err)
}

func (s *ProfileRepositorySuite) seed(statsEnabled bool, enabledAt *time.Time) *model.Profile {
	settings := model.NewSettings()
	settings.ProfileId = "p1"
	settings.Statistics = &model.StatisticsSettings{Enabled: statsEnabled, EnabledAt: enabledAt}
	settings.CustomRules = []*model.CustomRule{{ID: primitive.NewObjectID(), Action: model.ACTION_BLOCK, Value: "existing.com"}}
	p := &model.Profile{ID: primitive.NewObjectID(), ProfileId: "p1", AccountId: "a1", Name: "one", Settings: settings}
	s.Require().NoError(s.repo.CreateProfile(context.Background(), p))
	return p
}

func statsSet(v bool) repository.FieldSet {
	return repository.FieldSet{Field: "settings.statistics.enabled", Value: v}
}

var updateNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

// specRef: api-endpoint-behaviour.md G21 — only the named fields change; before and after are returned.
func (s *ProfileRepositorySuite) TestUpdateFields_SetsOnlyNamedFields() {
	ctx := context.Background()
	s.seed(false, nil)

	before, after, err := s.repo.UpdateFields(ctx, "p1", repository.ProfileFieldsUpdate{
		Set: []repository.FieldSet{
			{Field: "settings.logs.retention", Value: model.Retention("1w")},
			{Field: "settings.advanced.recursor", Value: model.RECURSOR_SDNS},
			{Field: "name", Value: "renamed"},
		},
		EnabledAtNow: updateNow,
	})
	s.Require().NoError(err)
	s.Equal(model.RetentionOneHour, before.Settings.Logs.Retention)
	s.Equal("one", before.Name)
	s.Equal(model.Retention("1w"), after.Settings.Logs.Retention)
	s.Equal(model.RECURSOR_SDNS, after.Settings.Advanced.Recursor)
	s.Equal("renamed", after.Name)
	s.True(after.Settings.Security.DNSSECSettings.Enabled, "untouched fields keep their stored value")
	s.Equal(model.ACTION_BLOCK, after.Settings.Privacy.BlocklistsSubdomainsRule)
	s.Nil(after.Settings.Statistics.EnabledAt, "enabled_at is not set when statistics are not touched")
}

// specRef: api-endpoint-behaviour.md G21 — values are stored as literals.
func (s *ProfileRepositorySuite) TestUpdateFields_StoresValuesVerbatim() {
	s.seed(false, nil)
	_, after, err := s.repo.UpdateFields(context.Background(), "p1", repository.ProfileFieldsUpdate{
		Set: []repository.FieldSet{{Field: "name", Value: "$name"}},
	})
	s.Require().NoError(err)
	s.Equal("$name", after.Name)
}

// specRef: api-endpoint-behaviour.md G7, G22 — enabled_at follows the stored value it replaces.
func (s *ProfileRepositorySuite) TestUpdateFields_StatisticsEnabledAt() {
	old := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		stored   bool
		storedAt *time.Time
		set      bool
		wantAt   *time.Time
	}{
		{name: "off to on sets now", stored: false, set: true, wantAt: &updateNow},
		{name: "on to off clears", stored: true, storedAt: &old, set: false, wantAt: nil},
		{name: "on to on keeps", stored: true, storedAt: &old, set: true, wantAt: &old},
		{name: "on to on without enabled_at stays without", stored: true, set: true, wantAt: nil},
		{name: "off to off keeps", stored: false, storedAt: &old, set: false, wantAt: &old},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.seed(tc.stored, tc.storedAt)
			before, after, err := s.repo.UpdateFields(context.Background(), "p1", repository.ProfileFieldsUpdate{
				Set: []repository.FieldSet{statsSet(tc.set)}, EnabledAtNow: updateNow,
			})
			s.Require().NoError(err)
			s.Equal(tc.stored, before.Settings.Statistics.Enabled)
			s.Equal(tc.set, after.Settings.Statistics.Enabled)
			if tc.wantAt == nil {
				s.Nil(after.Settings.Statistics.EnabledAt)
				return
			}
			s.Require().NotNil(after.Settings.Statistics.EnabledAt)
			s.True(tc.wantAt.Equal(*after.Settings.Statistics.EnabledAt), "got %v", after.Settings.Statistics.EnabledAt)
		})
	}
}

// specRef: api-endpoint-behaviour.md G21 — custom rules are appended only when no stored rule has the value.
func (s *ProfileRepositorySuite) TestUpdateFields_AppendCustomRulesIfAbsent() {
	s.seed(false, nil)
	newRule := &model.CustomRule{ID: primitive.NewObjectID(), Action: model.ACTION_ALLOW, Value: "new.com"}
	dupRule := &model.CustomRule{ID: primitive.NewObjectID(), Action: model.ACTION_ALLOW, Value: "existing.com"}
	upd := repository.ProfileFieldsUpdate{
		Set:               []repository.FieldSet{{Field: "settings.privacy.default_rule", Value: model.DEFAULT_RULE_BLOCK}},
		AppendCustomRules: []*model.CustomRule{newRule, dupRule},
	}
	_, after, err := s.repo.UpdateFields(context.Background(), "p1", upd)
	s.Require().NoError(err)
	s.Require().Len(after.Settings.CustomRules, 2)
	s.Equal("existing.com", after.Settings.CustomRules[0].Value)
	s.EqualValues(model.ACTION_BLOCK, after.Settings.CustomRules[0].Action)
	s.Equal(newRule.ID, after.Settings.CustomRules[1].ID)

	_, after, err = s.repo.UpdateFields(context.Background(), "p1", upd)
	s.Require().NoError(err)
	s.Len(after.Settings.CustomRules, 2, "a repeated update appends nothing")
}

// specRef: api-endpoint-behaviour.md G22 — of two disables only one sees the true value it replaces.
func (s *ProfileRepositorySuite) TestUpdateFields_ConcurrentDisablesSeeOneTransition() {
	s.seed(true, &updateNow)
	upd := repository.ProfileFieldsUpdate{Set: []repository.FieldSet{statsSet(false)}, EnabledAtNow: updateNow}
	results := make(chan bool, 2)
	for range 2 {
		go func() {
			before, _, err := s.repo.UpdateFields(context.Background(), "p1", upd)
			s.NoError(err)
			results <- before != nil && before.Settings.Statistics.Enabled
		}()
	}
	sawEnabled := 0
	for range 2 {
		if <-results {
			sawEnabled++
		}
	}
	s.Equal(1, sawEnabled)
}

// specRef: api-endpoint-behaviour.md G21 — an unknown profile is reported as not found.
func (s *ProfileRepositorySuite) TestUpdateFields_MissingProfile() {
	_, _, err := s.repo.UpdateFields(context.Background(), "missing", repository.ProfileFieldsUpdate{
		Set: []repository.FieldSet{statsSet(true)},
	})
	s.ErrorIs(err, dbErrors.ErrProfileNotFound)
}

// specRef: api-endpoint-behaviour.md J13, J14 — one query returns logs.enabled per existing profile.
func (s *ProfileRepositorySuite) TestGetProfilesLogsEnabled() {
	ctx := context.Background()
	_, err := s.repo.profilesCollection.InsertMany(ctx, []any{
		bson.D{{Key: "profile_id", Value: "on"}, {Key: "settings", Value: bson.D{{Key: "logs", Value: bson.D{{Key: "enabled", Value: true}}}}}},
		bson.D{{Key: "profile_id", Value: "off"}, {Key: "settings", Value: bson.D{{Key: "logs", Value: bson.D{{Key: "enabled", Value: false}}}}}},
		bson.D{{Key: "profile_id", Value: "no-block"}, {Key: "settings", Value: bson.D{}}},
		bson.D{{Key: "profile_id", Value: "other"}, {Key: "settings", Value: bson.D{{Key: "logs", Value: bson.D{{Key: "enabled", Value: true}}}}}},
	})
	s.Require().NoError(err)

	got, err := s.repo.GetProfilesLogsEnabled(ctx, []string{"on", "off", "no-block", "missing"})
	s.Require().NoError(err)
	s.Equal(map[string]bool{"on": true, "off": false, "no-block": false}, got)

	empty, err := s.repo.GetProfilesLogsEnabled(ctx, nil)
	s.Require().NoError(err)
	s.Empty(empty)
}

func TestProfileRepositorySuite(t *testing.T) {
	suite.Run(t, new(ProfileRepositorySuite))
}
