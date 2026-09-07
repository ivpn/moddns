package mongodb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AccountRepositorySuite provides isolated integration tests using testify's suite.
type AccountRepositorySuite struct {
	suite.Suite
	client    *mongo.Client
	repo      AccountRepository
	dbName    string
	container testcontainers.Container
}

// SetupSuite establishes a Mongo connection and prepares an isolated test database.
func (s *AccountRepositorySuite) SetupSuite() {
	ctx := context.Background()

	// Mongo image version sourced from docker compose: mongo:7.0.8
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
	if err != nil {
		s.T().Fatalf("failed to start mongo container: %v", err)
	}
	s.container = container

	host, err := container.Host(ctx)
	if err != nil {
		s.T().Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "27017/tcp")
	if err != nil {
		s.T().Fatalf("failed to get mapped port: %v", err)
	}

	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	clientOpts := options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource})
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		s.T().Fatalf("mongo connect failed (uri=%s): %v", uri, err)
	}
	if err := client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		s.T().Fatalf("mongo ping failed (authSource=%s uri=%s): %v", authSource, uri, err)
	}

	s.dbName = firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_test")
	_ = client.Database(s.dbName).Drop(connectCtx)

	s.client = client
	s.repo = NewAccountRepository(client, s.dbName, "accounts_test")

	// Simple write probe to ensure authenticated permissions.
	if _, werr := client.Database(s.dbName).Collection("_perm_check").InsertOne(connectCtx, bson.D{{Key: "ok", Value: true}, {Key: "ts", Value: time.Now()}}); werr != nil {
		s.T().Fatalf("write permission check failed (db=%s uri=%s): %v", s.dbName, uri, werr)
	}
}

// TearDownSuite drops the test database.
func (s *AccountRepositorySuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Database(s.dbName).Drop(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

// SetupTest truncates collections between tests.
func (s *AccountRepositorySuite) SetupTest() {
	if s.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.Database(s.dbName).Collection("accounts_test").Drop(ctx)
}

// Utility: choose first non-empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// TestProfileOperations validates add/remove/idempotency of profiles.
func (s *AccountRepositorySuite) TestProfileOperations() {
	if s.client == nil {
		s.T().Skip("client unavailable")
	}
	ctx := context.Background()

	accountID := primitive.NewObjectID().Hex()
	initialProfile := "profile-1"

	_, err := s.repo.CreateAccount(ctx, "user@example.com", "password123", accountID, initialProfile)
	s.Require().NoError(err, "CreateAccount should succeed")

	tests := []struct {
		name       string
		op         func() error
		wantLen    int
		wantValues []string
	}{
		{"idempotent add existing profile (no duplicate)", func() error { return s.repo.AddProfileToAccount(ctx, accountID, initialProfile) }, 1, []string{initialProfile}},
		{"add new profile", func() error { return s.repo.AddProfileToAccount(ctx, accountID, "profile-2") }, 2, []string{initialProfile, "profile-2"}},
		{"remove existing profile", func() error { return s.repo.RemoveProfileFromAccount(ctx, accountID, initialProfile) }, 1, []string{"profile-2"}},
	}

	for _, tc := range tests {
		err := tc.op()
		s.Require().NoError(err, tc.name)
		fetched, err := s.repo.GetAccountById(ctx, accountID)
		s.Require().NoError(err, tc.name+" fetch")
		s.Require().Len(fetched.Profiles, tc.wantLen, tc.name+" length")
		wantMap := map[string]bool{}
		for _, v := range tc.wantValues {
			wantMap[v] = true
		}
		for _, v := range fetched.Profiles {
			s.True(wantMap[v], "unexpected profile %s in %s", v, tc.name)
		}
	}

	// duplicate add check
	err = s.repo.AddProfileToAccount(ctx, accountID, "profile-2")
	s.Require().NoError(err, "duplicate add should not error")
	fetched, err := s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err, "fetch after duplicate add")
	count := 0
	for _, v := range fetched.Profiles {
		if v == "profile-2" {
			count++
		}
	}
	s.Equal(1, count, "profile-2 should appear exactly once")

	// deletion code update
	code := "DEL123"
	expires := time.Now().Add(time.Hour)
	err = s.repo.UpdateDeletionCode(ctx, accountID, code, expires)
	s.Require().NoError(err, "UpdateDeletionCode")
	fetched, err = s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err, "fetch after deletion code update")
	s.Equal(code, fetched.DeletionCode)
	s.NotNil(fetched.DeletionCodeExpires)
	s.True(fetched.DeletionCodeExpires.After(time.Now()), "expiry should be in future")
}

