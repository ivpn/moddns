package webauthn

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/model"
)

const (
	testRPID   = "test.local"
	testRPName = "modDNS Test"
	testOrigin = "https://app.test.local"
)

var errNotFound = errors.New("not found")

// memStore is a CeremonyStore in memory.
type memStore struct {
	mu     sync.Mutex
	states map[string][]byte
	ttl    map[string]time.Duration
}

func newMemStore() *memStore {
	return &memStore{states: map[string][]byte{}, ttl: map[string]time.Duration{}}
}

func (s *memStore) SaveCeremony(_ context.Context, token string, state []byte, expiresIn time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[token] = state
	s.ttl[token] = expiresIn
	return nil
}

func (s *memStore) ConsumeCeremony(_ context.Context, token string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[token]
	if !ok {
		return nil, errNotFound
	}
	delete(s.states, token)
	return state, nil
}

// testUser is an account with the credentials registered so far.
type testUser struct {
	user          User
	authenticator virtualwebauthn.Authenticator
}

func newTestRP(t *testing.T) (*RelyingParty, *memStore) {
	t.Helper()
	store := newMemStore()
	rp, err := New(Config{RPID: testRPID, RPDisplayName: testRPName, RPOrigins: []string{testOrigin}}, store)
	require.NoError(t, err)
	return rp, store
}

func newTestUser() *testUser {
	id := primitive.NewObjectID()
	handle := []byte(id.Hex())
	return &testUser{
		user:          User{AccountID: id, Handle: handle, Name: "user@example.com", DisplayName: "user@example.com"},
		authenticator: virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: handle}),
	}
}

func (u *testUser) load(_ context.Context, accountID string) (User, error) {
	if accountID != u.user.AccountID.Hex() {
		return User{}, errNotFound
	}
	return u.user, nil
}

var virtualRP = virtualwebauthn.RelyingParty{ID: testRPID, Name: testRPName, Origin: testOrigin}

func browserRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func attest(t *testing.T, options *CredentialCreation, u *testUser, cred virtualwebauthn.Credential) string {
	t.Helper()
	raw, err := json.Marshal(options)
	require.NoError(t, err)
	parsed, err := virtualwebauthn.ParseAttestationOptions(string(raw))
	require.NoError(t, err)
	return virtualwebauthn.CreateAttestationResponse(virtualRP, u.authenticator, cred, *parsed)
}

func assert(t *testing.T, options *CredentialAssertion, u *testUser, cred virtualwebauthn.Credential) string {
	t.Helper()
	raw, err := json.Marshal(options)
	require.NoError(t, err)
	parsed, err := virtualwebauthn.ParseAssertionOptions(string(raw))
	require.NoError(t, err)
	return virtualwebauthn.CreateAssertionResponse(virtualRP, u.authenticator, cred, *parsed)
}

// register runs signup for u and stores the new credential on it.
func register(t *testing.T, rp *RelyingParty, u *testUser) virtualwebauthn.Credential {
	t.Helper()
	ctx := context.Background()
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	options, token, err := rp.BeginRegistration(ctx, u.user, "sub-1")
	require.NoError(t, err)
	result, err := rp.FinishRegistration(ctx, token, u.load, browserRequest(attest(t, options, u, cred)))
	require.NoError(t, err)

	u.user.Credentials = append(u.user.Credentials, *result.Credential)
	u.authenticator.AddCredential(cred)
	return cred
}

// specRef: F1, F2
func TestRegistration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, _ := newTestRP(t)
	u := newTestUser()
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	options, token, err := rp.BeginRegistration(ctx, u.user, "sub-1")
	require.NoError(t, err)
	result, err := rp.FinishRegistration(ctx, token, u.load, browserRequest(attest(t, options, u, cred)))
	require.NoError(t, err)

	require.Equal(t, u.user.AccountID.Hex(), result.AccountID)
	require.Equal(t, "sub-1", result.SubID)
	require.Equal(t, cred.ID, result.Credential.CredentialID)
	require.Equal(t, u.user.AccountID, result.Credential.AccountID)
	require.True(t, result.Credential.Flags.UserVerified)
}

