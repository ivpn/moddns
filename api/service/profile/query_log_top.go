package profile

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/ivpn/dns/api/internal/geoip"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
)

// Cache-aside for the top-domains, top-clients and top-blocklists aggregations (unindexable
// full-window bucket unpack — see api-endpoint-behaviour #J23).
const (
	queryLogTopCachePrefix = "logs:"
	queryLogTopCacheTTL    = 5 * time.Minute
	// An empty list is cached briefly: finding "nothing" still unpacks every bucket in the
	// window, while a fresh profile's first rows show within this.
	queryLogTopEmptyCacheTTL = 30 * time.Second
	// queryLogTopMissLimit bounds the aggregations (cache misses) one account may start per
	// minute across all its profiles; a page load misses at most four keys per timespan.
	queryLogTopMissLimit  = 40
	queryLogTopMissWindow = time.Minute
	// queryLogTopFetchLimit is the list size cached per key; a request's limit
	// is applied on read so one entry serves every limit.
	queryLogTopFetchLimit = 50
	queryLogClientsKind   = "clients"
	queryLogBlocklistKind = "blocklists"
)

// ClientEnricher resolves a client IP to ASN and country; *geoip.Enricher
// satisfies it.
type ClientEnricher interface {
	Lookup(ip string) geoip.Info
}

// SetClientEnricher wires the GeoIP enrichment of the top-clients list; a nil
// enricher leaves the enrichment fields null.
func (p *ProfileService) SetClientEnricher(e ClientEnricher) { p.clientEnricher = e }

func queryLogTopCacheKey(kind, profileId string, gen int64, timespan string) string {
	return queryLogTopCachePrefix + kind + ":" + profileId + ":g" + strconv.FormatInt(gen, 10) + ":" + timespan
}

// topEntry is a cached top list with the covered_from computed alongside it (J23).
type topEntry[T any] struct {
	Items       []T        `json:"items"`
	CoveredFrom *time.Time `json:"covered_from"`
}

// cachedTop returns the cached entry for key, or false on a miss, a cache error,
// a value in any other shape or an unknown generation (cacheable false).
func cachedTop[T any](ctx context.Context, p *ProfileService, cacheable bool, key string) (topEntry[T], bool) {
	var entry topEntry[T]
	if !cacheable {
		return entry, false
	}
	raw, err := p.Cache.Get(ctx, key)
	if err != nil || raw == "" {
		return entry, false
	}
	if json.Unmarshal([]byte(raw), &entry) != nil || entry.Items == nil {
		return topEntry[T]{}, false
	}
	return entry, true
}

// fetchCoveredFrom answers nil on a failed lookup; the lists still answer.
func (p *ProfileService) fetchCoveredFrom(ctx context.Context, profileId string, retention model.Retention, timespan string) (*time.Time, bool) {
	covered, err := p.QueryLogsService.GetProfileQueryLogCoveredFrom(ctx, profileId, retention, timespan)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to read query log coverage")
		return nil, false
	}
	return covered, true
}

// storeTop caches a list for the full TTL, or an empty one for queryLogTopEmptyCacheTTL.
func storeTop[T any](ctx context.Context, p *ProfileService, cacheable bool, key string, entry topEntry[T]) {
	if !cacheable {
		return
	}
	ttl := queryLogTopCacheTTL
	if len(entry.Items) == 0 {
		// Stored as [] so the entry reads back as a hit (cachedTop rejects null).
		entry.Items, ttl = []T{}, queryLogTopEmptyCacheTTL
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err := p.Cache.Set(ctx, key, raw, ttl); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to cache query log top list")
	}
}

// loadTop returns the cached entry for key or computes it with fetch. A computation counts
// against the account's miss limit, and concurrent misses of one key on this instance share
// a single computation. fetch reports whether its entry may be cached.
func loadTop[T any](ctx context.Context, p *ProfileService, accountId string, cacheable bool, key string, fetch func() (topEntry[T], bool, error)) (topEntry[T], error) {
	if entry, ok := cachedTop[T](ctx, p, cacheable, key); ok {
		return entry, nil
	}
	limiter := utils.IDLimiter{Cache: p.Cache, Label: "rate_limits", ID: accountId + ":query_logs_top", Max: queryLogTopMissLimit, Exp: queryLogTopMissWindow}
	if err := limiter.Tick(); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to tick query log top limiter")
	} else if !limiter.IsAllowed() {
		return topEntry[T]{}, ErrQueryLogsRateLimited
	}
	v, err, _ := p.topFlight.Do(key, func() (any, error) {
		entry, storable, err := fetch()
		if err != nil {
			return nil, err
		}
		if storable {
			storeTop(ctx, p, cacheable, key, entry)
		}
		return entry, nil
	})
	if err != nil {
		return topEntry[T]{}, err
	}
	return v.(topEntry[T]), nil
}

