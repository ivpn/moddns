package mongodb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SubscriptionRepositorySuite covers the four lifecycle-related repository
// queries: the two coarse pre-filters (FindExpiredUnnotified,
// FindPendingDeleteUnnotified) and the two flag-only queries
// (FindWithLANotified, FindWithPendingDeleteNotified), plus the bulk
// SetNotified / SetPendingDeleteNotified writers.
//
// The repository queries are intentionally *coarse* — the cron post-filters
// via model.Subscription.GetStatus() — so these tests verify each query's
// inclusion/exclusion behaviour rather than the model predicate itself
// (which is unit-tested in api/model/subscription_test.go).
type SubscriptionRepositorySuite struct {
	suite.Suite
	client    *mongo.Client
	repo      SubscriptionRepository
	dbName    string
	container testcontainers.Container
}

func (s *SubscriptionRepositorySuite) SetupSuite() {
	ctx := context.Background()

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
		s.T().Fatalf("mongo connect failed: %v", err)
	}
	if err := client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		s.T().Fatalf("mongo ping failed: %v", err)
	}

	s.dbName = firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_test")
	_ = client.Database(s.dbName).Drop(connectCtx)

	s.client = client
	s.repo = NewSubscriptionRepository(client, s.dbName, "subscriptions_test")
}

