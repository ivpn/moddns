package filter

// Tests for the settings-store error policy in DomainFilter.Execute and
// IPFilter.Execute: a failing stage yields StatusUnavailable instead of an
// accidental fail-open. Rows: docs/specs/proxy-filtering-behaviour.md Section I.
//
// Per-profile inputs (blocklist subscriptions, custom rules, blocked services,
// privacy settings) travel on the request context, so the only stage read that
// can fail is the live blocklist membership lookup (GetBlocklistEntry), used by
// the domain-phase blocklists stage and the IP-phase CNAME stage.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// stageErrorRecorder records (phase, stage) pairs; stages run concurrently so
// it is mutex-guarded.
type stageErrorRecorder struct {
	mu    sync.Mutex
	pairs [][2]string
}

func (r *stageErrorRecorder) RecordFilterStageError(phase, stage string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pairs = append(r.pairs, [2]string{phase, stage})
}

func (r *stageErrorRecorder) has(phase, stage string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.pairs {
		if p[0] == phase && p[1] == stage {
			return true
		}
	}
	return false
}

func (r *stageErrorRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pairs)
}

const stageErrBlocklistID = "bl-stage-err"

func stageErrReqCtx(t *testing.T, profileID string, privacy map[string]string) *requestcontext.RequestContext {
	t.Helper()
	logger := logging.NewFactory(zerolog.DebugLevel).ForProfile(profileID, true)
	return &requestcontext.RequestContext{
		ProfileId:       profileID,
		Blocklists:      []string{stageErrBlocklistID},
		PrivacySettings: privacy,
		Logger:          logger,
	}
}

func stageErrDomainDctx(qname string) *proxy.DNSContext {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), dns.TypeA)
	return &proxy.DNSContext{Req: msg}
}

// stageErrCNAMEDctx builds a resolved answer with a CNAME hop so the IP-phase
// CNAME stage performs a live blocklist lookup on the target.
func stageErrCNAMEDctx(qname string) *proxy.DNSContext {
	res := buildCNAMEChainResponse(qname, dns.TypeA, []string{"tracker.evil.net"}, "93.184.216.34")
	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn(qname), dns.TypeA)
	return &proxy.DNSContext{Req: req, Res: res}
}

// failingMembershipCache returns a mock whose blocklist membership lookup fails.
func failingMembershipCache() *mocks.Cache {
	mockCache := new(mocks.Cache)
	mockCache.On("GetBlocklistEntry", mock.Anything, stageErrBlocklistID, mock.Anything).
		Return(false, errStore).Maybe()
	return mockCache
}

// hasDecision reports whether any partial result carries the given decision at
// the given tier.
func hasDecision(results []model.StageResult, decision model.Decision, tier int) bool {
	for _, r := range results {
		if r.Decision == decision && r.Tier == tier {
			return true
		}
	}
	return false
}

var errStore = errors.New("dial tcp 10.0.0.6:6379: i/o timeout")

// specRef: proxy-filtering-behaviour.md #I1
// specRef: proxy-filtering-behaviour.md #I8
func TestDomainFilterExecute_StageError_Unavailable(t *testing.T) {
	const profileID = "stage-error-i1"
	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, failingMembershipCache(), nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(context.Background(), reqCtx, stageErrDomainDctx("example.com"))

	require.Error(t, err, "Execute must surface the stage error")
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status)
	assert.Nil(t, reqCtx.FilterResult.Reasons, "an unavailable result carries no reasons")
	assert.True(t, rec.has(FilterTypeDomain, StageBlocklists), "recorder pairs: %v", rec.pairs)
	assert.Equal(t, 1, rec.count(), "only the erroring stage is recorded")
	// Successful stages still report their (None) results for logging.
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionNone, TierCustomRules))
}

// specRef: proxy-filtering-behaviour.md #I2
func TestDomainFilterExecute_StageError_WinsOverBlock(t *testing.T) {
	const profileID = "stage-error-i2"
	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, failingMembershipCache(), nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	reqCtx.CustomRules = []map[string]string{{"action": ACTION_BLOCK, "value": "ads.example.com"}}
	err := f.Execute(context.Background(), reqCtx, stageErrDomainDctx("ads.example.com"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "a stage error must not be downgraded to the partial Block")
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionBlock, TierCustomRules),
		"the successful custom-rules Block is still recorded as a partial result")
	assert.True(t, rec.has(FilterTypeDomain, StageBlocklists))
}

