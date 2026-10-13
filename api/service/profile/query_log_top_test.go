package profile_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"

	"github.com/ivpn/dns/api/cache"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/geoip"
	"github.com/ivpn/dns/api/model"
)

// topCovered is the covered_from the repository reports in these tests (J20).
var topCovered = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

const topCoveredJSON = `"covered_from":"2026-10-12T08:00:00Z"`

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
	// No generation stored yet: entries are read and written under generation 0.
	suite.mockCache.On("Get", mock.Anything, cache.GenerationKey("logs", "profile123")).Return("", redis.Nil).Maybe()
}

// tableRef: api-endpoint-behaviour #J20
func (suite *ProfileTestSuite) TestGetProfileQueryLogTop() {
	ctx := context.Background()
	on := &model.LogsSettings{Enabled: true, LogDomains: true, Retention: model.RetentionOneWeek}
	key := "logs:blocked:profile123:g0:LAST_7_DAYS"
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
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)
		var cached []byte
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).
			Run(func(args mock.Arguments) { cached = args.Get(2).([]byte) }).Return(nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 2)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.Equal(repoItems[:2], got.Items)
		suite.Require().NotNil(got.CoveredFrom)
		suite.True(topCovered.Equal(*got.CoveredFrom))
		// tableRef: api-endpoint-behaviour #J23
		var entry struct {
			Items       []model.QueryLogTopDomain `json:"items"`
			CoveredFrom *time.Time                `json:"covered_from"`
		}
		suite.Require().NoError(json.Unmarshal(cached, &entry))
		suite.Equal(repoItems, entry.Items, "the full list is cached")
		suite.Require().NotNil(entry.CoveredFrom, "covered_from is cached with the list")
		suite.True(topCovered.Equal(*entry.CoveredFrom))
	})

	suite.Run("kind resolved maps to processed status and its own key", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, "logs:resolved:profile123:g0:LAST_7_DAYS").Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "processed", 168, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)
		suite.mockCache.On("Set", ctx, "logs:resolved:profile123:g0:LAST_7_DAYS", mock.Anything, 5*time.Minute).Return(nil)

		_, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "resolved", 10)
		suite.NoError(err)
	})

	suite.Run("hit skips the aggregation and still honours the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`{"items":[{"domain":"a.com","count":9},{"domain":"b.com","count":5}],`+topCoveredJSON+`}`, nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 1)
		suite.NoError(err)
		suite.Equal([]model.QueryLogTopDomain{{Domain: "a.com", Count: 9}}, got.Items)
		suite.Require().NotNil(got.CoveredFrom, "covered_from comes from the cache entry")
		suite.True(topCovered.Equal(*got.CoveredFrom))
		suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopDomains", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogCoveredFrom", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	// tableRef: api-endpoint-behaviour #J23
	suite.Run("a cache entry in the bare-list shape reads as a miss", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`[{"domain":"a.com","count":9}]`, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).Return(nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.Equal(repoItems, got.Items)
		suite.NotNil(got.CoveredFrom)
	})

	suite.Run("a failed covered_from lookup answers null and is not cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(nil, errors.New("boom"))

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.Equal(repoItems, got.Items)
		suite.Nil(got.CoveredFrom)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("empty result is returned as [] and never cached", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(nil, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.NotNil(got.Items)
		suite.Empty(got.Items)
		suite.NotNil(got.CoveredFrom, "other rows can cover the window with no blocked domain")
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
	key := "logs:clients:profile123:g0:LAST_1_DAY"
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
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneDay, 24).Return(&topCovered, nil)
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
		suite.Require().NotNil(got.CoveredFrom)
		suite.True(topCovered.Equal(*got.CoveredFrom))
		suite.Contains(string(cached), topCoveredJSON)
		suite.False(strings.Contains(string(cached), "GOOGLE") || strings.Contains(string(cached), "15169"), "cache must hold counts only")
	})

	suite.Run("hit is enriched with the current databases and trimmed to the limit", func() {
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`{"items":[{"ip":"8.8.8.8","count":40,"asn":null,"as_org":null,"country":null},{"ip":"203.0.113.7","count":3,"asn":null,"as_org":null,"country":null}],`+topCoveredJSON+`}`, nil)

		got, err := suite.service.GetProfileQueryLogClients(ctx, "account123", "profile123", model.LAST_1_DAY, 1)
		suite.NoError(err)
		suite.Len(got.Items, 1)
		suite.Equal(&cc, got.Items[0].Country)
		suite.NotNil(got.CoveredFrom)
	})

	suite.Run("no enricher wired leaves the fields null", func() {
		suite.service.SetClientEnricher(nil)
		suite.resetTopMocks()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`{"items":[{"ip":"8.8.8.8","count":40}],"covered_from":null}`, nil)

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
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneDay, 24).Return(nil, nil)

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
	suite.mockCache.On("Incr", ctx, cache.GenerationKey("logs", "profile123"), cache.GenerationTTL).Return(int64(1), nil).Once()

	suite.NoError(suite.service.DeleteProfileQueryLogs(ctx, "account123", "profile123"))

	// One bump retires the devices list and every top list; no per-key deletes (J23).
	suite.mockCache.AssertNotCalled(suite.T(), "Del", mock.Anything, mock.Anything)
}