// specRef: A4, A5, F5, F6
func TestAssertions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, _ := newTestRP(t)
	u := newTestUser()
	cred := register(t, rp, u)

	cred.Counter = 4
	options, token, err := rp.BeginLogin(ctx, u.user)
	require.NoError(t, err)
	result, err := rp.FinishLogin(ctx, token, u.load, browserRequest(assert(t, options, u, cred)))
	require.NoError(t, err)
	require.Equal(t, u.user.AccountID.Hex(), result.AccountID)
	require.Equal(t, cred.ID, result.CredentialID)
	require.Equal(t, uint32(4), result.SignCount)
	require.Nil(t, result.Credential, "an assertion yields no record to store")

	u.user.Credentials[0].Authenticator.SignCount = 4
	cred.Counter = 5
	options, token, err = rp.BeginReauth(ctx, u.user, "email_change")
	require.NoError(t, err)
	result, err = rp.FinishReauth(ctx, token, u.user.AccountID.Hex(), u.load, browserRequest(assert(t, options, u, cred)))
	require.NoError(t, err)
	require.Equal(t, "email_change", result.Purpose)
	require.Equal(t, uint32(5), result.SignCount)
}

// specRef: A5
func TestCloneDetected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, _ := newTestRP(t)
	u := newTestUser()
	cred := register(t, rp, u)
	u.user.Credentials[0].Authenticator.SignCount = 10

	cred.Counter = 3
	options, token, err := rp.BeginLogin(ctx, u.user)
	require.NoError(t, err)
	result, err := rp.FinishLogin(ctx, token, u.load, browserRequest(assert(t, options, u, cred)))
	require.ErrorIs(t, err, ErrCloneDetected)
	require.Nil(t, result)
}

// specRef: F3b
func TestBeginAddPasskeyExcludesCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, _ := newTestRP(t)
	u := newTestUser()
	first := register(t, rp, u)

	options, token, err := rp.BeginAddPasskey(ctx, u.user)
	require.NoError(t, err)
	raw, err := json.Marshal(options)
	require.NoError(t, err)
	parsed, err := virtualwebauthn.ParseAttestationOptions(string(raw))
	require.NoError(t, err)
	require.True(t, first.IsExcludedForAttestation(*parsed))

	second := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	result, err := rp.FinishAddPasskey(ctx, token, u.user.AccountID.Hex(), u.load, browserRequest(attest(t, options, u, second)))
	require.NoError(t, err)
	require.Equal(t, second.ID, result.Credential.CredentialID)
}

