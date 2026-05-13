package subscription_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/api/config"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/subscription"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestUpdateSubscriptionFromPASession covers the conditional signup webhook
// behaviour added when an optional `subid` is accepted on the resync endpoint.
// specRef: SY7
type UpdateSubscriptionFromPASessionSuite struct {
	suite.Suite
	mockSubscriptionRepo *mocks.SubscriptionRepository
	mockProfileRepo      *mocks.ProfileRepository
	mockCache            *mocks.Cachecache
}

func (s *UpdateSubscriptionFromPASessionSuite) SetupTest() {
	s.mockSubscriptionRepo = mocks.NewSubscriptionRepository(s.T())
	s.mockProfileRepo = mocks.NewProfileRepository(s.T())
	s.mockCache = mocks.NewCachecache(s.T())
}

// newPreauthServer returns an httptest server that responds with a Preauth
// matching `token`. Used so ValidateAndGetPreauth succeeds.
func newPreauthServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		preauth := model.Preauth{
			ID:          "preauth-id-1",
			TokenHash:   tokenHashStr,
			IsActive:    true,
			ActiveUntil: time.Now().Add(24 * time.Hour).UTC(),
			Tier:        "IVPN Tier 2",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(preauth)
	}))
}

// newWebhookServer returns an httptest server tracking call count and the
// last "uuid" the caller posted.
type webhookProbe struct {
	server   *httptest.Server
	calls    int32
	lastUUID atomic.Value // string
}

func newWebhookServer(t *testing.T, status int) *webhookProbe {
	t.Helper()
	probe := &webhookProbe{}
	probe.lastUUID.Store("")
	probe.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&probe.calls, 1)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		probe.lastUUID.Store(body["uuid"])
		w.WriteHeader(status)
	}))
	return probe
}

func (s *UpdateSubscriptionFromPASessionSuite) buildService(preauthURL, webhookURL string) *subscription.SubscriptionService {
	apiCfg := config.APIConfig{
		PreauthURL:       preauthURL,
		SignupWebhookURL: webhookURL,
		SignupWebhookPSK: "test-psk",
	}
	httpClient := webhookClient.Http{Cfg: apiCfg}
	return subscription.NewSubscriptionService(
		s.mockSubscriptionRepo,
		s.mockProfileRepo,
		s.mockCache,
		config.ServiceConfig{},
		apiCfg,
		httpClient,
	)
}

func (s *UpdateSubscriptionFromPASessionSuite) primeValidationMocks(sessionID, token string) {
	s.mockCache.On("GetPASession", mock.Anything, sessionID).Return(&model.PASession{
		ID:        sessionID,
		Token:     token,
		PreauthID: "preauth-id-1",
	}, nil)
}

func (s *UpdateSubscriptionFromPASessionSuite) primePersistMocks(sub *model.Subscription) {
	s.mockSubscriptionRepo.On("Upsert", mock.Anything, mock.AnythingOfType("model.Subscription")).Return(nil)
	s.mockSubscriptionRepo.On("ClearLegacyType", mock.Anything, sub.AccountID.Hex()).Return(nil)
	s.mockProfileRepo.On("GetProfilesByAccountId", mock.Anything, sub.AccountID.Hex()).Return([]model.Profile{}, nil)
}

func newSub() *model.Subscription {
	return &model.Subscription{
		ID:        uuid.New(),
		AccountID: primitive.NewObjectID(),
	}
}

// SY7: subID empty -> webhook is NOT called.
func (s *UpdateSubscriptionFromPASessionSuite) TestEmptySubIDSkipsWebhook() {
	preauthSrv := newPreauthServer(s.T(), "tok-1")
	defer preauthSrv.Close()
	probe := newWebhookServer(s.T(), http.StatusOK)
	defer probe.server.Close()

	svc := s.buildService(preauthSrv.URL, probe.server.URL)
	sub := newSub()
	s.primeValidationMocks("sess-1", "tok-1")
	s.primePersistMocks(sub)

	err := svc.UpdateSubscriptionFromPASession(context.Background(), sub, "sess-1", "")

	s.Require().NoError(err)
	s.Equal(int32(0), atomic.LoadInt32(&probe.calls), "webhook must NOT fire when subID is empty")
}

