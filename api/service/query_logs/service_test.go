package querylogs

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ivpn/dns/api/db/mongodb"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type stubQueryLogsRepository struct {
	getCalls    int
	deleteCalls int
	lastSort    string
}

func (s *stubQueryLogsRepository) GetQueryLogs(ctx context.Context, profileId string, retention model.Retention, status string, timespan int, deviceId, search, sortBy string, page, limit int) ([]model.QueryLog, error) {
	s.getCalls++
	s.lastSort = sortBy
	return nil, nil
}

func (s *stubQueryLogsRepository) DeleteQueryLogs(ctx context.Context, profileId string) error {
	s.deleteCalls++
	return nil
}

// TestGetProfileQueryLogsInvalidTimespan validates that an invalid timespan string short-circuits
// before hitting the repository and returns an error. A lightweight stub ensures the repository
// layer is not touched.
func TestGetProfileQueryLogsInvalidTimespan(t *testing.T) {
	mockRepo := &stubQueryLogsRepository{}
	svc := NewQueryLogsService(mockRepo)

	tests := []struct {
		name     string
		timespan string
	}{
		{"empty timespan", ""},
		{"nonsense timespan", "NOT_A_SPAN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := svc.GetProfileQueryLogs(context.Background(), "profile-1", model.RetentionOneDay, "all", tc.timespan, "", "", "created", 0, 0)
			if err == nil {
				t.Fatalf("expected error for timespan %q, got nil", tc.timespan)
			}
			if logs != nil {
				t.Fatalf("expected nil logs on error, got %#v", logs)
			}
			if mockRepo.getCalls != 0 {
				t.Fatalf("expected repository GetQueryLogs not to be called, got %d calls", mockRepo.getCalls)
			}
		})
	}
}

func TestGetProfileQueryLogsSortForwarded(t *testing.T) {
	mockRepo := &stubQueryLogsRepository{}
	svc := NewQueryLogsService(mockRepo)

	_, err := svc.GetProfileQueryLogs(context.Background(), "profile-1", model.RetentionOneDay, "all", model.LAST_1_DAY, "", "", "domain", 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mockRepo.getCalls != 1 {
		t.Fatalf("expected repository to be called once, got %d", mockRepo.getCalls)
	}
	if mockRepo.lastSort != "domain" {
		t.Fatalf("expected sort 'domain', got %q", mockRepo.lastSort)
	}
}

type QueryLogsServiceSuite struct {
	suite.Suite
	client    *mongo.Client
	repo      mongodb.QueryLogsRepository
	service   *QueryLogsService
	dbName    string
	container testcontainers.Container
	collMap   map[model.Retention]*mongo.Collection
	profileID string
}

func (s *QueryLogsServiceSuite) SetupSuite() {
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
	s.Require().NoError(err, "failed to start mongo container")
	s.container = container

	host, err := container.Host(ctx)
	s.Require().NoError(err, "failed to get container host")
	port, err := container.MappedPort(ctx, "27017/tcp")
	s.Require().NoError(err, "failed to get mapped port")

	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", url.QueryEscape(username), url.QueryEscape(password), host, port.Port())
	clientOpts := options.Client().ApplyURI(uri).SetAuth(options.Credential{Username: username, Password: password, AuthSource: authSource})
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, clientOpts)
	s.Require().NoError(err, "mongo connect failed")
	s.Require().NoError(client.Database(authSource).RunCommand(connectCtx, bson.D{{Key: "ping", Value: 1}}).Err(), "mongo ping failed")

	s.dbName = firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_query_logs_test")
	_ = client.Database(s.dbName).Drop(connectCtx)

	s.client = client
	// Use canonical collection name prefix so timeseries collections align with repository naming expectations
	s.repo = mongodb.NewQueryLogsRepository(client, s.dbName, "query_logs")
	s.service = NewQueryLogsService(&s.repo)
	s.profileID = primitive.NewObjectID().Hex()

	// map collections for seeding convenience
	s.collMap = map[model.Retention]*mongo.Collection{
		model.RetentionOneHour:  client.Database(s.dbName).Collection("query_logs_1h"),
		model.RetentionSixHours: client.Database(s.dbName).Collection("query_logs_6h"),
		model.RetentionOneDay:   client.Database(s.dbName).Collection("query_logs_1d"),
		model.RetentionOneWeek:  client.Database(s.dbName).Collection("query_logs_1w"),
		model.RetentionOneMonth: client.Database(s.dbName).Collection("query_logs_1m"),
	}

	// Initial seed is deferred to SetupTest to guarantee fresh data per test.
}