// specRef: F9, F10
func TestFinishAcceptsOnlyItsOwnCeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	other := primitive.NewObjectID().Hex()

	tests := []struct {
		name   string
		finish func(rp *RelyingParty, u *testUser, token string) (*Result, error)
	}{
		{"login state finished as reauth", func(rp *RelyingParty, u *testUser, token string) (*Result, error) {
			return rp.FinishReauth(ctx, token, u.user.AccountID.Hex(), u.load, browserRequest("{}"))
		}},
		{"login state finished as registration", func(rp *RelyingParty, u *testUser, token string) (*Result, error) {
			return rp.FinishRegistration(ctx, token, u.load, browserRequest("{}"))
		}},
		{"login state finished as add passkey", func(rp *RelyingParty, u *testUser, token string) (*Result, error) {
			return rp.FinishAddPasskey(ctx, token, u.user.AccountID.Hex(), u.load, browserRequest("{}"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rp, store := newTestRP(t)
			u := newTestUser()
			register(t, rp, u)

			_, token, err := rp.BeginLogin(ctx, u.user)
			require.NoError(t, err)
			_, err = tt.finish(rp, u, token)
			require.ErrorIs(t, err, ErrCeremonyMismatch)
			require.Empty(t, store.states, "the refused state is consumed")
		})
	}

	t.Run("reauth state finished by another account", func(t *testing.T) {
		t.Parallel()
		rp, store := newTestRP(t)
		u := newTestUser()
		register(t, rp, u)

		_, token, err := rp.BeginReauth(ctx, u.user, "email_change")
		require.NoError(t, err)
		_, err = rp.FinishReauth(ctx, token, other, u.load, browserRequest("{}"))
		require.ErrorIs(t, err, ErrCeremonyMismatch)
		require.Empty(t, store.states)
	})

	t.Run("account-bound state finished without an account", func(t *testing.T) {
		t.Parallel()
		rp, store := newTestRP(t)
		u := newTestUser()
		register(t, rp, u)

		_, token, err := rp.BeginReauth(ctx, u.user, "email_change")
		require.NoError(t, err)
		_, err = rp.FinishReauth(ctx, token, "", u.load, browserRequest("{}"))
		require.ErrorIs(t, err, ErrCeremonyMismatch)

		_, token, err = rp.BeginAddPasskey(ctx, u.user)
		require.NoError(t, err)
		_, err = rp.FinishAddPasskey(ctx, token, "", u.load, browserRequest("{}"))
		require.ErrorIs(t, err, ErrCeremonyMismatch)
		require.Empty(t, store.states)
	})

	t.Run("add passkey state finished by another account", func(t *testing.T) {
		t.Parallel()
		rp, store := newTestRP(t)
		u := newTestUser()
		register(t, rp, u)

		_, token, err := rp.BeginAddPasskey(ctx, u.user)
		require.NoError(t, err)
		_, err = rp.FinishAddPasskey(ctx, token, other, u.load, browserRequest("{}"))
		require.ErrorIs(t, err, ErrCeremonyMismatch)
		require.Empty(t, store.states)
	})
}

// specRef: F9
func TestCeremonyStateIsSingleUse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, store := newTestRP(t)
	u := newTestUser()
	cred := register(t, rp, u)

	options, token, err := rp.BeginLogin(ctx, u.user)
	require.NoError(t, err)
	require.Equal(t, CeremonyTimeout, store.ttl[token])
	response := assert(t, options, u, cred)

	_, err = rp.FinishLogin(ctx, token, u.load, browserRequest(response))
	require.NoError(t, err)
	_, err = rp.FinishLogin(ctx, token, u.load, browserRequest(response))
	require.ErrorIs(t, err, errNotFound)
}

// The server rejects a response to options older than the ceremony timeout.
// specRef: F9
func TestCeremonyExpires(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rp, store := newTestRP(t)
	u := newTestUser()
	cred := register(t, rp, u)

	options, token, err := rp.BeginLogin(ctx, u.user)
	require.NoError(t, err)

	var c ceremony
	require.NoError(t, json.Unmarshal(store.states[token], &c))
	require.WithinDuration(t, time.Now().Add(CeremonyTimeout), c.SessionData.Expires, 5*time.Second)
	c.SessionData.Expires = time.Now().Add(-time.Second)
	store.states[token], err = json.Marshal(c)
	require.NoError(t, err)

	_, err = rp.FinishLogin(ctx, token, u.load, browserRequest(assert(t, options, u, cred)))
	require.Error(t, err)

	_, token, err = rp.BeginRegistration(ctx, u.user, "")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(store.states[token], &c))
	require.WithinDuration(t, time.Now().Add(CeremonyTimeout), c.SessionData.Expires, 5*time.Second)
}

func TestCredentialRecordRoundTrip(t *testing.T) {
	t.Parallel()
	accountID := primitive.NewObjectID()
	record := model.Credential{
		CredentialID:    []byte{0x01, 0x02},
		PublicKey:       []byte{0xAA},
		AttestationType: "none",
		Transport:       []string{"internal", "hybrid"},
		Flags:           model.CredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: true, BackupState: true},
		Authenticator:   model.Authenticator{AAGUID: []byte{0x09}, SignCount: 7, CloneWarning: true, Attachment: "platform"},
	}

	lib := toLibCredential(&record)
	got := toRecord(&lib, accountID)

	require.Equal(t, accountID, got.AccountID)
	require.Equal(t, "AQI=", got.CredentialIDString)
	require.False(t, got.ID.IsZero())
	got.ID, got.AccountID, got.CredentialIDString, got.CreatedAt, got.UpdatedAt = primitive.ObjectID{}, primitive.ObjectID{}, "", time.Time{}, time.Time{}
	require.Equal(t, record, *got)
}
