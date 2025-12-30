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
	prof "github.com/ivpn/dns/api/service/profile"
	"github.com/ivpn/dns/libs/urlshort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	qlAccID   = "507f1f77bcf86cd799439031"
	qlSessTok = "session-token-ql"
	qlProfile = "profile-ql"
)

type QueryLogsAPIShortSuite struct {
	suite.Suite
	svc *mocks.Servicer
	db  *mocks.Db
	v   *validator.APIValidator
	cfg *config.Config
}

func (s *QueryLogsAPIShortSuite) SetupSuite() {
	var err error
	s.v, err = validator.NewAPIValidator()
	s.Require().NoError(err)
	s.cfg = &config.Config{API: &config.APIConfig{ApiAllowOrigin: "http://localhost:3000", ApiAllowIP: "*"}, Server: &config.ServerConfig{Name: "modDNS Test", FQDN: "test.local"}}
}

func (s *QueryLogsAPIShortSuite) SetupTest() {
	s.svc = mocks.NewServicer(s.T())
	s.db = mocks.NewDb(s.T())
}

func (s *QueryLogsAPIShortSuite) server() *APIServer {
	testService := service.Service{Store: s.db, ProfileServicer: s.svc, SessionServicer: s.svc}
	cache := mocks.NewCachecache(s.T())
	gen := mocks.NewGeneratoridgen(s.T())
	mail := mocks.NewMaileremail(s.T())
	short := urlshort.NewURLShortener()
	srv, err := NewServer(s.cfg, testService, s.db, cache, gen, s.v, mail, short)
	s.Require().NoError(err)
	srv.RegisterRoutes()
	return srv
}

func (s *QueryLogsAPIShortSuite) auth(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: qlSessTok})
	s.db.On("GetSession", mock.Anything, qlSessTok).Return(model.Session{AccountID: qlAccID}, true, nil)
}

func (s *QueryLogsAPIShortSuite) TestGetLogsSuccess() {
	logs := []model.QueryLog{{ProfileID: qlProfile, Status: "processed", Timestamp: time.Now(), DNSRequest: model.DNSRequest{Domain: "example.com"}}}
	s.svc.On("GetProfileQueryLogs", mock.Anything, qlAccID, qlProfile, "processed", "LAST_1_HOUR", "", "", "created", 1, 25).Return(logs, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=1&limit=25&status=processed&timespan=LAST_1_HOUR", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	var out []model.QueryLog
	require.NoError(s.T(), json.NewDecoder(resp.Body).Decode(&out))
	assert.Len(s.T(), out, 1)
}

func (s *QueryLogsAPIShortSuite) TestGetLogsValidationError() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=0&limit=30&status=processed&timespan=LAST_1_HOUR", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
	s.svc.AssertNotCalled(s.T(), "GetProfileQueryLogs")
}

func (s *QueryLogsAPIShortSuite) TestGetLogsServiceError() {
	s.svc.On("GetProfileQueryLogs", mock.Anything, qlAccID, qlProfile, "all", "LAST_1_HOUR", "", "", "created", 1, 25).Return(nil, errors.New("boom"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=1&limit=25&status=all&timespan=LAST_1_HOUR", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
}

func (s *QueryLogsAPIShortSuite) TestGetLogsRateLimited() {
	s.svc.On("GetProfileQueryLogs", mock.Anything, qlAccID, qlProfile, "all", "LAST_1_HOUR", "", "", "created", 1, 25).Return(nil, prof.ErrQueryLogsRateLimited)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=1&limit=25&status=all&timespan=LAST_1_HOUR", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusTooManyRequests, resp.StatusCode)
}

func (s *QueryLogsAPIShortSuite) TestGetLogsSearchDevice() {
	s.svc.On("GetProfileQueryLogs", mock.Anything, qlAccID, qlProfile, "all", "LAST_1_HOUR", "dev123", "exam", "domain", 2, 10).Return([]model.QueryLog{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=2&limit=10&status=all&timespan=LAST_1_HOUR&device_id=dev123&search=exam&sort_by=domain", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	s.svc.AssertCalled(s.T(), "GetProfileQueryLogs", mock.Anything, qlAccID, qlProfile, "all", "LAST_1_HOUR", "dev123", "exam", "domain", 2, 10)
}

func (s *QueryLogsAPIShortSuite) TestGetLogsInvalidSort() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs?page=1&limit=25&sort_by=random", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
	s.svc.AssertNotCalled(s.T(), "GetProfileQueryLogs")
}

func (s *QueryLogsAPIShortSuite) TestDownloadLogsSuccess() {
	ts := time.Now()
	s.svc.On("DownloadProfileQueryLogs", mock.Anything, qlAccID, qlProfile, 0, 0).Return([]model.QueryLog{{ProfileID: qlProfile, Status: "processed", Timestamp: ts, DNSRequest: model.DNSRequest{Domain: "ex.com"}}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs/download", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Contains(s.T(), resp.Header.Get("Content-Disposition"), "attachment")
}

func (s *QueryLogsAPIShortSuite) TestDownloadLogsError() {
	s.svc.On("DownloadProfileQueryLogs", mock.Anything, qlAccID, qlProfile, 0, 0).Return(nil, errors.New("failure"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+qlProfile+"/logs/download", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
}

func (s *QueryLogsAPIShortSuite) TestDeleteLogsSuccess() {
	s.svc.On("DeleteProfileQueryLogs", mock.Anything, qlAccID, qlProfile).Return(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/profiles/"+qlProfile+"/logs", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusNoContent, resp.StatusCode)
}

func (s *QueryLogsAPIShortSuite) TestDeleteLogsError() {
	s.svc.On("DeleteProfileQueryLogs", mock.Anything, qlAccID, qlProfile).Return(errors.New("delete failed"))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/profiles/"+qlProfile+"/logs", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
}

func TestQueryLogsAPIShortSuite(t *testing.T) { suite.Run(t, new(QueryLogsAPIShortSuite)) }
