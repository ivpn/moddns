package server

// Tests for the admission-time split between "profile does not exist" (drop,
// Q6) and "settings store unreachable" (SERVFAIL, Q12).
// Rows: docs/specs/proxy-request-admission-behaviour.md.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/cache"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// recordingMetrics implements Metrics and records stage-error pairs.
type recordingMetrics struct {
	mu          sync.Mutex
	stageErrors [][2]string
}

func (m *recordingMetrics) RecordQuery(string)                               {}
func (m *recordingMetrics) RecordProfileCacheLookup(bool)                    {}
func (m *recordingMetrics) RecordQueryDuration(string, time.Duration)        {}
func (m *recordingMetrics) RecordDomainFilterDuration(string, time.Duration) {}
func (m *recordingMetrics) RecordIPFilterDuration(string, time.Duration)     {}
func (m *recordingMetrics) RecordUpstreamDuration(string, time.Duration)     {}
func (m *recordingMetrics) RecordBlocked(string)                             {}
func (m *recordingMetrics) RecordFilterStageError(phase, stage string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stageErrors = append(m.stageErrors, [2]string{phase, stage})
}

func (m *recordingMetrics) pairs() [][2]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][2]string(nil), m.stageErrors...)
}

func newSettingsServer(c *mocks.Cache) (*Server, *recordingMetrics) {
	s := newProfileRateLimitServer(c, "refuse")
	m := &recordingMetrics{}
	s.Metrics = m
	return s, m
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestPrepareRequest_SettingsBatchError_Servfail(t *testing.T) {
	const profileID = "storeerrprofile1"
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(nil, errors.New("redis pipeline failed: dial tcp 10.0.0.6:6379: i/o timeout"))
	s, m := newSettingsServer(c)

	reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))

	require.NoError(t, err, "an infrastructure error must not be a drop")
	require.Nil(t, reqCtx)
	require.NotNil(t, errResp, "the client must receive an answer")
	assert.Equal(t, dns.RcodeServerFailure, errResp.Rcode)
	assert.True(t, errResp.Response, "QR bit set")
	assert.Equal(t, [][2]string{{"admission", "profile_settings"}}, m.pairs())
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestPrepareRequest_PrivacyInfraError_Servfail(t *testing.T) {
	const profileID = "storeerrprofile2"
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(&model.ProfileSettings{PrivacyErr: errors.New("i/o timeout")}, nil)
	s, m := newSettingsServer(c)

	reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))

	require.NoError(t, err, "a per-key infrastructure error must not be conflated with not-found")
	require.Nil(t, reqCtx)
	require.NotNil(t, errResp)
	assert.Equal(t, dns.RcodeServerFailure, errResp.Rcode)
	assert.Equal(t, [][2]string{{"admission", "profile_settings"}}, m.pairs())
}

// specRef: proxy-request-admission-behaviour.md #Q6
func TestPrepareRequest_PrivacyNotFound_Drops(t *testing.T) {
	const profileID = "unknownprofile2"
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(&model.ProfileSettings{PrivacyErr: fmt.Errorf("%w: [privacy]", cache.ErrSettingsNotFound)}, nil)
	s, m := newSettingsServer(c)

	reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))

	require.ErrorIs(t, err, errProfileIdNotFound)
	require.Nil(t, reqCtx)
	require.Nil(t, errResp, "a nonexistent profile is dropped without a response")
	assert.Empty(t, m.pairs(), "not-found is not a store error")
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestPrepareRequest_SettingsBatchError_NotCached(t *testing.T) {
	const profileID = "storeerrprofile3"
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(nil, errors.New("redis pipeline failed")).Twice()
	s, _ := newSettingsServer(c)

	for i := range 2 {
		_, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))
		require.NoError(t, err, "call %d", i)
		require.NotNil(t, errResp, "call %d", i)
		assert.Equal(t, dns.RcodeServerFailure, errResp.Rcode, "call %d", i)
	}
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestServeDNS_SettingsUnavailableAnswersServfail(t *testing.T) {
	const profileID = "storeerrprofile4"
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).
		Return(nil, errors.New("redis pipeline failed: connection refused"))
	s, _ := newSettingsServer(c)

	dctx := newDoHDNSContext(profileID)
	err := s.ServeDNS(context.Background(), nil, dctx)

	require.NoError(t, err, "SERVFAIL is a response, not a drop")
	require.NotNil(t, dctx.Res)
	assert.Equal(t, dns.RcodeServerFailure, dctx.Res.Rcode)
	assert.Equal(t, dctx.Req.Id, dctx.Res.Id)
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestServFailResponse_Shape(t *testing.T) {
	s := &Server{}
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Id = 0xBEEF

	resp := s.servFailResponse(req)

	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)
	assert.True(t, resp.Response)
	assert.Equal(t, uint16(0xBEEF), resp.Id)
	assert.Empty(t, resp.Answer)
}