// specRef: proxy-filtering-behaviour.md #I3
func TestDomainFilterExecute_StageError_WinsOverAllow(t *testing.T) {
	const profileID = "stage-error-i3"
	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, failingMembershipCache(), nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	reqCtx.CustomRules = []map[string]string{{"action": ACTION_ALLOW, "value": "ok.example.com"}}
	err := f.Execute(context.Background(), reqCtx, stageErrDomainDctx("ok.example.com"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "a stage error must not be downgraded to the partial Allow")
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionAllow, TierCustomRules))
	assert.True(t, rec.has(FilterTypeDomain, StageBlocklists))
}

// specRef: proxy-filtering-behaviour.md #I4
// specRef: proxy-filtering-behaviour.md #I8
func TestIPFilterExecute_StageError_DiscardsUpstreamAnswer(t *testing.T) {
	const profileID = "stage-error-i4"
	rec := &stageErrorRecorder{}
	// nil catalog/ASN lookup: filterServices is inert; nil rebinding config: inert.
	f := NewIPFilter(nil, failingMembershipCache(), nil, nil, nil, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	// Domain phase ran and produced a Processed (None) result.
	reqCtx.PartialFilteringResults = []model.StageResult{{Decision: model.DecisionNone, Tier: TierBlocklists}}
	reqCtx.FilterResult = model.FilterResult{Status: model.StatusProcessed}

	err := f.Execute(context.Background(), reqCtx, stageErrCNAMEDctx("metrics.shop.example"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "IP-phase store error must not fall through to Processed")
	assert.True(t, rec.has(FilterTypeIP, StageCNAME), "recorder pairs: %v", rec.pairs)
	assert.Equal(t, 1, rec.count())
}

// specRef: proxy-filtering-behaviour.md #I5
func TestExecute_NoErrors_NoMatches_Processed(t *testing.T) {
	const profileID = "stage-error-i5"
	rec := &stageErrorRecorder{}

	t.Run("domain phase", func(t *testing.T) {
		mockCache := mocks.NewCache(t)
		mockCache.EXPECT().GetBlocklistEntry(mock.Anything, stageErrBlocklistID, mock.Anything).Return(false, nil).Maybe()

		f := NewDomainFilter(nil, mockCache, nil)
		f.Metrics = rec
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

		err := f.Execute(context.Background(), reqCtx, stageErrDomainDctx("example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	})

	t.Run("ip phase", func(t *testing.T) {
		// No CNAME in the answer: the IP phase never touches the store.
		f := NewIPFilter(nil, mocks.NewCache(t), nil, nil, nil, nil)
		f.Metrics = rec
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

		err := f.Execute(context.Background(), reqCtx, dnsCtxWithAAnswer(t, "93.184.216.34"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	})

	assert.Equal(t, 0, rec.count(), "no stage error must be recorded when nothing failed")
}

// specRef: proxy-filtering-behaviour.md #I7
func TestIPFilterExecute_CatalogUnavailable_Inert(t *testing.T) {
	const (
		profileID = "stage-error-i7"
		asn       = uint(15169)
	)
	rec := &stageErrorRecorder{}
	f := NewIPFilter(nil, mocks.NewCache(t), staticCatalogErr{err: errors.New("catalog load")}, staticASNLookup{asn: asn}, nil, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	reqCtx.BlockedServices = []string{"google"}
	err := f.Execute(context.Background(), reqCtx, dnsCtxWithAAnswer(t, "8.8.8.8"))

	require.NoError(t, err, "a local catalog load failure is not a store error")
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	assert.Equal(t, 0, rec.count())
}

// Execute must tolerate a nil metrics recorder (tests and tools construct
// filters without one).
// specRef: proxy-filtering-behaviour.md #I1
func TestDomainFilterExecute_StageError_NilRecorder(t *testing.T) {
	const profileID = "stage-error-nil-recorder"
	f := NewDomainFilter(nil, failingMembershipCache(), nil)
	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

	require.NotPanics(t, func() { _ = f.Execute(context.Background(), reqCtx, stageErrDomainDctx("example.com")) })
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status)
}

// applyDefaultRule reads the privacy settings already carried by the request
// context; a strict mock with no expectations fails the test if the stage
// reaches for the store.
// specRef: proxy-filtering-behaviour.md #I5
func TestApplyDefaultRule_ReadsRequestContext_NoCacheCall(t *testing.T) {
	const profileID = "default-rule-no-redis"
	f := NewDomainFilter(nil, mocks.NewCache(t), nil)

	// No blocklist subscriptions, so the blocklists stage makes no lookups either.
	noBlocklists := func(privacy map[string]string) *requestcontext.RequestContext {
		reqCtx := stageErrReqCtx(t, profileID, privacy)
		reqCtx.Blocklists = nil
		return reqCtx
	}

	t.Run("stage direct", func(t *testing.T) {
		res, err := f.applyDefaultRule(context.Background(), noBlocklists(map[string]string{DEFAULT_RULE: RULE_BLOCK}), stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.DecisionBlock, res.Decision)
		assert.Equal(t, TierDefaultRule, res.Tier)
		assert.Contains(t, res.Reasons, DEFAULT_RULE)
	})

	t.Run("stage direct allow", func(t *testing.T) {
		res, err := f.applyDefaultRule(context.Background(), noBlocklists(map[string]string{DEFAULT_RULE: RULE_ALLOW}), stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.DecisionNone, res.Decision)
	})

	t.Run("through Execute", func(t *testing.T) {
		reqCtx := noBlocklists(map[string]string{DEFAULT_RULE: RULE_BLOCK})
		err := f.Execute(context.Background(), reqCtx, stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status)
		assert.Contains(t, reqCtx.FilterResult.Reasons, DEFAULT_RULE)
	})
}

// The whole filter path — both phases, every stage armed — reaches the store
// only for blocklist membership. Every other per-profile input comes from the
// settings batch on the request context (Section I note). A strict mockery
// mock fails the test on any call without an expectation.
// specRef: proxy-filtering-behaviour.md #I8
func TestFilterPath_OnlyBlocklistMembershipHitsStore(t *testing.T) {
	const (
		profileID = "store-boundary"
		asn       = uint(15169)
		answerIP  = "93.184.216.34"
	)
	blocklists := []string{"bl1", "bl2"}

	mockCache := mocks.NewCache(t)
	for _, bl := range blocklists {
		mockCache.EXPECT().GetBlocklistEntry(mock.Anything, bl, mock.Anything).Return(false, nil)
	}

	domainFilter := NewDomainFilter(nil, mockCache, staticCatalog{cat: googleCatalogWithASN(asn)})
	ipFilter := NewIPFilter(nil, mockCache, staticCatalog{cat: googleCatalogWithASN(asn)}, staticASNLookup{asn: asn + 1}, nil, nil)

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{SUBDOMAINS_RULE: RULE_BLOCK})
	reqCtx.Blocklists = blocklists
	reqCtx.BlockedServices = []string{"google"}
	reqCtx.CustomRules = []map[string]string{
		{"action": ACTION_BLOCK, "value": "ads.other.example", "syntax": "domain"},
		{"action": ACTION_ALLOW, "value": "10.9.8.7", "syntax": "ip4_addr"},
		{"action": ACTION_ALLOW, "value": "AS64496", "syntax": "asn"},
	}

	// Domain phase: QNAME plus parent-walk candidates, each against both lists.
	dctx := stageErrDomainDctx("www.shop.example.com")
	require.NoError(t, domainFilter.Execute(context.Background(), reqCtx, dctx))
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)

	// IP phase with a CNAME hop: only the target's membership is looked up.
	dctx.Res = buildCNAMEChainResponse("www.shop.example.com", dns.TypeA, []string{"edge.cdn.example"}, answerIP)
	require.NoError(t, ipFilter.Execute(context.Background(), reqCtx, dctx))
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)

	for _, call := range mockCache.Calls {
		assert.Equal(t, "GetBlocklistEntry", call.Method, "only blocklist membership may reach the store")
	}
	assert.NotEmpty(t, mockCache.Calls, "the membership lookup itself must still be live")
}
