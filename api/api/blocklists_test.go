package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	blAccID   = "507f1f77bcf86cd799439041"
	blSessTok = "session-token-blocklists"
)

type BlocklistsAPISuite struct {
	suite.Suite
	svc *mocks.Servicer
	db  *mocks.Db
	v   *validator.APIValidator
	cfg *config.Config
}

func (s *BlocklistsAPISuite) SetupSuite() {
	var err error
	s.v, err = validator.NewAPIValidator()
	s.Require().NoError(err)
	s.cfg = &config.Config{API: &config.APIConfig{ApiAllowOrigin: "http://localhost:3000", ApiAllowIP: "*"}, Server: &config.ServerConfig{Name: "modDNS Test", FQDN: "test.local"}}
}

func (s *BlocklistsAPISuite) SetupTest() {
	s.svc = mocks.NewServicer(s.T())
	s.db = mocks.NewDb(s.T())
}

func (s *BlocklistsAPISuite) server() *APIServer {
	testService := service.Service{Store: s.db, BlocklistServicer: s.svc, SessionServicer: s.svc}
	cache := mocks.NewCachecache(s.T())
	gen := mocks.NewGeneratoridgen(s.T())
	mail := mocks.NewMaileremail(s.T())
	short := urlshort.NewURLShortener()
	srv, err := NewServer(s.cfg, testService, s.db, cache, gen, s.v, mail, short)
	s.Require().NoError(err)
	srv.RegisterRoutes()
	return srv
}

func (s *BlocklistsAPISuite) auth(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: auth.AUTH_COOKIE, Value: blSessTok})
	s.db.On("GetSession", mock.Anything, blSessTok).Return(model.Session{AccountID: blAccID}, true, nil)
}

func (s *BlocklistsAPISuite) TestGetBlocklistsDefaultSort() {
	matcher := mock.MatchedBy(func(filter map[string]any) bool { return len(filter) == 0 })
	s.svc.On("GetBlocklist", mock.Anything, matcher, "updated").Return([]*model.Blocklist{{BlocklistID: "ads"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocklists", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	s.svc.AssertCalled(s.T(), "GetBlocklist", mock.Anything, matcher, "updated")
}

func (s *BlocklistsAPISuite) TestGetBlocklistsCustomSortAndFilter() {
	matcher := mock.MatchedBy(func(filter map[string]any) bool {
		val, ok := filter["default"].(bool)
		return ok && val
	})
	s.svc.On("GetBlocklist", mock.Anything, matcher, "entries").Return([]*model.Blocklist{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocklists?default=true&sort_by=entries", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	s.svc.AssertCalled(s.T(), "GetBlocklist", mock.Anything, matcher, "entries")
}

func (s *BlocklistsAPISuite) TestGetBlocklistsInvalidSort() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocklists?sort_by=invalid", nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
	s.svc.AssertNotCalled(s.T(), "GetBlocklist", mock.Anything, mock.Anything, mock.Anything)
}

func TestBlocklistsAPISuite(t *testing.T) { suite.Run(t, new(BlocklistsAPISuite)) }