// specRef: proxy-request-admission-behaviour.md #Q12
func TestPrepareRequest_FilterInputReadError_Servfail(t *testing.T) {
	tests := []struct {
		name     string
		settings *model.ProfileSettings
	}{
		{
			name: "blocklists list unreadable",
			settings: &model.ProfileSettings{
				Privacy:       map[string]string{"default_rule": "allow"},
				BlocklistsErr: errors.New("i/o timeout"),
			},
		},
		{
			name: "custom rules unreadable",
			settings: &model.ProfileSettings{
				Privacy:        map[string]string{"default_rule": "allow"},
				CustomRulesErr: errors.New("i/o timeout"),
			},
		},
		{
			name: "services list unreadable",
			settings: &model.ProfileSettings{
				Privacy:     map[string]string{"default_rule": "allow"},
				ServicesErr: errors.New("connection reset by peer"),
			},
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profileID := fmt.Sprintf("inputerrprofile%d", i)
			c := mocks.NewCache(t)
			c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(tt.settings, nil)
			s, m := newSettingsServer(c)

			reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))

			require.NoError(t, err, "incomplete filter inputs are a store failure, not a drop")
			require.Nil(t, reqCtx)
			require.NotNil(t, errResp)
			assert.Equal(t, dns.RcodeServerFailure, errResp.Rcode)
			assert.Equal(t, [][2]string{{"admission", "profile_settings"}}, m.pairs())
		})
	}
}

// specRef: proxy-request-admission-behaviour.md #Q12
// Absent optional groups are not store failures: defaults apply and the
// request proceeds with the batch's filter inputs on the request context.
func TestPrepareRequest_AbsentOptionalGroups_Proceeds(t *testing.T) {
	const profileID = "absentgroupsprofile1"
	absent := func(name string) error { return fmt.Errorf("%w: [%s]", cache.ErrSettingsNotFound, name) }
	rules := []map[string]string{{"value": "ads.example", "action": "block", "syntax": "domain"}}
	settings := &model.ProfileSettings{
		Privacy:                map[string]string{"default_rule": "allow"},
		LogsErr:                absent("logs"),
		DNSSECErr:              absent("security dnssec"),
		AdvancedErr:            absent("advanced"),
		RebindingProtectionErr: absent("security rebinding_protection"),
		StatisticsErr:          absent("statistics"),
		Blocklists:             []string{"bl1", "bl2"},
		Services:               []string{"google"},
		CustomRules:            rules,
	}
	c := mocks.NewCache(t)
	c.EXPECT().GetProfileSettingsBatch(mock.Anything, profileID).Return(settings, nil)
	s, m := newSettingsServer(c)
	s.Upstreams = map[string]*proxy.CustomUpstreamConfig{"default": {}}

	reqCtx, errResp, err := s.prepareRequest(context.Background(), nil, newDoHDNSContext(profileID))

	require.NoError(t, err)
	require.Nil(t, errResp)
	require.NotNil(t, reqCtx)
	assert.Empty(t, m.pairs(), "absent groups are not store errors")
	assert.Equal(t, []string{"bl1", "bl2"}, reqCtx.Blocklists)
	assert.Equal(t, []string{"google"}, reqCtx.BlockedServices)
	assert.Equal(t, rules, reqCtx.CustomRules)
	assert.Nil(t, reqCtx.StatisticsSettings)
	assert.Equal(t, "default", reqCtx.UpstreamName)
}
