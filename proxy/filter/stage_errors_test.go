package filter

// Tests for the settings-store error policy in DomainFilter.Execute and
// IPFilter.Execute: a failing stage yields StatusUnavailable instead of an
// accidental fail-open. Rows: docs/specs/proxy-filtering-behaviour.md Section I.

import (
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

func stageErrReqCtx(t *testing.T, profileID string, privacy map[string]string) *requestcontext.RequestContext {
	t.Helper()
	logger := logging.NewFactory(zerolog.DebugLevel).ForProfile(profileID, true)
	return &requestcontext.RequestContext{
		ProfileId:       profileID,
		PrivacySettings: privacy,
		Logger:          logger,
	}
}

func stageErrDomainDctx(qname string) *proxy.DNSContext {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), dns.TypeA)
	return &proxy.DNSContext{Req: msg}
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
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileBlocklists", mock.Anything, profileID).Return(nil, errStore).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, mockCache, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(reqCtx, stageErrDomainDctx("example.com"))

	require.Error(t, err, "Execute must surface the stage error")
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status)
	assert.Nil(t, reqCtx.FilterResult.Reasons, "an unavailable result carries no reasons")
	assert.True(t, rec.has(FilterTypeDomain, "blocklists"), "recorder pairs: %v", rec.pairs)
	assert.Equal(t, 1, rec.count(), "only the erroring stage is recorded")
	// Successful stages still report their (None) results for logging.
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionNone, TierCustomRules))
}

// specRef: proxy-filtering-behaviour.md #I2
func TestDomainFilterExecute_StageError_WinsOverBlock(t *testing.T) {
	const profileID = "stage-error-i2"
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileBlocklists", mock.Anything, profileID).Return(nil, errStore).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{"rule-block"}, nil).Maybe()
	mockCache.On("GetCustomRulesHash", mock.Anything, "rule-block").
		Return(map[string]string{"action": ACTION_BLOCK, "value": "ads.example.com"}, nil).Maybe()

	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, mockCache, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(reqCtx, stageErrDomainDctx("ads.example.com"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "a stage error must not be downgraded to the partial Block")
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionBlock, TierCustomRules),
		"the successful custom-rules Block is still recorded as a partial result")
	assert.True(t, rec.has(FilterTypeDomain, "blocklists"))
}