func (s *SubscriptionRepositorySuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Database(s.dbName).Drop(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

// SetupTest truncates the collection between tests.
func (s *SubscriptionRepositorySuite) SetupTest() {
	if s.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.Database(s.dbName).Collection("subscriptions_test").Drop(ctx)
}

// seedSub inserts a Subscription document and returns its ID. The caller
// supplies `tier`, the date fields, and the notified flags.
func (s *SubscriptionRepositorySuite) seedSub(tier string, activeUntil, updatedAt time.Time, notified, notifiedPD bool) uuid.UUID {
	sub := model.Subscription{
		ID:                    uuid.New(),
		AccountID:             primitive.NewObjectID(),
		Tier:                  tier,
		ActiveUntil:           activeUntil,
		UpdatedAt:             updatedAt,
		IsActive:              true,
		Notified:              notified,
		NotifiedPendingDelete: notifiedPD,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Require().NoError(s.repo.Create(ctx, sub))
	return sub.ID
}

// containsID returns true if subs contains a Subscription with the given ID.
func containsID(subs []model.Subscription, id uuid.UUID) bool {
	for _, s := range subs {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_IncludesFreshTier1() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 1", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, id), "fresh Tier 1 must appear in pre-filter")
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_IncludesFreshTier1Lite() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 1 Lite", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, id), "Tier 1 Lite variant must also match the regex pre-filter")
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_IncludesFreshIVPNStandard() {
	now := time.Now()
	id := s.seedSub("IVPN Standard", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, id), "IVPN Standard product name must match the regex pre-filter")
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_ExcludesPaidFresh() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 2", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.False(containsID(subs, id), "paid sub with fresh dates must not appear")
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_IncludesStaleDates() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 2", now.Add(-20*24*time.Hour), now.Add(-20*24*time.Hour), false, false)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, id), "paid sub with stale dates must match via date predicates")
}

func (s *SubscriptionRepositorySuite) TestFindPendingDeleteUnnotified_ExcludesAlreadyNotified() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 1", now.Add(30*24*time.Hour), now, false, true)

	ctx := context.Background()
	subs, err := s.repo.FindPendingDeleteUnnotified(ctx)
	s.Require().NoError(err)
	s.False(containsID(subs, id), "notified Tier 1 sub must be excluded (idempotency)")
}

func (s *SubscriptionRepositorySuite) TestFindWithPendingDeleteNotified_ReturnsOnlyFlagged() {
	now := time.Now()
	flagged := s.seedSub("IVPN Tier 1", now.Add(30*24*time.Hour), now, false, true)
	unflagged := s.seedSub("IVPN Tier 2", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindWithPendingDeleteNotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, flagged), "flagged sub must be returned")
	s.False(containsID(subs, unflagged), "unflagged sub must not be returned")
}

func (s *SubscriptionRepositorySuite) TestFindWithLANotified_ReturnsOnlyFlagged() {
	now := time.Now()
	flagged := s.seedSub("IVPN Tier 2", now.Add(-1*24*time.Hour), now, true, false)
	unflagged := s.seedSub("IVPN Tier 2", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	subs, err := s.repo.FindWithLANotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(subs, flagged), "flagged sub must be returned")
	s.False(containsID(subs, unflagged), "unflagged sub must not be returned")
}

func (s *SubscriptionRepositorySuite) TestSetPendingDeleteNotified_BulkRoundTrip() {
	now := time.Now()
	idA := s.seedSub("IVPN Tier 1", now.Add(30*24*time.Hour), now, false, false)
	idB := s.seedSub("IVPN Tier 1", now.Add(30*24*time.Hour), now, false, false)

	ctx := context.Background()
	s.Require().NoError(s.repo.SetPendingDeleteNotified(ctx, []uuid.UUID{idA, idB}, true))

	flagged, err := s.repo.FindWithPendingDeleteNotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(flagged, idA))
	s.True(containsID(flagged, idB))

	// Flip both back.
	s.Require().NoError(s.repo.SetPendingDeleteNotified(ctx, []uuid.UUID{idA, idB}, false))
	flagged, err = s.repo.FindWithPendingDeleteNotified(ctx)
	s.Require().NoError(err)
	s.False(containsID(flagged, idA))
	s.False(containsID(flagged, idB))
}

func (s *SubscriptionRepositorySuite) TestSetNotified_BulkRoundTrip() {
	now := time.Now()
	id := s.seedSub("IVPN Tier 2", now.Add(-1*24*time.Hour), now, false, false)

	ctx := context.Background()
	s.Require().NoError(s.repo.SetNotified(ctx, []uuid.UUID{id}, true))
	flagged, err := s.repo.FindWithLANotified(ctx)
	s.Require().NoError(err)
	s.True(containsID(flagged, id))

	s.Require().NoError(s.repo.SetNotified(ctx, []uuid.UUID{id}, false))
	flagged, err = s.repo.FindWithLANotified(ctx)
	s.Require().NoError(err)
	s.False(containsID(flagged, id))
}

// TestFindDuplicateTokenHashGroups validates the read-only reconciliation
// aggregation against real Mongo: only token_hash values held by >1 NON-retired
// subscription are reported. Retired subs (deletion_scheduled_at set) and
// legacy subs (no token_hash) are excluded.
//
// specRef: signup-reset-behaviour.md (reconciliation report)
func (s *SubscriptionRepositorySuite) TestFindDuplicateTokenHashGroups() {
	now := time.Now()
	scheduled := now
	mk := func(tokenHash string, deletionScheduled *time.Time) {
		sub := model.Subscription{
			ID:                  uuid.New(),
			AccountID:           primitive.NewObjectID(),
			Tier:                "IVPN Tier 2",
			ActiveUntil:         now.Add(30 * 24 * time.Hour),
			UpdatedAt:           now,
			IsActive:            true,
			TokenHash:           tokenHash,
			DeletionScheduledAt: deletionScheduled,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Require().NoError(s.repo.Create(ctx, sub))
	}

	mk("dup", nil)        // two non-retired sharing "dup" → a duplicate group
	mk("dup", nil)        //
	mk("unique", nil)     // single → not a duplicate
	mk("mix", nil)        // one non-retired +
	mk("mix", &scheduled) //   one retired (deletion_scheduled_at) → only 1 counts → not a duplicate
	mk("", nil)           // legacy: token_hash omitted (absent) → excluded
	mk("", nil)           //

	ctx := context.Background()
	groups, err := s.repo.FindDuplicateTokenHashGroups(ctx)
	s.Require().NoError(err)

	byHash := map[string]model.DuplicateTokenHashGroup{}
	for _, g := range groups {
		byHash[g.TokenHash] = g
	}

	s.Len(groups, 1, "only token_hash 'dup' is a duplicate group")
	dup, ok := byHash["dup"]
	s.Require().True(ok, "'dup' must be reported")
	s.Equal(2, dup.Count)
	s.Len(dup.AccountIDs, 2)
	_, mixReported := byHash["mix"]
	s.False(mixReported, "'mix' has only one non-retired sub → not a duplicate")
	_, emptyReported := byHash[""]
	s.False(emptyReported, "legacy empty token_hash must never be grouped")
}

// Entry point.
func TestSubscriptionRepositorySuite(t *testing.T) {
	suite.Run(t, new(SubscriptionRepositorySuite))
}