// GetProfileQueryLogTop returns the most frequent blocked or resolved domains
// in the profile's query logs. It answers {enabled:false} without querying
// unless both logging and domain logging are on.
func (p *ProfileService) GetProfileQueryLogTop(ctx context.Context, accountId, profileId, timespan, kind string, limit int) (*model.QueryLogTopDomains, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}
	logs := profile.Settings.Logs
	if logs == nil || !logs.Enabled || !logs.LogDomains {
		return &model.QueryLogTopDomains{Items: []model.QueryLogTopDomain{}}, nil
	}

	gen, cacheable := p.queryLogsCacheGeneration(ctx, profileId)
	key := queryLogTopCacheKey(kind, profileId, gen, timespan)
	entry, err := loadTop(ctx, p, accountId, cacheable, key, func() (entry topEntry[model.QueryLogTopDomain], covered bool, err error) {
		if entry.Items, err = p.QueryLogsService.GetProfileQueryLogTopDomains(ctx, profileId, logs.Retention, timespan, kind, queryLogTopFetchLimit); err != nil {
			return entry, false, err
		}
		entry.CoveredFrom, covered = p.fetchCoveredFrom(ctx, profileId, logs.Retention, timespan)
		return entry, covered, nil
	})
	if err != nil {
		return nil, err
	}
	items := entry.Items
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []model.QueryLogTopDomain{}
	}
	return &model.QueryLogTopDomains{Enabled: true, Items: items, CoveredFrom: entry.CoveredFrom, Timespan: timespan, Retention: logs.Retention}, nil
}

// GetProfileQueryLogClients returns the most frequent client IPs in the
// profile's query logs, enriched with ASN and country. It answers
// {enabled:false} without querying unless both logging and client-IP logging
// are on.
func (p *ProfileService) GetProfileQueryLogClients(ctx context.Context, accountId, profileId, timespan string, limit int) (*model.QueryLogTopClients, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}
	logs := profile.Settings.Logs
	if logs == nil || !logs.Enabled || !logs.LogClientsIPs {
		return &model.QueryLogTopClients{Items: []model.QueryLogTopClient{}}, nil
	}

	gen, cacheable := p.queryLogsCacheGeneration(ctx, profileId)
	key := queryLogTopCacheKey(queryLogClientsKind, profileId, gen, timespan)
	entry, err := loadTop(ctx, p, accountId, cacheable, key, func() (entry topEntry[model.QueryLogTopClient], covered bool, err error) {
		if entry.Items, err = p.QueryLogsService.GetProfileQueryLogTopClients(ctx, profileId, logs.Retention, timespan, queryLogTopFetchLimit); err != nil {
			return entry, false, err
		}
		entry.CoveredFrom, covered = p.fetchCoveredFrom(ctx, profileId, logs.Retention, timespan)
		return entry, covered, nil
	})
	if err != nil {
		return nil, err
	}
	items := entry.Items
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]model.QueryLogTopClient, len(items))
	for i, it := range items {
		var info geoip.Info
		if p.clientEnricher != nil {
			info = p.clientEnricher.Lookup(it.IP)
		}
		out[i] = model.QueryLogTopClient{IP: it.IP, Count: it.Count, ASN: info.ASN, ASOrg: info.ASOrg, Country: info.Country}
	}
	return &model.QueryLogTopClients{Enabled: true, Items: out, CoveredFrom: entry.CoveredFrom, Timespan: timespan, Retention: logs.Retention}, nil
}

// GetProfileQueryLogBlocklists returns the blocklists that blocked the most
// queries in the profile's query logs. It answers {enabled:false} without
// querying unless logging is on; reasons need neither domain nor client-IP logging.
func (p *ProfileService) GetProfileQueryLogBlocklists(ctx context.Context, accountId, profileId, timespan string, limit int) (*model.QueryLogTopBlocklists, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}
	logs := profile.Settings.Logs
	if logs == nil || !logs.Enabled {
		return &model.QueryLogTopBlocklists{Items: []model.QueryLogTopBlocklist{}}, nil
	}

	gen, cacheable := p.queryLogsCacheGeneration(ctx, profileId)
	key := queryLogTopCacheKey(queryLogBlocklistKind, profileId, gen, timespan)
	entry, err := loadTop(ctx, p, accountId, cacheable, key, func() (entry topEntry[model.QueryLogTopBlocklist], covered bool, err error) {
		if entry.Items, err = p.QueryLogsService.GetProfileQueryLogTopBlocklists(ctx, profileId, logs.Retention, timespan, queryLogTopFetchLimit); err != nil {
			return entry, false, err
		}
		p.nameBlocklists(ctx, entry.Items)
		entry.CoveredFrom, covered = p.fetchCoveredFrom(ctx, profileId, logs.Retention, timespan)
		return entry, covered, nil
	})
	if err != nil {
		return nil, err
	}
	items := entry.Items
	if len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		if items[i].Name == "" {
			items[i].Name = items[i].BlocklistID
		}
	}
	if items == nil {
		items = []model.QueryLogTopBlocklist{}
	}
	return &model.QueryLogTopBlocklists{Enabled: true, Items: items, CoveredFrom: entry.CoveredFrom, Timespan: timespan, Retention: logs.Retention}, nil
}

// nameBlocklists sets each item's catalogue name with one lookup. An id missing
// from the catalogue, or every id when the lookup fails, is named by itself.
func (p *ProfileService) nameBlocklists(ctx context.Context, items []model.QueryLogTopBlocklist) {
	if len(items) == 0 {
		return
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.BlocklistID
	}
	names := make(map[string]string, len(items))
	catalog, err := p.BlocklistService.GetBlocklist(ctx, map[string]any{"blocklist_ids": ids}, "")
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Int("blocklists", len(ids)).Msg("failed to resolve top blocklist names")
	}
	for _, b := range catalog {
		names[b.BlocklistID] = b.Name
	}
	for i := range items {
		items[i].Name = names[items[i].BlocklistID]
		if items[i].Name == "" {
			items[i].Name = items[i].BlocklistID
		}
	}
}