// tableRef: api-endpoint-behaviour #J23
func (suite *ProfileTestSuite) TestQueryLogTopCacheGeneration() {
	ctx := context.Background()
	on := &model.LogsSettings{Enabled: true, LogDomains: true, Retention: model.RetentionOneWeek}
	repoItems := []model.QueryLogTopDomain{{Domain: "a.com", Count: 9}}

	suite.Run("a list whose query started before a purge is written under the old generation", func() {
		suite.resetTopMocks()
		suite.mockCache.ExpectedCalls = nil
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, cache.GenerationKey("logs", "profile123")).Return("2", nil).Once()
		suite.mockCache.On("Get", ctx, "logs:blocked:profile123:g2:LAST_7_DAYS").Return("", redis.Nil)
		// A purge on another instance bumps the generation while the aggregation runs.
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).
			Run(func(mock.Arguments) {
				suite.mockCache.On("Get", ctx, cache.GenerationKey("logs", "profile123")).Return("3", nil)
			}).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)
		suite.mockCache.On("Set", ctx, "logs:blocked:profile123:g2:LAST_7_DAYS", mock.Anything, 5*time.Minute).Return(nil).Once()

		_, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.mockCache.AssertNumberOfCalls(suite.T(), "Set", 1)
	})

	suite.Run("an unreadable generation skips the cache both ways", func() {
		suite.resetTopMocks()
		suite.mockCache.ExpectedCalls = nil
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, cache.GenerationKey("logs", "profile123")).Return("", errors.New("redis down")).Once()
		suite.mockQueryLogsRepo.On("GetQueryLogTopDomains", ctx, "profile123", model.RetentionOneWeek, "blocked", 168, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 168).Return(&topCovered, nil)

		got, err := suite.service.GetProfileQueryLogTop(ctx, "account123", "profile123", model.LAST_7_DAYS, "blocked", 10)
		suite.NoError(err)
		suite.Equal(repoItems, got.Items)
		suite.mockCache.AssertNumberOfCalls(suite.T(), "Get", 1)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// tableRef: api-endpoint-behaviour #J24
// tableRef: api-endpoint-behaviour #J23
func (suite *ProfileTestSuite) TestGetProfileQueryLogBlocklists() {
	ctx := context.Background()
	key := "logs:blocklists:profile123:g0:LAST_1_DAY"
	repoItems := []model.QueryLogTopBlocklist{{BlocklistID: "oisd", Count: 9}, {BlocklistID: "hagezi_pro", Count: 5}, {BlocklistID: "gone", Count: 1}}
	catalogFilter := map[string]any{"blocklist_ids": []string{"oisd", "hagezi_pro", "gone"}}
	catalog := []*model.Blocklist{{BlocklistID: "hagezi_pro", Name: "Hagezi Pro"}, {BlocklistID: "oisd", Name: "OISD Big"}}
	named := []model.QueryLogTopBlocklist{
		{BlocklistID: "oisd", Name: "OISD Big", Count: 9}, {BlocklistID: "hagezi_pro", Name: "Hagezi Pro", Count: 5}, {BlocklistID: "gone", Name: "gone", Count: 1},
	}
	reset := func() {
		suite.resetTopMocks()
		suite.mockBlocklistRepo.ExpectedCalls = nil
		suite.mockBlocklistRepo.Calls = nil
	}
	// cachedSet captures the value written to the top cache.
	cachedSet := func() *string {
		var raw string
		suite.mockCache.On("Set", ctx, key, mock.Anything, 5*time.Minute).
			Run(func(args mock.Arguments) { raw = string(args.Get(2).([]byte)) }).Return(nil)
		return &raw
	}

	suite.Run("gate is logs.enabled only", func() {
		for name, logs := range map[string]*model.LogsSettings{
			"logs disabled":     {Enabled: false, LogDomains: true, LogClientsIPs: true, Retention: model.RetentionOneDay},
			"nil logs settings": nil,
		} {
			reset()
			suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(logs), nil)

			got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
			suite.NoError(err, name)
			suite.False(got.Enabled, name)
			suite.NotNil(got.Items, name)
			suite.Empty(got.Items, name)
			suite.mockCache.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
			suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopBlocklists", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			suite.mockBlocklistRepo.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything, mock.Anything)
		}

		reset()
		bare := &model.LogsSettings{Enabled: true, Retention: model.RetentionOneDay}
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(bare), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneDay, 24, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneDay, 24).Return(&topCovered, nil)
		suite.mockBlocklistRepo.On("Get", ctx, catalogFilter, "updated").Return(catalog, nil)
		cachedSet()

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled, "neither domain nor client IP logging is needed")
		suite.Equal(named, got.Items)
	})

	on := &model.LogsSettings{Enabled: true, Retention: model.RetentionOneWeek}

	suite.Run("miss resolves names in one lookup before caching, falls back to the id and trims to the limit", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 24).Return(&topCovered, nil)
		suite.mockBlocklistRepo.On("Get", ctx, catalogFilter, "updated").Return(catalog, nil).Once()
		raw := cachedSet()

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 2)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.Equal(named[:2], got.Items)
		suite.Require().NotNil(got.CoveredFrom)
		suite.True(topCovered.Equal(*got.CoveredFrom))
		var cached struct {
			Items       []model.QueryLogTopBlocklist `json:"items"`
			CoveredFrom *time.Time                   `json:"covered_from"`
		}
		suite.Require().NoError(json.Unmarshal([]byte(*raw), &cached))
		suite.Equal(named, cached.Items, "the cached list carries the names")
		suite.NotNil(cached.CoveredFrom)
	})

	suite.Run("catalog error falls back to the id and still answers", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(repoItems, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 24).Return(&topCovered, nil)
		suite.mockBlocklistRepo.On("Get", ctx, catalogFilter, "updated").Return(nil, errors.New("mongo down"))
		cachedSet()

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		for _, it := range got.Items {
			suite.Equal(it.BlocklistID, it.Name)
		}
		suite.Len(got.Items, 3)
	})

	suite.Run("hit skips both lookups, honours the limit and names a nameless row by its id", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return(`{"items":[{"blocklist_id":"oisd","name":"OISD Big","count":9},{"blocklist_id":"hagezi_pro","count":5}],`+topCoveredJSON+`}`, nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 2)
		suite.NoError(err)
		suite.Equal([]model.QueryLogTopBlocklist{{BlocklistID: "oisd", Name: "OISD Big", Count: 9}, {BlocklistID: "hagezi_pro", Name: "hagezi_pro", Count: 5}}, got.Items)
		suite.NotNil(got.CoveredFrom)
		suite.mockQueryLogsRepo.AssertNotCalled(suite.T(), "GetQueryLogTopBlocklists", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		suite.mockBlocklistRepo.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("empty result is returned as [] without a catalog lookup and never cached", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(nil, nil)
		suite.mockQueryLogsRepo.On("GetQueryLogCoveredFrom", ctx, "profile123", model.RetentionOneWeek, 24).Return(nil, nil)

		got, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.NoError(err)
		suite.True(got.Enabled)
		suite.NotNil(got.Items)
		suite.Empty(got.Items)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		suite.mockBlocklistRepo.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("aggregation error is returned and not cached", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)
		suite.mockCache.On("Get", ctx, key).Return("", errors.New("redis: nil"))
		suite.mockQueryLogsRepo.On("GetQueryLogTopBlocklists", ctx, "profile123", model.RetentionOneWeek, 24, 50).Return(nil, errors.New("boom"))

		_, err := suite.service.GetProfileQueryLogBlocklists(ctx, "account123", "profile123", model.LAST_1_DAY, 10)
		suite.Error(err)
		suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	suite.Run("foreign profile is not found", func() {
		reset()
		suite.mockProfileRepo.On("GetProfileById", ctx, "profile123").Return(suite.topProfile(on), nil)

		_, err := suite.service.GetProfileQueryLogBlocklists(ctx, "other-account", "profile123", model.LAST_1_DAY, 10)
		suite.ErrorIs(err, dbErrors.ErrProfileNotFound)
	})
}
