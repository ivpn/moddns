package profile_test

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stretchr/testify/mock"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/geoip"
	"github.com/ivpn/dns/api/model"
)

type stubEnricher map[string]geoip.Info

func (s stubEnricher) Lookup(ip string) geoip.Info { return s[ip] }

func (suite *ProfileTestSuite) topProfile(logs *model.LogsSettings) *model.Profile {
	return &model.Profile{ProfileId: "profile123", AccountId: "account123", Settings: &model.ProfileSettings{Logs: logs}}
}

func (suite *ProfileTestSuite) resetTopMocks() {
	suite.mockProfileRepo.ExpectedCalls = nil
	suite.mockQueryLogsRepo.ExpectedCalls = nil
	suite.mockCache.ExpectedCalls = nil
	suite.mockQueryLogsRepo.Calls = nil
	suite.mockCache.Calls = nil
}

// tableRef: api-endpoint-behaviour #J20
func (suite *ProfileTestSuite) TestGetProfileQueryLogTop() {
	ctx := context.Background()
	on := &model.LogsSettings{Enabled: true, LogDomains: true, Retention: model.RetentionOneWeek}
	key := "logs:blocked:profile123:LAST_7_DAYS"
	repoItems := []model.QueryLogTopDomain{{Domain: "a.com", Count: 9}, {Domain: "b.com", Count: 5}, {Domain: "c.com", Count: 1}}

	suite.Run("gate off answers enabled=false without cache or query", func() {
		for name, logs := range map[string]*model.LogsSettings{
			"logs disabled":       {Enabled: false, LogDomains: true, Retention: model.RetentionOneWeek},
			"domains not logged":  {Enabled: true, LogDomains: false, Retention: model.RetentionOneWeek},
			"nil logs settings":   nil,
			"clients only logged": {Enabled: true, LogClientsIPs: true, Retention: model.RetentionOneWeek},
		} {
			suite.resetTopMocks()
			suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(logs), nil)

			got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
			suite.NoError(err, name)
			suite.False(got.Enabled, name)
			suite.NotNil(got.Items, name)
			suite.Empty(got.Items, name)
			suite.mockCache.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
			suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopDomains", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		}
	})

	suite.Run("miss aggregates the full list, caches it and trims to the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		// Retention and kind forwarded; the fetch size is the cached maximum, not the request limit.
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(repoItems, nil)
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).Return(nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 2)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.Equal(repoItems[:2], got.Items)
	})

	suite.Run("kind resolved maps to processed status and its own key", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, "logs:resolved:profile123:LAST_7_DAYS").Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "processed", 168, 50).Return(repoItems, nil)
		suite.mockCache.On("Set", ctx, "logs:resolved:profile123:LAST_7_DAYS", mock.Anything, 5*time.Minute).Return(nil)

		_, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "resolved", 10)
		suite.NoError(err)
	})

	suite.Run("hit skips the aggregation and still honours the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`[{"domain":"a.com","count":9},{"domain":"b.com","count":5}]`, nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 1)
		suite.NoError(err)
		suite.Equal([]model.QueryLogTopDomain{{Domain: "a.com", Count: 9}}, got.Items)
		suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopDomains", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("empty result is returned as [] and never cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(nil, nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.NotNil(got.Items)
		suite.Empty(got.Items)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("aggregation error is returned and not cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(nil, errors.New("boom"))

		_, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.Error(err)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("foreign profile is not found", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)

		_, err := suite.service.GetProfileQueryLogTop(ctx, "other-account", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.ErrorIs(err, dbErrors.ErrProfileNotFound)
	})
}

// tableRef: api-endpoint-behaviour #J21
func (suite *ProfileTestSuite) TestGetProfileQueryLogClients() {
	ctx := context.Background()
	on := &model.LogsSettings{Enabled: true, LogClientsIPs: true, Retention: model.RetentionOneDay}
	key := "logs:clients:profile123:LAST_1_DAY"
	asn, org, cc := uint32(15169), "GOOGLE", "US"
	suite.service.SetClientEnricher(stubEnricher{"8.8.8.8": {ASN: &asn, ASOrg: &org, Country: &cc}})
	defer suite.service.SetClientEnricher(nil)

	suite.Run("gate off answers enabled=false without cache or query", func() {
		for name, logs := range map[string]*model.LogsSettings{
			"logs disabled":     {Enabled: false, LogClientsIPs: true, Retention: model.RetentionOneDay},
			"ips not logged":    {Enabled: true, LogClientsIPs: false, LogDomains: true, Retention: model.RetentionOneDay},
			"nil logs settings": nil,
		} {
			suite.resetTopMocks()
			suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(logs), nil)

			got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
			suite.NoError(err, name)
			suite.False(got.Enabled, name)
			suite.NotNil(got.Items, name)
			suite.Empty(got.Items, name)
			suite.mockCache.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
		}
	})

	suite.Run("enrichment is applied after the cache and never cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopClients", ctx, "profile123", model.RetentionOneDay, 24, 50).
			Return([]model.QueryLogTopClient{{IP: "8.8.8.8", Count: 40}, {IP: "203.0.113.7", Count: 3}}, nil)
		var cached []byte
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).
			Run(func(args mock.Arguments) { cached = args.Get(2).([]byte) }).Return(nil)

		got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.Len(got.Items, 2)
		suite.Equal(&asn, got.Items[0].ASN)
		suite.Equal(&org, got.Items[0].ASOrg)
		suite.Equal(&cc, got.Items[0].Country)
		suite.Nil(got.Items[1].ASN)
		suite.Nil(got.Items[1].ASOrg)
		suite.Nil(got.Items[1].Country)
		suite.False(strings.Contains(string(cached), "GOOGLE") || strings.Contains(string(cached), "15169"), "cache must hold counts only")
	})

	suite.Run("hit is enriched with the current databases and trimmed to the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`[{"ip":"8.8.8.8","count":40,"asn":null,"as_org":null,"country":null},{"ip":"203.0.113.7","count":3,"asn":null,"as_org":null,"country":null}]`, nil)

		got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 1)
		suite.NoError(err)
		suite.Len(got.Items, 1)
		suite.Equal(&cc, got.Items[0].Country)
	})

	suite.Run("no enricher wired leaves the fields null", func() {
		suite.service.SetClientEnricher(nil)
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`[{"ip":"8.8.8.8","count":40}]`, nil)

		got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.Len(got.Items, 1)
		suite.Nil(got.Items[0].ASN)
	})

	suite.Run("empty result is never cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopClients", ctx, "profile123", model.RetentionOneDay, 24, 50).Return(nil, nil)

		got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.NotNil(got.Items)
		suite.Empty(got.Items)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// tableRef: api-endpoint-behaviour #J23
