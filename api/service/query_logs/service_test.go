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

func (s *stubQueryLogsRepository) GetQueryLogDevices(ctx context.Context, profileId string, retention model.Retention) ([]model.QueryLogDevice, error) {
	return nil, nil
}

func (s *stubQueryLogsRepository) GetQueryLogTopDomains(ctx context.Context, profileId string, retention model.Retention, status string, timespanHours, limit int) ([]model.QueryLogTopDomain, error) {
	s.getCalls++
	return nil, nil
}

func (s *stubQueryLogsRepository) GetQueryLogTopClients(ctx context.Context, profileId string, retention model.Retention, timespanHours, limit int) ([]model.QueryLogTopClient, error) {
	s.getCalls++
	return nil, nil
}

func (s *stubQueryLogsRepository) DeleteQueryLogs(ctx context.Context, profileId string) error {
	s.deleteCalls++
	return nil
}

func (s *stubQueryLogsRepository) ListQueryLogProfileIDs(ctx context.Context) ([]string, error) {
	return nil, nil
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
		// specRef: query-log-outcomes-behaviour.md #O11 — answered SERVFAIL by the proxy, neither blocked nor processed
		bson.D{{Key: "timestamp", Value: now.Add(-4 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "unavailable"}, {Key: "reasons", Value: bson.A{}}, {Key: "outcome", Value: "filter_unavailable"}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "unavailable.example.net"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "SERVFAIL"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.8"}, {Key: "protocol", Value: "udp"}},
		// "No answer" class (C5): the timeout row matches; the DNSSEC verdict and
		// the outcome-less REFUSED row stand in for rows the filter must NOT match.
		bson.D{{Key: "timestamp", Value: now.Add(-5 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "outcome", Value: "timeout"}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "timeout.unanswered.test"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "SERVFAIL"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.8"}, {Key: "protocol", Value: "udp"}},
		bson.D{{Key: "timestamp", Value: now.Add(-6 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{"dnssec_failed"}}, {Key: "outcome", Value: "servfail_dnssec"}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "dnssec.unanswered.test"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "SERVFAIL"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.8"}, {Key: "protocol", Value: "udp"}},
		bson.D{{Key: "timestamp", Value: now.Add(-7 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: "laptop"}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "legacy.unanswered.test"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "REFUSED"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.8"}, {Key: "protocol", Value: "udp"}},
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
		{"all no search within 1d", "all", "LAST_1_DAY", "", "", "created", 0, 0, 7, ""}, // excludes old.example.com outside 1d
		// tableRef: query-log-outcomes-behaviour.md #C5 — outcome-based class (C3 set); DNSSEC verdicts and rows without an outcome are not matched
		{"unanswered selects every no-answer outcome", "unanswered", "LAST_1_DAY", "", "", "created", 0, 0, 2, ""},
		{"unanswered excludes resolved and blocked rows", "unanswered", "LAST_1_DAY", "", "example.com", "created", 0, 0, 0, ""},
		{"unanswered combines with device filter", "unanswered", "LAST_1_DAY", "tablet", "", "created", 0, 0, 0, ""},
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
		s.Equal(8, len(logs))
		domains := []string{}
		for _, l := range logs {
			domains = append(domains, l.DNSRequest.Domain)
		}
		s.Equal([]string{"dnssec.unanswered.test", "example.com", "example.org", "legacy.unanswered.test", "old.example.com", "sub.example.com", "timeout.unanswered.test", "unavailable.example.net"}, domains)
	})

	s.Run("client ip ascending", func() {
		logs, err := s.service.GetProfileQueryLogs(ctx, s.profileID, retention, "all", "LAST_7_DAYS", "", "", "client_ip", 0, 0)
		s.Require().NoError(err)
		s.Equal(8, len(logs))
		ips := []string{}
		for _, l := range logs {
			ips = append(ips, l.ClientIP)
		}
		s.Equal([]string{"1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7", "1.2.3.8", "1.2.3.8", "1.2.3.8", "1.2.3.8"}, ips)
	})
}

// TestDownloadProfileQueryLogs verifies that timespan filter is not applied (0) and all statuses returned.
func (s *QueryLogsServiceSuite) TestDownloadProfileQueryLogs() {
	ctx := context.Background()
	retention := model.RetentionOneWeek

	logs, err := s.service.DownloadProfileQueryLogs(ctx, s.profileID, retention, 0, 0)
	s.Require().NoError(err)
	// Should include the document outside 1d window (old.example.com) but not other profile's logs.
	s.Equal(8, len(logs), "download should return all 8 logs for profile")
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

// TestGetProfileQueryLogDevices verifies the distinct-device aggregation:
// sorted distinct ids, empty/missing device_id excluded, cross-profile
// isolation, last_seen = the device's newest timestamp, and whole-window
// scope (no timespan floor — the -25h "laptop" doc still counts).
// tableRef: api-endpoint-behaviour #J5
func (s *QueryLogsServiceSuite) TestGetProfileQueryLogDevices() {
	ctx := context.Background()
	retention := model.RetentionOneWeek

	// Extra docs local to this test (SetupTest reseeds per test, so the shared
	// seed's count assertions elsewhere stay untouched): empty and missing
	// device_id must be excluded from the device list.
	now := time.Now()
	_, err := s.collMap[retention].InsertMany(ctx, []any{
		bson.D{{Key: "timestamp", Value: now.Add(-4 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "device_id", Value: ""}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "nodevice.example.com"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.8"}, {Key: "protocol", Value: "udp"}},
		bson.D{{Key: "timestamp", Value: now.Add(-5 * time.Hour)}, {Key: "profile_id", Value: s.profileID}, {Key: "status", Value: "processed"}, {Key: "reasons", Value: bson.A{}}, {Key: "dns_request", Value: bson.D{{Key: "domain", Value: "legacy.example.com"}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}}, {Key: "client_ip", Value: "1.2.3.9"}, {Key: "protocol", Value: "udp"}},
	})
	s.Require().NoError(err)

	devices, err := s.service.GetProfileQueryLogDevices(ctx, s.profileID, retention)
	s.Require().NoError(err)

	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, d.DeviceId)
	}
	// Sorted ascending; "laptop" present despite its newest doc being -2h and
	// oldest -25h (whole retention window, no timespan floor); no "" or
	// missing-field entries; other-profile's devices excluded.
	s.Equal([]string{"laptop", "phone", "tablet"}, ids)

	// last_seen carries the newest timestamp per device.
	for _, d := range devices {
		if d.DeviceId == "laptop" {
			s.WithinDuration(time.Now().Add(-2*time.Hour), d.LastSeen, time.Minute, "laptop last_seen should be its newest doc")
		}
		s.False(d.LastSeen.IsZero(), "last_seen must be set")
	}
}

// insertTopDocs inserts one log document per (domain, status, client_ip, age) row.
func (s *QueryLogsServiceSuite) insertTopDocs(profileID string, rows ...[4]any) {
	now := time.Now()
	docs := make([]any, 0, len(rows))
	for _, r := range rows {
		docs = append(docs, bson.D{
			{Key: "timestamp", Value: now.Add(-r[3].(time.Duration))},
			{Key: "profile_id", Value: profileID},
			{Key: "device_id", Value: "d"},
			{Key: "status", Value: r[1]},
			{Key: "reasons", Value: bson.A{}},
			{Key: "dns_request", Value: bson.D{{Key: "domain", Value: r[0]}, {Key: "query_type", Value: "A"}, {Key: "response_code", Value: "NOERROR"}, {Key: "dnssec", Value: false}}},
			{Key: "client_ip", Value: r[2]},
			{Key: "protocol", Value: "udp"},
		})
	}
	_, err := s.collMap[model.RetentionOneWeek].InsertMany(context.Background(), docs)
	s.Require().NoError(err)
}

// TestGetProfileQueryLogTopDomains verifies grouping per status, count desc
// then domain asc, the limit, the timespan floor, empty-domain exclusion and
// profile isolation.
// tableRef: api-endpoint-behaviour #J20
func (s *QueryLogsServiceSuite) TestGetProfileQueryLogTopDomains() {
	ctx := context.Background()
	retention := model.RetentionOneWeek
	h := time.Hour
	pid := "top-domains-profile"
	s.insertTopDocs(pid,
		[4]any{"b.com", "blocked", "1.1.1.1", 1 * h}, [4]any{"b.com", "blocked", "1.1.1.1", 2 * h}, [4]any{"b.com", "blocked", "1.1.1.1", 3 * h},
		[4]any{"a.com", "blocked", "1.1.1.1", 1 * h}, [4]any{"a.com", "blocked", "1.1.1.1", 2 * h}, [4]any{"a.com", "blocked", "1.1.1.1", 3 * h}, // ties with b.com: a before b
		[4]any{"c.com", "blocked", "1.1.1.1", 1 * h},
		[4]any{"old.com", "blocked", "1.1.1.1", 30 * h}, [4]any{"old.com", "blocked", "1.1.1.1", 31 * h}, [4]any{"old.com", "blocked", "1.1.1.1", 32 * h}, [4]any{"old.com", "blocked", "1.1.1.1", 33 * h}, // outside LAST_1_DAY
		[4]any{"", "blocked", "1.1.1.1", 1 * h}, // empty domain skipped
		[4]any{"ok.com", "processed", "1.1.1.1", 1 * h}, [4]any{"ok.com", "processed", "1.1.1.1", 2 * h},
		[4]any{"unavail.com", "unavailable", "1.1.1.1", 1 * h},
	)
	s.insertTopDocs("someone-else", [4]any{"a.com", "blocked", "9.9.9.9", 1 * h}, [4]any{"a.com", "blocked", "9.9.9.9", 1 * h})

	blocked, err := s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, model.LAST_1_DAY, model.QueryLogTopKindBlocked, 50)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "a.com", Count: 3}, {Domain: "b.com", Count: 3}, {Domain: "c.com", Count: 1}}, blocked)

	limited, err := s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, model.LAST_1_DAY, model.QueryLogTopKindBlocked, 2)
	s.Require().NoError(err)
	s.Equal(blocked[:2], limited)

	week, err := s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, model.LAST_7_DAYS, model.QueryLogTopKindBlocked, 1)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "old.com", Count: 4}}, week)

	resolved, err := s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, model.LAST_1_DAY, model.QueryLogTopKindResolved, 50)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopDomain{{Domain: "ok.com", Count: 2}}, resolved)

	none, err := s.service.GetProfileQueryLogTopDomains(ctx, "no-such-profile", retention, model.LAST_1_DAY, model.QueryLogTopKindBlocked, 50)
	s.Require().NoError(err)
	s.NotNil(none)
	s.Empty(none)

	_, err = s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, model.LAST_1_DAY, "bogus", 50)
	s.ErrorIs(err, ErrInvalidTopKind)
	_, err = s.service.GetProfileQueryLogTopDomains(ctx, pid, retention, "NOPE", model.QueryLogTopKindBlocked, 50)
	s.Error(err)
}

