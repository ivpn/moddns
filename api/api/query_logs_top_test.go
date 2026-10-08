package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (s *QueryLogsAPIShortSuite) get(path string) *http.Response {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	s.auth(req)
	resp, err := s.server().App.Test(req, -1)
	require.NoError(s.T(), err)
	return resp
}

// tableRef: api-endpoint-behaviour #J20
func (s *QueryLogsAPIShortSuite) TestTopDomainsDefaultsAndBody() {
	want := &model.QueryLogTopDomains{Enabled: true, Items: []model.QueryLogTopDomain{{Domain: "example.com", Count: 12}}}
	s.svc.On("GetProfileQueryLogTop", mock.Anything, qlAccID, qlProfile, model.LAST_1_DAY, "blocked", 10).Return(want, nil)

	resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/top?kind=blocked")
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	var got model.QueryLogTopDomains
	require.NoError(s.T(), json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(s.T(), *want, got)
}

// tableRef: api-endpoint-behaviour #J20
func (s *QueryLogsAPIShortSuite) TestTopDomainsRejectsBadParams() {
	for _, q := range []string{
		"",                             // kind is required
		"?kind=other",                  // unknown kind
		"?kind=blocked&limit=0",        // below range
		"?kind=blocked&limit=51",       // above range
		"?kind=blocked&timespan=LAST_", // unknown timespan
	} {
		resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/top" + q)
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, q)
	}
}

// tableRef: api-endpoint-behaviour #J20
func (s *QueryLogsAPIShortSuite) TestTopDomainsForeignProfileIsNotFound() {
	s.svc.On("GetProfileQueryLogTop", mock.Anything, qlAccID, qlProfile, model.LAST_1_DAY, "resolved", 50).Return(nil, dbErrors.ErrProfileNotFound)
	resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/top?kind=resolved&limit=50")
	assert.Equal(s.T(), http.StatusNotFound, resp.StatusCode)
}

// tableRef: api-endpoint-behaviour #J21
func (s *QueryLogsAPIShortSuite) TestClientsDefaultsAndNullEnrichment() {
	asn := uint32(15169)
	want := &model.QueryLogTopClients{Enabled: true, Items: []model.QueryLogTopClient{
		{IP: "8.8.8.8", Count: 40, ASN: &asn},
		{IP: "203.0.113.7", Count: 3},
	}}
	s.svc.On("GetProfileQueryLogClients", mock.Anything, qlAccID, qlProfile, "LAST_7_DAYS", 10).Return(want, nil)

	resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/clients?timespan=LAST_7_DAYS")
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	var raw struct {
		Enabled bool             `json:"enabled"`
		Items   []map[string]any `json:"items"`
	}
	require.NoError(s.T(), json.NewDecoder(resp.Body).Decode(&raw))
	require.Len(s.T(), raw.Items, 2)
	assert.EqualValues(s.T(), 15169, raw.Items[0]["asn"])
	// Unknown enrichment is an explicit null, not an absent key.
	for _, k := range []string{"asn", "as_org", "country"} {
		v, ok := raw.Items[1][k]
		assert.True(s.T(), ok, k)
		assert.Nil(s.T(), v, k)
	}
}

// tableRef: api-endpoint-behaviour #J21
func (s *QueryLogsAPIShortSuite) TestClientsRejectsBadParams() {
	for _, q := range []string{"?limit=0", "?limit=51", "?timespan=nope"} {
		resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/clients" + q)
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, q)
	}
}

// Present-but-invalid limit is a 400, never a silent default.
// tableRef: api-endpoint-behaviour #J20
// tableRef: api-endpoint-behaviour #J21
// tableRef: api-endpoint-behaviour #J24
func (s *QueryLogsAPIShortSuite) TestTopAndClientsRejectNonNumericAndOutOfRangeLimit() {
	for _, ep := range []string{"/logs/top?kind=blocked&", "/logs/clients?", "/logs/blocklists?"} {
		for _, l := range []string{"x", "", "1.5", "0", "-1", "51", "99999999999999999999"} {
			resp := s.get("/api/v1/profiles/" + qlProfile + ep + "limit=" + l)
			assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, ep+"limit="+l)
		}
	}
}