// specRef: proxy-filtering-behaviour.md #I3
func TestDomainFilterExecute_StageError_WinsOverAllow(t *testing.T) {
	const profileID = "stage-error-i3"
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileBlocklists", mock.Anything, profileID).Return(nil, errStore).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{"rule-allow"}, nil).Maybe()
	mockCache.On("GetCustomRulesHash", mock.Anything, "rule-allow").
		Return(map[string]string{"action": ACTION_ALLOW, "value": "ok.example.com"}, nil).Maybe()

	rec := &stageErrorRecorder{}
	f := NewDomainFilter(nil, mockCache, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(reqCtx, stageErrDomainDctx("ok.example.com"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "a stage error must not be downgraded to the partial Allow")
	assert.True(t, hasDecision(reqCtx.PartialFilteringResults, model.DecisionAllow, TierCustomRules))
	assert.True(t, rec.has(FilterTypeDomain, "blocklists"))
}

// specRef: proxy-filtering-behaviour.md #I4
// specRef: proxy-filtering-behaviour.md #I8
func TestIPFilterExecute_StageError_DiscardsUpstreamAnswer(t *testing.T) {
	const profileID = "stage-error-i4"
	mockCache := new(mocks.Cache)
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return(nil, errStore).Maybe()

	rec := &stageErrorRecorder{}
	// nil catalog/ASN lookup: filterServices is inert; nil rebinding config: inert.
	f := NewIPFilter(nil, mockCache, nil, nil, nil, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	// Domain phase ran and produced a Processed (None) result.
	reqCtx.PartialFilteringResults = []model.StageResult{{Decision: model.DecisionNone, Tier: TierBlocklists}}
	reqCtx.FilterResult = model.FilterResult{Status: model.StatusProcessed}

	err := f.Execute(reqCtx, dnsCtxWithAAnswer(t, "93.184.216.34"))

	require.Error(t, err)
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status, "IP-phase store error must not fall through to Processed")
	assert.True(t, rec.has(FilterTypeIP, "custom_rules"), "recorder pairs: %v", rec.pairs)
	assert.Equal(t, 1, rec.count())
}

// specRef: proxy-filtering-behaviour.md #I5
func TestExecute_NoErrors_NoMatches_Processed(t *testing.T) {
	const profileID = "stage-error-i5"
	rec := &stageErrorRecorder{}

	t.Run("domain phase", func(t *testing.T) {
		mockCache := new(mocks.Cache)
		mockCache.On("GetProfileBlocklists", mock.Anything, profileID).Return([]string{}, nil).Maybe()
		mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

		f := NewDomainFilter(nil, mockCache, nil)
		f.Metrics = rec
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

		err := f.Execute(reqCtx, stageErrDomainDctx("example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	})

	t.Run("ip phase", func(t *testing.T) {
		mockCache := new(mocks.Cache)
		mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

		f := NewIPFilter(nil, mockCache, nil, nil, nil, nil)
		f.Metrics = rec
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

		err := f.Execute(reqCtx, dnsCtxWithAAnswer(t, "93.184.216.34"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	})

	assert.Equal(t, 0, rec.count(), "no stage error must be recorded when nothing failed")
}

// specRef: proxy-filtering-behaviour.md #I6
// specRef: proxy-filtering-behaviour.md #I8
func TestIPFilterExecute_ServicesListError_Unavailable(t *testing.T) {
	const (
		profileID = "stage-error-i6"
		asn       = uint(15169)
	)
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return(nil, errStore).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

	rec := &stageErrorRecorder{}
	f := NewIPFilter(nil, mockCache, staticCatalog{cat: googleCatalogWithASN(asn)}, staticASNLookup{asn: asn}, nil, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(reqCtx, dnsCtxWithAAnswer(t, "8.8.8.8"))

	require.Error(t, err, "a services-list read error is a settings-store error, not a disabled feature")
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status)
	assert.True(t, rec.has(FilterTypeIP, "services"), "recorder pairs: %v", rec.pairs)
}

// specRef: proxy-filtering-behaviour.md #I7
func TestIPFilterExecute_CatalogUnavailable_Inert(t *testing.T) {
	const (
		profileID = "stage-error-i7"
		asn       = uint(15169)
	)
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileServicesBlocked", mock.Anything, profileID).Return([]string{"google"}, nil).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

	rec := &stageErrorRecorder{}
	f := NewIPFilter(nil, mockCache, staticCatalogErr{err: errors.New("catalog load")}, staticASNLookup{asn: asn}, nil, nil)
	f.Metrics = rec

	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})
	err := f.Execute(reqCtx, dnsCtxWithAAnswer(t, "8.8.8.8"))

	require.NoError(t, err, "a local catalog load failure is not a store error")
	assert.Equal(t, model.StatusProcessed, reqCtx.FilterResult.Status)
	assert.Equal(t, 0, rec.count())
}

// Execute must tolerate a nil metrics recorder (tests and tools construct
// filters without one).
// specRef: proxy-filtering-behaviour.md #I1
func TestDomainFilterExecute_StageError_NilRecorder(t *testing.T) {
	const profileID = "stage-error-nil-recorder"
	mockCache := new(mocks.Cache)
	mockCache.On("GetProfileBlocklists", mock.Anything, profileID).Return(nil, errStore).Maybe()
	mockCache.On("GetCustomRulesHashes", mock.Anything, profileID).Return([]string{}, nil).Maybe()

	f := NewDomainFilter(nil, mockCache, nil)
	reqCtx := stageErrReqCtx(t, profileID, map[string]string{})

	require.NotPanics(t, func() { _ = f.Execute(reqCtx, stageErrDomainDctx("example.com")) })
	assert.Equal(t, model.StatusUnavailable, reqCtx.FilterResult.Status)
}

// applyDefaultRule reads the privacy settings already carried by the request
// context; a strict mock with no GetProfilePrivacySettings expectation fails
// the test if the stage reaches for Redis.
// specRef: proxy-filtering-behaviour.md #I5
func TestApplyDefaultRule_ReadsRequestContext_NoCacheCall(t *testing.T) {
	const profileID = "default-rule-no-redis"
	mockCache := mocks.NewCache(t)
	mockCache.EXPECT().GetProfileBlocklists(mock.Anything, profileID).Return([]string{}, nil).Maybe()
	mockCache.EXPECT().GetCustomRulesHashes(mock.Anything, profileID).Return([]string{}, nil).Maybe()

	f := NewDomainFilter(nil, mockCache, nil)

	t.Run("stage direct", func(t *testing.T) {
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{DEFAULT_RULE: RULE_BLOCK})
		res, err := f.applyDefaultRule(reqCtx, stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.DecisionBlock, res.Decision)
		assert.Equal(t, TierDefaultRule, res.Tier)
		assert.Contains(t, res.Reasons, DEFAULT_RULE)
	})

	t.Run("stage direct allow", func(t *testing.T) {
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{DEFAULT_RULE: RULE_ALLOW})
		res, err := f.applyDefaultRule(reqCtx, stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.DecisionNone, res.Decision)
	})

	t.Run("through Execute", func(t *testing.T) {
		reqCtx := stageErrReqCtx(t, profileID, map[string]string{DEFAULT_RULE: RULE_BLOCK})
		err := f.Execute(reqCtx, stageErrDomainDctx("anything.example.com"))
		require.NoError(t, err)
		assert.Equal(t, model.StatusBlocked, reqCtx.FilterResult.Status)
		assert.Contains(t, reqCtx.FilterResult.Reasons, DEFAULT_RULE)
	})
}