// TestGetProfileQueryLogTopClients verifies grouping by client IP across
// statuses, count desc then ip asc, the limit, the timespan floor and profile
// isolation.
// tableRef: api-endpoint-behaviour #J21
func (s *QueryLogsServiceSuite) TestGetProfileQueryLogTopClients() {
	ctx := context.Background()
	retention := model.RetentionOneWeek
	h := time.Hour
	pid := "top-clients-profile"
	s.insertTopDocs(pid,
		[4]any{"x.com", "blocked", "203.0.113.7", 1 * h}, [4]any{"x.com", "processed", "203.0.113.7", 2 * h}, [4]any{"y.com", "processed", "203.0.113.7", 3 * h},
		[4]any{"x.com", "processed", "198.51.100.2", 1 * h}, [4]any{"x.com", "processed", "198.51.100.2", 2 * h}, [4]any{"x.com", "processed", "198.51.100.2", 3 * h}, // ties: 198.* before 203.*
		[4]any{"x.com", "processed", "2001:db8::1", 1 * h},
		[4]any{"x.com", "processed", "192.0.2.9", 40 * h}, [4]any{"x.com", "processed", "192.0.2.9", 41 * h}, [4]any{"x.com", "processed", "192.0.2.9", 42 * h}, [4]any{"x.com", "processed", "192.0.2.9", 43 * h}, // outside LAST_1_DAY
		[4]any{"x.com", "processed", "", 1 * h}, // empty ip skipped
	)
	s.insertTopDocs("someone-else", [4]any{"x.com", "processed", "203.0.113.7", 1 * h})

	got, err := s.service.GetProfileQueryLogTopClients(ctx, pid, retention, model.LAST_1_DAY, 50)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopClient{{IP: "198.51.100.2", Count: 3}, {IP: "203.0.113.7", Count: 3}, {IP: "2001:db8::1", Count: 1}}, got)

	limited, err := s.service.GetProfileQueryLogTopClients(ctx, pid, retention, model.LAST_1_DAY, 1)
	s.Require().NoError(err)
	s.Equal(got[:1], limited)

	week, err := s.service.GetProfileQueryLogTopClients(ctx, pid, retention, model.LAST_7_DAYS, 1)
	s.Require().NoError(err)
	s.Equal([]model.QueryLogTopClient{{IP: "192.0.2.9", Count: 4}}, week)
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