// tableRef: api-endpoint-behaviour #J20
// tableRef: api-endpoint-behaviour #J21
// tableRef: api-endpoint-behaviour #J24
func (s *QueryLogsAPIShortSuite) TestTopAndClientsAcceptShortTimespans() {
	for _, ts := range []string{"LAST_3_HOURS", "LAST_6_HOURS"} {
		s.svc.On("GetProfileQueryLogTop", mock.Anything, qlAccID, qlProfile, ts, "blocked", 10).Return(&model.QueryLogTopDomains{Enabled: true}, nil).Once()
		resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/top?kind=blocked&timespan=" + ts)
		assert.Equal(s.T(), http.StatusOK, resp.StatusCode, ts)

		s.svc.On("GetProfileQueryLogClients", mock.Anything, qlAccID, qlProfile, ts, 10).Return(&model.QueryLogTopClients{Enabled: true}, nil).Once()
		resp = s.get("/api/v1/profiles/" + qlProfile + "/logs/clients?timespan=" + ts)
		assert.Equal(s.T(), http.StatusOK, resp.StatusCode, ts)

		s.svc.On("GetProfileQueryLogBlocklists", mock.Anything, qlAccID, qlProfile, ts, 10).Return(&model.QueryLogTopBlocklists{Enabled: true}, nil).Once()
		resp = s.get("/api/v1/profiles/" + qlProfile + "/logs/blocklists?timespan=" + ts)
		assert.Equal(s.T(), http.StatusOK, resp.StatusCode, ts)
	}
}

// tableRef: api-endpoint-behaviour #J1
// tableRef: api-endpoint-behaviour #J20
func (s *QueryLogsAPIShortSuite) TestLogsListRejectsShortTimespans() {
	for _, ts := range []string{"LAST_3_HOURS", "LAST_6_HOURS"} {
		resp := s.get("/api/v1/profiles/" + qlProfile + "/logs?page=1&limit=25&status=all&timespan=" + ts)
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, ts)
	}
}

// tableRef: api-endpoint-behaviour #J24
func (s *QueryLogsAPIShortSuite) TestBlocklistsDefaultsAndBody() {
	want := &model.QueryLogTopBlocklists{Enabled: true, Items: []model.QueryLogTopBlocklist{{BlocklistID: "oisd", Count: 7}}}
	s.svc.On("GetProfileQueryLogBlocklists", mock.Anything, qlAccID, qlProfile, "LAST_1_DAY", 10).Return(want, nil)

	resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/blocklists")
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	var raw struct {
		Enabled bool             `json:"enabled"`
		Items   []map[string]any `json:"items"`
	}
	require.NoError(s.T(), json.NewDecoder(resp.Body).Decode(&raw))
	assert.True(s.T(), raw.Enabled)
	require.Len(s.T(), raw.Items, 1)
	assert.Equal(s.T(), "oisd", raw.Items[0]["blocklist_id"])
	assert.EqualValues(s.T(), 7, raw.Items[0]["count"])
}

// tableRef: api-endpoint-behaviour #J24
func (s *QueryLogsAPIShortSuite) TestBlocklistsRejectsBadParams() {
	for _, q := range []string{"?limit=0", "?limit=51", "?timespan=nope", "?timespan=LAST_YEAR"} {
		resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/blocklists" + q)
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode, q)
	}
}

// tableRef: api-endpoint-behaviour #J24
func (s *QueryLogsAPIShortSuite) TestBlocklistsForeignProfileIsNotFound() {
	s.svc.On("GetProfileQueryLogBlocklists", mock.Anything, qlAccID, qlProfile, "LAST_1_DAY", 50).Return(nil, dbErrors.ErrProfileNotFound)
	resp := s.get("/api/v1/profiles/" + qlProfile + "/logs/blocklists?limit=50")
	assert.Equal(s.T(), http.StatusNotFound, resp.StatusCode)
}