func (s *QueryLogsServiceSuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.client != nil {
		_ = s.client.Database(s.dbName).Drop(ctx)
	}
	if s.container != nil {
		_ = s.container.Terminate(ctx)
	}
}

func (s *QueryLogsServiceSuite) SetupTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Drop only the week retention collection to reset data (other retentions unused in tests)
	_ = s.collMap[model.RetentionOneWeek].Drop(ctx)
	// Reseed fresh data for each test
	s.seedQueryLogs(ctx)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// seedQueryLogs inserts a controlled set of documents to exercise search, status filtering, timespan window, pagination.
func (s *QueryLogsServiceSuite) seedQueryLogs(ctx context.Context) {
	coll := s.collMap[model.RetentionOneWeek]
	now := time.Now()
	docs := []any{
		bson.D{{Key: "timestamp", Value: now.Add(-2 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "blocked"}, {Key: "reasons", Value: bson.A{"malware"}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "example.com"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.4"}, {Key: "protocol", Value: "udp"}},
		bson.D{{Key: "timestamp", Value: now.Add(-3 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "phone"}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "sub.example.com"}, {Key: "query_type", Value: "AAAA"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: true}}}, {Key: "client_ip", Value: "1.2.3.5"}, {Key: "protocol", Value: "udp"}},
		bson.D{{Key: "timestamp", Value: now.Add(-25 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "blocked"}, {Key: "reasons", Value: bson.A{"tracker"}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "old.example.com"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.6"}, {Key: "protocol", Value: "udp"}}, // outside 1d timespan
		bson.D{{Key: "timestamp", Value: now.Add(-1 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "tablet"}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "example.org"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NXDOMAIN"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.7"}, {Key: "protocol", Value: "udp"}},
		// Another profile for isolation
		bson.D{{Key: "timestamp", Value: now.Add(-2 * time.Hour)}, {Key: "profile_id", Value: "other-profile"}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "blocked"}, {Key: "reasons", Value: bson.A{"malware"}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "example.com"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "9.9.9.9"}, {Key: "protocol", Value: "udp"}},
	}
	_, err := coll.InsertMany(ctx, docs)
	s.Require().NoError(err, "seed InsertMany should succeed")
}

// TestGetProfileQueryLogs exercises filtering by status, timespan (1d), device, search substring, and pagination.
func (s *QueryLogsServiceSuite) TestGetProfileQueryLogs() {
	ctx := context.Background()
	retention := model.RetentionOneWeek // repository uses this to pick collection

	cases := []struct {
		name                 string
		status               string
		timespan             string
		deviceId             string
		search               string
		sortBy               string
		page                 int
		limit                int
		wantCount            int
		assertDomainContains string
	}{
		{"blocked search example within 1d", "blocked", "LAST_1_DAY", "", "example", "created", 0, 0, 1, "example"},
		// Only sub.example.com matches processed status; example.com is blocked. Expect 1 result.
		{"processed search com within 1d", "processed", "LAST_1_DAY", "", "com", "created", 0, 0, 1, "com"},
		{"all no search within 1d", "all", "LAST_1_DAY", "", "", "created", 0, 0, 3, ""}, // excludes old.chatgpt.com outside 1d
		{"device filtered processed", "processed", "LAST_1_DAY", "tablet", "", "created", 0, 0, 1, "example.org"},
		{"pagination first page size 1", "processed", "LAST_1_DAY", "", "com", "created", 1, 1, 1, "com"},
		{"search miss returns empty", "blocked", "LAST_1_DAY", "", "nomatch", "created", 0, 0, 0, ""},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			logs, err := s.service.GetProfileQueryLogs(ctx, s.profileID, retention, tc.status, tc.timespan, tc.deviceId, tc.search, tc.sortBy, tc.page, tc.limit)
			s.Require().NoError(err, "service call should not error")
			s.Equal(tc.wantCount, len(logs), "unexpected log count")
			if tc.assertDomainContains != "" && tc.wantCount > 0 {
				for _, l := range logs {
					s.Contains(l.DNSRequest.Domain, tc.assertDomainContains, "domain should contain substring")
				}
			}
		})
	}
}

func (s *QueryLogsServiceSuite) TestGetProfileQueryLogsSorting() {
	ctx := context.Background()
	retention := model.RetentionOneWeek

	s.Run("domain ascending", func() {
		logs, err := s.service.GetProfileQueryLogs(ctx, s.profileID, retention, "all", "LAST_7_DAYS", "", "", "domain", 0, 0)
		s.Require().NoError(err)
		s.Equal(4, len(logs))
		domains := []string{}
		for _, l := range logs {
			domains = append(domains, l.DNSRequest.Domain)
		}
		s.Equal([]string{"example.com", "example.org", "old.example.com", "sub.example.com"}, domains)
	})

	s.Run("client ip ascending", func() {
		logs, err := s.service.GetProfileQueryLogs(ctx, s.profileID, retention, "all", "LAST_7_DAYS", "", "", "client_ip", 0, 0)
		s.Require().NoError(err)
		s.Equal(4, len(logs))
		ips := []string{}
		for _, l := range logs {
			ips = append(ips, l.ClientIP)
		}
		s.Equal([]string{"1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7"}, ips)
	})
}

// TestDownloadProfileQueryLogs verifies that timespan filter is not applied (0) and all statuses returned.
func (s *QueryLogsServiceSuite) TestDownloadProfileQueryLogs() {
	ctx := context.Background()
	retention := model.RetentionOneWeek

	logs, err := s.service.DownloadProfileQueryLogs(ctx, s.profileID, retention, 0, 0)
	s.Require().NoError(err)
	// Should include the document outside 1d window (old.chatgpt.com) but not other profile's logs.
	s.Equal(4, len(logs), "download should return all 4 logs for profile")
	foundOld := false
	for _, l := range logs {
		if l.DNSRequest.Domain == "old.example.com" {
			foundOld = true
		}
		// ensure no cross-profile contamination
		s.Equal(s.profileID, l.ProfileID)
	}
	s.True(foundOld, "expected old.example.com present in download set")
}

// TestDeleteProfileQueryLogs ensures removal from all retention collections.
func (s *QueryLogsServiceSuite) TestDeleteProfileQueryLogs() {
	ctx := context.Background()
	retention := model.RetentionOneWeek
	// Sanity pre-check
	pre, err := s.service.DownloadProfileQueryLogs(ctx, s.profileID, retention, 0, 0)
	s.Require().NoError(err)
	s.True(len(pre) > 0, "precondition: logs exist before delete")

	err = s.service.DeleteProfileQueryLogs(ctx, s.profileID)
	s.Require().NoError(err, "delete should succeed")

	post, err := s.service.DownloadProfileQueryLogs(ctx, s.profileID, retention, 0, 0)
	s.Require().NoError(err)
	s.Equal(0, len(post), "logs should be gone after delete")
}

// TestSearchRegexInjection verifies that regex meta characters in search are treated literally (escaped)
// and do not broaden matches. The repository uses regexp.QuoteMeta, so patterns like "example.com.*"
// should return zero results instead of matching multiple domains.
func (s *QueryLogsServiceSuite) TestSearchRegexInjection() {
	ctx := context.Background()
	retention := model.RetentionOneWeek

	cases := []struct {
		name      string
		search    string
		status    string
		timespan  string
		wantCount int
	}{
		{"literal dot star pattern", "example.com.*", "all", "LAST_7_DAYS", 0},
		{"literal parentheses", "(example.com)", "all", "LAST_7_DAYS", 0},
		{"literal end anchor symbol", "sub.example.com$", "processed", "LAST_7_DAYS", 0},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			logs, err := s.service.GetProfileQueryLogs(ctx, s.profileID, retention, tc.status, tc.timespan, "", tc.search, "created", 0, 0)
			s.Require().NoError(err)
			s.Equal(tc.wantCount, len(logs), "regex meta should be escaped; unexpected matches for %q", tc.search)
		})
	}
}

func TestQueryLogsServiceSuite(t *testing.T) {
	suite.Run(t, new(QueryLogsServiceSuite))
}