// SY7: subID provided -> webhook called exactly once with that subID.
func (s *UpdateSubscriptionFromPASessionSuite) TestNonEmptySubIDFiresWebhook() {
	preauthSrv := newPreauthServer(s.T(), "tok-2")
	defer preauthSrv.Close()
	probe := newWebhookServer(s.T(), http.StatusOK)
	defer probe.server.Close()

	svc := s.buildService(preauthSrv.URL, probe.server.URL)
	sub := newSub()
	s.primeValidationMocks("sess-2", "tok-2")
	s.primePersistMocks(sub)

	requestSubID := "550e8400-e29b-41d4-a716-446655440000"
	err := svc.UpdateSubscriptionFromPASession(context.Background(), sub, "sess-2", requestSubID)

	s.Require().NoError(err)
	s.Equal(int32(1), atomic.LoadInt32(&probe.calls), "webhook must fire exactly once")
	s.Equal(requestSubID, probe.lastUUID.Load(), "webhook payload must carry the request subid (not the internal sub UUID)")
}

// SY7: webhook returns non-200 with non-empty subID -> error propagates.
func (s *UpdateSubscriptionFromPASessionSuite) TestWebhookFailurePropagates() {
	preauthSrv := newPreauthServer(s.T(), "tok-3")
	defer preauthSrv.Close()
	probe := newWebhookServer(s.T(), http.StatusInternalServerError)
	defer probe.server.Close()

	svc := s.buildService(preauthSrv.URL, probe.server.URL)
	sub := newSub()
	s.primeValidationMocks("sess-3", "tok-3")
	s.primePersistMocks(sub)

	err := svc.UpdateSubscriptionFromPASession(context.Background(), sub, "sess-3", "550e8400-e29b-41d4-a716-446655440000")
	s.Require().Error(err)
	s.Contains(err.Error(), "signup webhook")
}

func TestUpdateSubscriptionFromPASession(t *testing.T) {
	require.NotPanics(t, func() {})
	suite.Run(t, new(UpdateSubscriptionFromPASessionSuite))
}

// TestUpdateSubscriptionFromPASession_ClearsLegacyType verifies the chore/banner-beta
// contract: a successful resync via PASession must clear the pre-0.1.8 `type` field
// so the beta-ending banner stops rendering for the account.
func TestUpdateSubscriptionFromPASession_ClearsLegacyType(t *testing.T) {
	accountOID := primitive.NewObjectID()
	subID := uuid.New()
	sessionID := "pa-session-test"
	preauthID := "preauth-id-test"
	token := "pa-token-test"
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := base64.StdEncoding.EncodeToString(tokenHash[:])
	activeUntil := time.Now().Add(30 * 24 * time.Hour).UTC()

	// httptest preauth service: returns the entry whose token_hash matches our
	// session token so ValidateAndGetPreauth succeeds without divergence.
	preauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		preauth := model.Preauth{
			ID:          preauthID,
			TokenHash:   tokenHashStr,
			IsActive:    true,
			ActiveUntil: activeUntil,
			Tier:        "IVPN Tier 2",
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(preauth))
	}))
	t.Cleanup(preauthServer.Close)

	mockRepo := mocks.NewSubscriptionRepository(t)
	mockProfileRepo := mocks.NewProfileRepository(t)
	mockCache := mocks.NewCachecache(t)

	apiCfg := config.APIConfig{PreauthURL: preauthServer.URL}
	svc := subscription.NewSubscriptionService(
		mockRepo,
		mockProfileRepo,
		mockCache,
		config.ServiceConfig{},
		apiCfg,
		webhookClient.Http{Cfg: apiCfg},
	)

	mockCache.On("GetPASession", mock.Anything, sessionID).Return(&model.PASession{
		ID:        sessionID,
		Token:     token,
		PreauthID: preauthID,
	}, nil)

	// Upsert must succeed for the clear-legacy-type call to be reached.
	mockRepo.On("Upsert", mock.Anything, mock.AnythingOfType("model.Subscription")).Return(nil)

	// Core assertion: ClearLegacyType is called with the account's hex ID after
	// a successful resync. If this expectation isn't met, mockery fails the test.
	mockRepo.On("ClearLegacyType", mock.Anything, accountOID.Hex()).Return(nil)

	// repopulateProfileCache fetches profiles; no profiles == best-effort no-op.
	mockProfileRepo.On("GetProfilesByAccountId", mock.Anything, accountOID.Hex()).Return([]model.Profile{}, nil)

	sub := &model.Subscription{
		ID:          subID,
		AccountID:   accountOID,
		Type:        "Managed", // pre-0.1.8 legacy value, must be cleared by the resync flow
		ActiveUntil: time.Now().Add(-1 * time.Hour),
	}

	// subID empty: webhook is skipped per SY7, so the call returns nil and
	// the ClearLegacyType mockery assertion is the contract being verified.
	err := svc.UpdateSubscriptionFromPASession(context.Background(), sub, sessionID, "")
	require.NoError(t, err)
}