// TestAddDuplicateProfile explicitly verifies attempting to add an existing
// profile ID again is a no-op at the MongoDB level (ModifiedCount == 0) and
// returns no error. Because the repository does not surface ModifiedCount,
// we assert logically: the profile appears exactly once after two add calls.
func (s *AccountRepositorySuite) TestAddDuplicateProfile() {
	if s.client == nil {
		s.T().Skip("client unavailable")
	}
	ctx := context.Background()

	accountID := primitive.NewObjectID().Hex()
	dupProfile := "dup-profile-1"

	// Create account with an initial profile different from the one we'll duplicate
	_, err := s.repo.CreateAccount(ctx, "dup@example.com", "pass", accountID, "initial-X")
	s.Require().NoError(err, "CreateAccount")

	// First add should modify the document (profile appended)
	err = s.repo.AddProfileToAccount(ctx, accountID, dupProfile)
	s.Require().NoError(err, "first AddProfileToAccount")

	// Second add should be idempotent: no error, no second occurrence
	err = s.repo.AddProfileToAccount(ctx, accountID, dupProfile)
	s.Require().NoError(err, "second (duplicate) AddProfileToAccount")

	fetched, err := s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err, "GetAccountById after duplicate add")

	occurrences := 0
	for _, p := range fetched.Profiles {
		if p == dupProfile {
			occurrences++
		}
	}
	s.Equal(1, occurrences, "duplicate profile should exist only once")
}

// TestGetByEmailAndToken validates token and email lookups.
func (s *AccountRepositorySuite) TestGetByEmailAndToken() {
	if s.client == nil {
		s.T().Skip("client unavailable")
	}
	ctx := context.Background()

	accountID := primitive.NewObjectID().Hex()
	profileID := "profile-X"
	acc, err := s.repo.CreateAccount(ctx, "lookup@example.com", "secret", accountID, profileID)
	s.Require().NoError(err, "CreateAccount")

	acc.Tokens = []model.Token{{Value: "tok123", Type: "api"}}
	_, err = s.repo.UpdateAccount(ctx, acc)
	s.Require().NoError(err, "UpdateAccount with tokens")

	gotByEmail, err := s.repo.GetAccountByEmail(ctx, "lookup@example.com")
	s.Require().NoError(err, "GetAccountByEmail")
	s.Equal(acc.ID, gotByEmail.ID, "email lookup mismatch")

	gotByToken, err := s.repo.GetAccountByToken(ctx, "tok123", "api")
	s.Require().NoError(err, "GetAccountByToken")
	s.Equal(acc.ID, gotByToken.ID, "token lookup mismatch")
}

// TestEmailCaseInsensitiveRoundTrip validates that emails are stored in
// canonical lowercase form and that lookups tolerate any casing/whitespace.
// specRef: api-endpoint-behaviour.md A1, B6, C1
func (s *AccountRepositorySuite) TestEmailCaseInsensitiveRoundTrip() {
	if s.client == nil {
		s.T().Skip("client unavailable")
	}
	ctx := context.Background()

	accountID := primitive.NewObjectID().Hex()
	acc, err := s.repo.CreateAccount(ctx, " Mixed.Case@Example.COM ", "secret", accountID, "profile-case")
	s.Require().NoError(err, "CreateAccount")
	s.Equal("mixed.case@example.com", acc.Email, "stored email must be canonical lowercase")

	fetched, err := s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err, "GetAccountById")
	s.Equal("mixed.case@example.com", fetched.Email, "persisted email must be canonical lowercase")

	for _, variant := range []string{
		"mixed.case@example.com",
		"MIXED.CASE@EXAMPLE.COM",
		" Mixed.Case@Example.COM ",
	} {
		got, err := s.repo.GetAccountByEmail(ctx, variant)
		s.Require().NoError(err, "GetAccountByEmail(%q)", variant)
		s.Equal(acc.ID, got.ID, "lookup by %q must find the account", variant)
	}
}

// Entry point.
func TestAccountRepositorySuite(t *testing.T) {
	suite.Run(t, new(AccountRepositorySuite))
}
