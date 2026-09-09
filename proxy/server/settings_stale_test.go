package server

// Serve-stale behaviour of the profile settings cache (Q13, Q14) and the
// store breaker that gates fetches while the store is failing (Q12, Q13).
// Rows: docs/specs/proxy-request-admission-behaviour.md.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ivpn/dns/libs/logging"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/internal/ratelimit"
	"github.com/ivpn/dns/proxy/internal/settingscache"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const staleTTL = 30 * time.Second

// staleFixture is a Server whose settings cache runs on a controllable clock
// and whose rate limits are disabled, so the same profile can be queried
// repeatedly.
type staleFixture struct {
	s       *Server
	cache   *mocks.Cache
	metrics *recordingMetrics
	now     time.Time
}

func newStaleFixture(t *testing.T) *staleFixture {
	t.Helper()
	f := &staleFixture{cache: mocks.NewCache(t), metrics: &recordingMetrics{}, now: time.Unix(1_700_000_000, 0)}
	sc, err := settingscache.New(staleTTL, 16, settingscache.WithClock(func() time.Time { return f.now }))
	require.NoError(t, err)
	f.s = &Server{
		Config: &config.Config{
			Server:    &config.ServerConfig{},
			Upstream:  &config.UpstreamConfig{Default: "default"},
			RateLimit: &config.RateLimitConfig{},
		},
		Cache:                f.cache,
		ProfileSettingsCache: sc,
		LoggerFactory:        logging.NewDefaultFactory(),
		RateLimiter:          ratelimit.New(ratelimit.Config{}, nil),
		Metrics:              f.metrics,
	}
	return f
}

func (f *staleFixture) advance(d time.Duration) { f.now = f.now.Add(d) }

func goodSettings(rule string) *model.ProfileSettings {
	absent := fmt.Errorf("%w: [absent]", cache.ErrSettingsNotFound)
	return &model.ProfileSettings{
		Privacy:                map[string]string{"default_rule": rule},
		Blocklists:             []string{"bl1"},
		LogsErr:                absent,
		DNSSECErr:              absent,
		RebindingProtectionErr: absent,
		AdvancedErr:            absent,
		StatisticsErr:          absent,
	}
}

var errStoreDown = errors.New("redis pipeline failed: dial tcp: connection refused")

// specRef: proxy-request-admission-behaviour.md #Q13
func TestPrepareRequest_StaleServedWhenStoreFails(t *testing.T) {
	const profileID = "staleprofile001"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(goodSettings("block"), nil).Once()
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(nil, errStoreDown).Once()

	// Cold: fetched and cached.
	reqCtx, errResp, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp)
	require.Equal(t, "block", reqCtx.PrivacySettings["default_rule"])

	// Past the TTL with the store down: last-known-good settings are used.
	f.advance(staleTTL + time.Second)
	reqCtx, errResp, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp, "stale settings must be served, not SERVFAIL")
	require.NotNil(t, reqCtx)
	require.Equal(t, "block", reqCtx.PrivacySettings["default_rule"])
	require.Equal(t, []string{"bl1"}, reqCtx.Blocklists)
	require.Contains(t, f.metrics.lookups(), "stale")
	require.Empty(t, f.metrics.pairs(), "serving stale is not an admission error")

	// Breaker open: no further fetch within the probe interval (the mock allows
	// exactly two calls), still served stale.
	_, errResp, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp)
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestPrepareRequest_BreakerOpenNoStaleEntry_Servfail(t *testing.T) {
	const known, unknown = "knownprofile001", "coldprofile00001"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, known).Return(nil, errStoreDown).Once()

	_, errResp, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(known))
	require.NoError(t, err)
	require.NotNil(t, errResp)
	require.Equal(t, dns.RcodeServerFailure, errResp.Rcode)

	// A second profile inside the probe interval is refused without a fetch.
	_, errResp, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(unknown))
	require.NoError(t, err)
	require.NotNil(t, errResp)
	require.Equal(t, dns.RcodeServerFailure, errResp.Rcode)
	require.Equal(t, [][2]string{{"admission", "profile_settings"}, {"admission", "profile_settings"}}, f.metrics.pairs())
	require.Contains(t, f.metrics.lookups(), "unavailable")
}

// specRef: proxy-request-admission-behaviour.md #Q13
func TestPrepareRequest_BreakerProbesOncePerInterval(t *testing.T) {
	const profileID = "probeprofile0001"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(nil, errStoreDown).Once()
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(goodSettings("allow"), nil).Once()

	_, errResp, _ := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.Equal(t, dns.RcodeServerFailure, errResp.Rcode)

	// Within the interval: no fetch, still SERVFAIL.
	f.advance(settingscache.DefaultProbeInterval / 2)
	_, errResp, _ = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.Equal(t, dns.RcodeServerFailure, errResp.Rcode)

	// After the interval one probe goes through and succeeds.
	f.advance(settingscache.DefaultProbeInterval)
	reqCtx, errResp, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp)
	require.Equal(t, "allow", reqCtx.PrivacySettings["default_rule"])

	// Recovered: fresh hits need no fetch at all.
	_, errResp, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp)
}

// specRef: proxy-request-admission-behaviour.md #Q14
func TestPrepareRequest_DeletedProfileEvictsStaleEntry(t *testing.T) {
	const profileID = "deletedprofile01"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(goodSettings("allow"), nil).Once()
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(&model.ProfileSettings{PrivacyErr: fmt.Errorf("%w: [privacy]", cache.ErrSettingsNotFound)}, nil).Twice()

	_, errResp, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.NoError(t, err)
	require.Nil(t, errResp)

	// Profile deleted meanwhile; the next refresh reports not-found → Q6.
	f.advance(staleTTL + time.Second)
	_, errResp, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.ErrorIs(t, err, errProfileIdNotFound)
	require.Nil(t, errResp)

	// The stale entry is gone: the next query is a plain miss that fetches again.
	_, state := f.s.ProfileSettingsCache.Get(profileID)
	require.Equal(t, settingscache.Miss, state)
	_, _, err = f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
	require.ErrorIs(t, err, errProfileIdNotFound)
	require.Empty(t, f.metrics.pairs())
}

// specRef: proxy-request-admission-behaviour.md #Q13
func TestPrepareRequest_FreshEntryNeverFetches(t *testing.T) {
	const profileID = "freshprofile0001"
	f := newStaleFixture(t)
	f.cache.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(goodSettings("allow"), nil).Once()

	for range 3 {
		_, errResp, err := f.s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
		require.NoError(t, err)
		require.Nil(t, errResp)
		f.advance(staleTTL / 4)
	}
	require.Equal(t, []string{"miss", "hit", "hit"}, f.metrics.lookups())
}
