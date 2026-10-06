package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service"
	"github.com/ivpn/dns/libs/urlshort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	statAccID   = "507f1f77bcf86cd799439041"
	statSessTok = "session-token-stat"
	statProfile = "profile-stat"
)

type StatisticsAPISuite struct {
	suite.Suite
	svc *mocks.Servicer
	db  *mocks.Db
	v   *validator.APIValidator
	cfg *config.Config
}

func (s *StatisticsAPISuite) SetupSuite() {
	var err error
	s.v, err = validator.NewAPIValidator()
	s.Require().NoError(err)
	s.cfg = &config.Config{API: &config.APIConfig{ApiAllowOrigin: "http://localhost:3000", ApiAllowIP: "*"}, Server: &config.ServerConfig{Name: "modDNS Test", FQDN: "test.local"}}
}

func (s *StatisticsAPISuite) SetupTest() {
	s.svc = mocks.NewServicer(s.T())
	s.db = mocks.NewDb(s.T())
}

func (s *StatisticsAPISuite) get(query string) *http.Response {
	testService := service.Service{Store: s.db, ProfileServicer: s.svc, SessionServicer: s.svc}
	srv, err := NewServer(s.cfg, testService, s.db, mocks.NewCachecache(s.T()), mocks.NewGeneratoridgen(s.T()), s.v, mocks.NewMaileremail(s.T()), urlshort.NewURLShortener(), nil, nil)
	s.Require().NoError(err)
	srv.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+statProfile+"/statistics"+query, nil)
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: statSessTok})
	s.db.On("GetSession", mock.Anything, statSessTok).Return(model.Session{AccountID: statAccID}, true, nil)
	resp, err := srv.App.Test(req, -1)
	s.Require().NoError(err)
	return resp
}

// specRef: api-endpoint-behaviour.md J40 — an absent timespan means LAST_7_DAYS.
func (s *StatisticsAPISuite) TestDefaultTimespan() {
	now := time.Now().UTC().Truncate(time.Second)
	s.svc.On("GetStatistics", mock.Anything, statAccID, statProfile, "LAST_7_DAYS").
		Return(&model.StatisticsResponse{Enabled: true, Timespan: "LAST_7_DAYS", From: now, To: now, Series: []model.StatisticsPoint{}, Devices: []model.StatisticsDevice{}}, nil)
	resp := s.get("")
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	var out map[string]any
	require.NoError(s.T(), json.NewDecoder(resp.Body).Decode(&out))
	for _, k := range []string{"enabled", "enabled_at", "retention", "timespan", "from", "to", "bucket_seconds", "totals", "series", "reasons", "protocols", "devices"} {
		assert.Contains(s.T(), out, k)
	}
}

// specRef: api-endpoint-behaviour.md J40 — the four statistics timespans pass through to the service.
func (s *StatisticsAPISuite) TestAcceptedTimespans() {
	for _, ts := range model.StatisticsTimespans() {
		s.SetupTest()
		s.svc.On("GetStatistics", mock.Anything, statAccID, statProfile, ts).Return(&model.StatisticsResponse{Timespan: ts}, nil)
		resp := s.get("?timespan=" + ts)
		assert.Equal(s.T(), http.StatusOK, resp.StatusCode, ts)
	}
}

// specRef: api-endpoint-behaviour.md J40 — everything else, including the logs-only values, is 400 before any service call.
func (s *StatisticsAPISuite) TestRejectedTimespans() {
	for _, ts := range []string{"LAST_1_HOUR", "LAST_12_HOURS", "LAST_3_YEARS", "bogus", "last_7_days"} {
		s.SetupTest()
		resp := s.get("?timespan=" + ts)
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, ts)
		s.svc.AssertNotCalled(s.T(), "GetStatistics")
	}
}

// specRef: api-endpoint-behaviour.md J4 — a service failure is a 500.
func (s *StatisticsAPISuite) TestServiceError() {
	s.svc.On("GetStatistics", mock.Anything, statAccID, statProfile, "LAST_1_DAY").Return(nil, errors.New("boom"))
	resp := s.get("?timespan=LAST_1_DAY")
	assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
}

func TestStatisticsAPISuite(t *testing.T) { suite.Run(t, new(StatisticsAPISuite)) }