// tableRef: api-endpoint-behaviour #J24
func (suite *ProfileTestSuite) TestDeleteProfileQueryLogsInvalidatesTopCache() {
	ctx := context.Background()
	suite.resetTopMocks()
	suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(&model.LogsSettings{Retention: model.RetentionOneDay}), nil)
	suite.mockQueryLogsRepo.On("DeleteQueryLogs", ctx, "profile123").Return(nil)
	deleted := map[string]bool{}
	suite.mockCache.On("Del", ctx, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) { deleted[args.String(1)] = true }).Return(nil)

	suite.NoError(suite.service.DeleteProfileQueryLogs(ctx, "account123", "profile123"))

	for _, kind := range []string{"blocked", "resolved", "clients", "blocklists"} {
		for _, ts := range []string{"LAST_1_HOUR", "LAST_3_HOURS", "LAST_6_HOURS", "LAST_12_HOURS", "LAST_1_DAY", "LAST_7_DAYS", "LAST_MONTH"} {
			suite.True(deleted["logs:"+kind+":profile123:"+ts], kind+" "+ts)
		}
	}
	suite.True(deleted["query_log_devices:profile123"])
}

// tableRef: api-endpoint-behaviour #J24
// tableRef: api-endpoint-behaviour #J23
func (suite *ProfileTestSuite) TestGetProfileQueryLogBlocklists() {
	ctx := context.Background()
	key := "logs:blocklists:profile123:LAST_1_DAY"
	repoItems := []model.QueryLogTopBlocklist{{BlocklistID: "oisd", Count: 9}, {BlocklistID: "hagezi_pro", Count: 5}, {BlocklistID: "adguard", Count: 1}}

	suite.Run("gate is logs.enabled only", func() {
		for name, logs := range map[string]*model.LogsSettings{
			"logs disabled":     {Enabled: false, LogDomains: true, LogClientsIPs: true, Retention: model.RetentionOneDay},
			"nil logs settings": nil,
		} {
			suite.resetTopMocks()
			suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(logs), nil)

			got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
			suite.NoError(err, name)
			suite.False(got.Enabled, name)
			suite.NotNil(got.Items, name)
			suite.Empty(got.Items, name)
			suite.mockCache.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
			suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopBlocklists", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		}

		suite.resetTopMocks()
		bare := &model.LogsSettings{Enabled: true, Retention: model.RetentionOneDay}
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(bare), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneDay, 24, 50).Return(repoItems, nil)
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).Return(nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled, "neither domain nor client IP logging is needed")
		suite.Equal(repoItems, got.Items)
	})

	on := &model.LogsSettings{Enabled: true, Retention: model.RetentionOneWeek}

	suite.Run("miss aggregates the full list from the current retention, caches it and trims to the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(repoItems, nil)
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).Return(nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 2)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.Equal(repoItems[:2], got.Items)
	})

	suite.Run("hit skips the aggregation and still honours the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`[{"blocklist_id":"oisd","count":9},{"blocklist_id":"hagezi_pro","count":5}]`, nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 1)
		suite.NoError(err)
		suite.Equal([]model.QueryLogTopBlocklist{{BlocklistID: "oisd", Count: 9}}, got.Items)
		suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopBlocklists", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("empty result is returned as [] and never cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(nil, nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.NotNil(got.Items)
		suite.Empty(got.Items)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("aggregation error is returned and not cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(nil, errors.New("boom"))

		_, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.Error(err)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("foreign profile is not found", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)

		_, err := suite.service.GetProfileQueryLogBlocklists(ctx, "other-account", "profile123", model.LAST_1_DAY, 10)
		suite.ErrorIs(err, dbErrors.ErrProfileNotFound)
	})
}
