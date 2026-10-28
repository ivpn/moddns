package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/descope/virtualwebauthn"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/webauthn"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/passkey"
)

// The harness runs the passkey flows end to end: the service with the real
// go-webauthn relying party, ceremony state in Redis (miniredis) and a virtual
// authenticator producing the browser's responses. Only the database is faked.

const (
	harnessRPID    = "test.local"
	harnessRPName  = "modDNS Test"
	harnessOrigin  = "https://app.test.local"
	harnessSubID   = "sub-1"
	harnessSession = "pa-session"
)

// passkeyStore keeps the accounts, credentials and login sessions the passkey flows
// read and write. Any other store call fails the test through the embedded mock.
type passkeyStore struct {
	*mocks.Db
	mu          sync.Mutex
	accounts    map[string]*model.Account
	credentials []model.Credential
	sessions    map[string]model.Session
}

func (s *passkeyStore) GetAccount(_ context.Context, accountID string) (*model.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[accountID]
	if !ok {
		return nil, errors.New("account not found")
	}
	return acc, nil
}

func (s *passkeyStore) GetAccountById(ctx context.Context, accountID string) (*model.Account, error) {
	return s.GetAccount(ctx, accountID)
}

func (s *passkeyStore) GetAccountByEmail(_ context.Context, email string) (*model.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, acc := range s.accounts {
		if acc.Email == email {
			return acc, nil
		}
	}
	return nil, errors.New("account not found")
}

func (s *passkeyStore) UpdateAccount(_ context.Context, acc *model.Account) (*model.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[acc.ID.Hex()] = acc
	return acc, nil
}

func (s *passkeyStore) GetCredentials(_ context.Context, accountID primitive.ObjectID) ([]model.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var creds []model.Credential
	for _, c := range s.credentials {
		if c.AccountID == accountID {
			creds = append(creds, c)
		}
	}
	return creds, nil
}

func (s *passkeyStore) GetCredentialsCount(ctx context.Context, accountID primitive.ObjectID) (int, error) {
	creds, err := s.GetCredentials(ctx, accountID)
	return len(creds), err
}

func (s *passkeyStore) GetCredentialByID(_ context.Context, id primitive.ObjectID) (*model.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.credentials {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, errors.New("credential not found")
}

func (s *passkeyStore) SaveCredential(_ context.Context, credential *model.Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials = append(s.credentials, *credential)
	return nil
}

// UpdateCredential mirrors the Mongo repository: only the sign count is written.
func (s *passkeyStore) UpdateCredential(_ context.Context, credentialID []byte, accountID primitive.ObjectID, signCount uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.credentials {
		if string(c.CredentialID) == string(credentialID) && c.AccountID == accountID {
			s.credentials[i].Authenticator.SignCount = signCount
		}
	}
	return nil
}

func (s *passkeyStore) DeleteCredentialByID(_ context.Context, id, accountID primitive.ObjectID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.credentials[:0]
	for _, c := range s.credentials {
		if c.ID != id || c.AccountID != accountID {
			kept = append(kept, c)
		}
	}
	s.credentials = kept
	return nil
}

// SaveSession mirrors the Mongo repository, which stores the payload as JSON bytes.
func (s *passkeyStore) SaveSession(_ context.Context, sessionData gowebauthn.SessionData, token, accountID string) error {
	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = model.Session{AccountID: accountID, Token: token, Data: data}
	return nil
}

type passkeyHarness struct {
	t     *testing.T
	ctx   context.Context
	svc   *Service
	store *passkeyStore
	redis *miniredis.Miniredis
	rp    virtualwebauthn.RelyingParty
	// completed records the CompleteRegistration calls as subID/sessionID/tokenHash.
	completed []string
}

func newPasskeyHarness(t *testing.T) *passkeyHarness {
	t.Helper()

	cfg := config.Config{
		Server:  &config.ServerConfig{Name: harnessRPName, FQDN: harnessRPID},
		API:     &config.APIConfig{ApiAllowOrigin: harnessOrigin, SessionExpirationTime: time.Hour},
		Service: &config.ServiceConfig{MaxCredentials: 10, IdLimiterMax: 100, IdLimiterExpiration: time.Minute},
	}
	mr := miniredis.RunT(t)
	redisCache := cache.NewRedisCacheFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	rp, err := webauthn.New(webauthn.Config{
		RPID:          cfg.Server.FQDN,
		RPDisplayName: cfg.Server.Name,
		RPOrigins:     []string{cfg.API.ApiAllowOrigin},
	}, redisCache)
	require.NoError(t, err)
	store := &passkeyStore{Db: mocks.NewDb(t), accounts: map[string]*model.Account{}, sessions: map[string]model.Session{}}
	accounts := mocks.NewAccountServicer(t)
	subscriptions := mocks.NewSubscriptionServicer(t)

	h := &passkeyHarness{
		t:     t,
		ctx:   context.Background(),
		store: store,
		redis: mr,
		rp:    virtualwebauthn.RelyingParty{ID: harnessRPID, Name: harnessRPName, Origin: harnessOrigin},
		svc: &Service{
			Cfg:                  cfg,
			Store:                store,
			Cache:                redisCache,
			Webauthn:             rp,
			AccountServicer:      accounts,
			SubscriptionServicer: subscriptions,
		},
	}

	accounts.On("GetAccount", mock.Anything, mock.Anything).Return(store.GetAccount).Maybe()
	accounts.On("CompleteRegistration", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			h.completed = append(h.completed, args.String(2)+"/"+args.String(3)+"/"+args.String(4))
		}).Return(nil).Maybe()
	subscriptions.On("GetSubscription", mock.Anything, mock.Anything).
		Return(&model.Subscription{TokenHash: "token-hash"}, nil).Maybe()

	return h
}

func (h *passkeyHarness) newAccount(email string, password *string) *model.Account {
	acc := &model.Account{ID: primitive.NewObjectID(), Email: email, Password: password}
	h.store.accounts[acc.ID.Hex()] = acc
	return acc
}

// newAuthenticator returns an authenticator that answers for acc with one new credential.
func (h *passkeyHarness) newAuthenticator(acc *model.Account) (virtualwebauthn.Authenticator, virtualwebauthn.Credential) {
	authenticator := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: acc.WebAuthnID()})
	return authenticator, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
}

// grantReauthToken stores a reauth token of tokenType on acc and returns its value.
func (h *passkeyHarness) grantReauthToken(acc *model.Account, tokenType string) string {
	token, err := auth.NewToken(tokenType)
	require.NoError(h.t, err)
	acc.Tokens = append(acc.Tokens, *token)
	return token.Value
}

func (h *passkeyHarness) credentials(acc *model.Account) []model.Credential {
	creds, err := h.store.GetCredentials(h.ctx, acc.ID)
	require.NoError(h.t, err)
	return creds
}

// storedCredential returns the stored record of the virtual credential.
func (h *passkeyHarness) storedCredential(acc *model.Account, cred virtualwebauthn.Credential) model.Credential {
	for _, c := range h.credentials(acc) {
		if string(c.CredentialID) == string(cred.ID) {
			return c
		}
	}
	h.t.Fatalf("credential %x not stored", cred.ID)
	return model.Credential{}
}

// ceremonyKeys lists the ceremony states held in Redis.
func (h *passkeyHarness) ceremonyKeys() []string {
	var keys []string
	for _, k := range h.redis.Keys() {
		if strings.HasPrefix(k, "webauthn_ceremony:") {
			keys = append(keys, k)
		}
	}
	return keys
}

func browserRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func (h *passkeyHarness) attest(options any, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) string {
	parsed, err := virtualwebauthn.ParseAttestationOptions(toJSON(h.t, options))
	require.NoError(h.t, err)
	return virtualwebauthn.CreateAttestationResponse(h.rp, authenticator, cred, *parsed)
}

func (h *passkeyHarness) assert(options any, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) string {
	parsed, err := virtualwebauthn.ParseAssertionOptions(toJSON(h.t, options))
	require.NoError(h.t, err)
	return virtualwebauthn.CreateAssertionResponse(h.rp, authenticator, cred, *parsed)
}

// register runs the signup ceremony for acc with cred.
func (h *passkeyHarness) register(acc *model.Account, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) error {
	options, token, err := h.svc.BeginRegistration(h.ctx, acc, harnessSubID)
	require.NoError(h.t, err)
	return h.svc.FinishRegistration(h.ctx, token, browserRequest(h.attest(options, authenticator, cred)), harnessSession)
}

// login runs the login ceremony and returns the account and the login session token.
func (h *passkeyHarness) login(acc *model.Account, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) (*model.Account, string, error) {
	options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(h.t, err)
	account, session, _, err := h.svc.FinishLogin(h.ctx, token, browserRequest(h.assert(options, authenticator, cred)), true)
	return account, session, err
}

// reauth runs the reauth ceremony for purpose and returns the issued reauth token.
func (h *passkeyHarness) reauth(acc *model.Account, purpose string, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) (*model.Token, error) {
	options, token, err := h.svc.BeginReauth(h.ctx, purpose, acc.ID.Hex())
	require.NoError(h.t, err)
	return h.svc.FinishReauth(h.ctx, acc.ID.Hex(), token, browserRequest(h.assert(options, authenticator, cred)))
}

// addPasskey runs the add-passkey ceremony, verified by a passkey_add reauth token.
func (h *passkeyHarness) addPasskey(acc *model.Account, reauthToken string, authenticator virtualwebauthn.Authenticator, cred virtualwebauthn.Credential) error {
	options, token, err := h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &reauthToken, &model.MfaData{})
	require.NoError(h.t, err)
	return h.svc.FinishAddPasskey(h.ctx, acc.ID.Hex(), token, browserRequest(h.attest(options, authenticator, cred)))
}

// registered returns a passkey-only account holding one registered credential.
func (h *passkeyHarness) registered(email string) (*model.Account, virtualwebauthn.Authenticator, virtualwebauthn.Credential) {
	acc := h.newAccount(email, nil)
	authenticator, cred := h.newAuthenticator(acc)
	require.NoError(h.t, h.register(acc, authenticator, cred))
	authenticator.AddCredential(cred)
	return acc, authenticator, cred
}

// specRef: F1, F2
func TestPasskeyHarnessRegistration(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)

	acc, _, cred := h.registered("signup@example.com")

	stored := h.storedCredential(acc, cred)
	require.Equal(t, acc.ID, stored.AccountID)
	require.NotEmpty(t, stored.PublicKey)
	require.True(t, stored.Flags.UserPresent)
	require.True(t, stored.Flags.UserVerified)
	require.Equal(t, []string{harnessSubID + "/" + harnessSession + "/token-hash"}, h.completed)
	require.Empty(t, h.store.sessions, "signup does not open a login session")
}

// Signup saves its credential without the MaxCredentials check; add passkey applies it.
// specRef: F2, F4
func TestPasskeyHarnessMaxCredentials(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	h.svc.Cfg.Service.MaxCredentials = 0

	acc, authenticator, cred := h.registered("max@example.com")
	require.Len(t, h.credentials(acc), 1)

	reauth, err := h.reauth(acc, "passkey_add", authenticator, cred)
	require.NoError(t, err)
	_, second := h.newAuthenticator(acc)
	err = h.addPasskey(acc, reauth.Value, authenticator, second)
	require.ErrorIs(t, err, ErrMaxExceededCredential)
	require.Len(t, h.credentials(acc), 1)
}

// specRef: A4, A5
func TestPasskeyHarnessLogin(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("login@example.com")

	cred.Counter = 7
	account, sessionToken, err := h.login(acc, authenticator, cred)
	require.NoError(t, err)
	require.Equal(t, acc.ID, account.ID)
	require.NotEmpty(t, sessionToken)
	require.Equal(t, uint32(7), h.storedCredential(acc, cred).Authenticator.SignCount)

	session, ok := h.store.sessions[sessionToken]
	require.True(t, ok)
	require.Equal(t, acc.ID.Hex(), session.AccountID)
	require.NoError(t, session.UnmarshalSessionData())
	require.Empty(t, session.SessionData.Challenge, "a login session carries no challenge")
	require.Equal(t, acc.WebAuthnID(), session.SessionData.UserID)
	require.WithinDuration(t, time.Now().Add(time.Hour), session.SessionData.Expires, time.Minute)
}

// specRef: A5
func TestPasskeyHarnessLoginWithoutSession(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("nosession@example.com")

	options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(t, err)
	account, sessionToken, purpose, err := h.svc.FinishLogin(h.ctx, token, browserRequest(h.assert(options, authenticator, cred)), false)
	require.NoError(t, err)
	require.Equal(t, acc.ID, account.ID)
	require.Empty(t, sessionToken)
	require.Empty(t, purpose)
	require.Empty(t, h.store.sessions)
}

// A sign count that does not increase fails the login and leaves the stored count as it was.
// specRef: A5
func TestPasskeyHarnessLoginCloneWarning(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("clone@example.com")

	cred.Counter = 10
	_, _, err := h.login(acc, authenticator, cred)
	require.NoError(t, err)

	cred.Counter = 3
	_, _, err = h.login(acc, authenticator, cred)
	require.ErrorContains(t, err, "credential clone warning")
	require.Equal(t, uint32(10), h.storedCredential(acc, cred).Authenticator.SignCount)
	require.Len(t, h.store.sessions, 1)
}

// A credential the account doesn't hold cannot sign in.
// specRef: A5
func TestPasskeyHarnessLoginUnknownCredential(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, _ := h.registered("unknown@example.com")

	_, other := h.newAuthenticator(acc)
	_, _, err := h.login(acc, authenticator, other)
	require.Error(t, err)
	require.Empty(t, h.store.sessions)
}

// specRef: A4
func TestPasskeyHarnessBeginLoginWithoutPasskey(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	password := "hash"
	acc := h.newAccount("password-only@example.com", &password)

	_, _, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.Error(t, err)
	require.Empty(t, h.ceremonyKeys(), "no ceremony is started")
}

// specRef: F5, F6
func TestPasskeyHarnessReauth(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("reauth@example.com")

	for purpose, tokenType := range reauthTokenTypes {
		token, err := h.reauth(acc, purpose, authenticator, cred)
		require.NoError(t, err, purpose)
		require.Equal(t, tokenType, token.Type, purpose)
		require.Contains(t, acc.Tokens, *token, purpose)
	}

	_, _, err := h.svc.BeginReauth(h.ctx, "unknown", acc.ID.Hex())
	require.ErrorContains(t, err, "unsupported reauth purpose")
}

// specRef: F3, F3b, F4
func TestPasskeyHarnessAddPasskey(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("add@example.com")

	reauth, err := h.reauth(acc, "passkey_add", authenticator, cred)
	require.NoError(t, err)

	options, token, err := h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &reauth.Value, &model.MfaData{})
	require.NoError(t, err)
	parsed, err := virtualwebauthn.ParseAttestationOptions(toJSON(t, options))
	require.NoError(t, err)
	require.True(t, cred.IsExcludedForAttestation(*parsed), "the registered credential is excluded")

	second := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	require.False(t, second.IsExcludedForAttestation(*parsed))
	attestation := virtualwebauthn.CreateAttestationResponse(h.rp, authenticator, second, *parsed)
	require.NoError(t, h.svc.FinishAddPasskey(h.ctx, acc.ID.Hex(), token, browserRequest(attestation)))
	authenticator.AddCredential(second)
	require.Len(t, h.credentials(acc), 2)
	require.Empty(t, acc.Tokens, "the reauth token is consumed")

	_, _, err = h.login(acc, authenticator, second)
	require.NoError(t, err)
}

// specRef: F3
func TestPasskeyHarnessAddPasskeyNeedsVerification(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, _, _ := h.registered("add-unverified@example.com")

	wrong := "not-a-token"
	_, _, err := h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &wrong, &model.MfaData{})
	require.Error(t, err)
	require.Empty(t, h.ceremonyKeys(), "no ceremony is started")
}

// specRef: F8, F8a
func TestPasskeyHarnessDeletePasskey(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, first := h.registered("delete@example.com")

	second := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	require.NoError(t, h.addPasskey(acc, h.grantReauthToken(acc, auth.TokenTypeReauthPasskeyAdd), authenticator, second))
	authenticator.AddCredential(second)

	reauth, err := h.reauth(acc, "passkey_delete", authenticator, second)
	require.NoError(t, err)
	firstID := h.storedCredential(acc, first).ID
	require.NoError(t, h.svc.DeletePasskeyByID(h.ctx, firstID, acc.ID, nil, &reauth.Value, &model.MfaData{}))
	require.Len(t, h.credentials(acc), 1)

	_, _, err = h.login(acc, authenticator, first)
	require.Error(t, err, "a deleted passkey cannot sign in")
	_, _, err = h.login(acc, authenticator, second)
	require.NoError(t, err)

	reauth, err = h.reauth(acc, "passkey_delete", authenticator, second)
	require.NoError(t, err)
	err = h.svc.DeletePasskeyByID(h.ctx, h.storedCredential(acc, second).ID, acc.ID, nil, &reauth.Value, &model.MfaData{})
	require.ErrorIs(t, err, passkey.ErrLastSignInMethod)
	require.Len(t, h.credentials(acc), 1)
}

// specRef: F8
func TestPasskeyHarnessDeleteOtherAccountsPasskey(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	owner, _, cred := h.registered("owner@example.com")
	other, _, _ := h.registered("other@example.com")

	token := h.grantReauthToken(other, auth.TokenTypeReauthPasskeyDelete)
	err := h.svc.DeletePasskeyByID(h.ctx, h.storedCredential(owner, cred).ID, other.ID, nil, &token, &model.MfaData{})
	require.ErrorContains(t, err, "credential does not belong to account")
	require.Len(t, h.credentials(owner), 1)
	require.Len(t, other.Tokens, 1, "the reauth token is not consumed")
}

// A finish call accepts only state begun by its own ceremony type and, for add passkey
// and reauth, by the same account. Refused state is consumed all the same.
// specRef: F10
func TestPasskeyHarnessCeremonyBinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		begin   func(h *passkeyHarness, acc *model.Account) (any, string)
		finish  func(h *passkeyHarness, acc *model.Account, token, response string) error
		wantErr error
		// attest is true when the finish call expects a registration response.
		attest bool
	}{
		{
			name: "login state finished as reauth",
			begin: func(h *passkeyHarness, acc *model.Account) (any, string) {
				options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
				require.NoError(h.t, err)
				return options, token
			},
			finish: func(h *passkeyHarness, acc *model.Account, token, response string) error {
				_, err := h.svc.FinishReauth(h.ctx, acc.ID.Hex(), token, browserRequest(response))
				return err
			},
			wantErr: passkey.ErrFinishReauth,
		},
		{
			name: "reauth state finished as login",
			begin: func(h *passkeyHarness, acc *model.Account) (any, string) {
				options, token, err := h.svc.BeginReauth(h.ctx, "email_change", acc.ID.Hex())
				require.NoError(h.t, err)
				return options, token
			},
			finish: func(h *passkeyHarness, acc *model.Account, token, response string) error {
				_, _, _, err := h.svc.FinishLogin(h.ctx, token, browserRequest(response), true)
				return err
			},
			wantErr: passkey.ErrFinishLogin,
		},
		{
			name: "reauth state finished by another account",
			begin: func(h *passkeyHarness, acc *model.Account) (any, string) {
				options, token, err := h.svc.BeginReauth(h.ctx, "email_change", acc.ID.Hex())
				require.NoError(h.t, err)
				return options, token
			},
			finish: func(h *passkeyHarness, acc *model.Account, token, response string) error {
				_, err := h.svc.FinishReauth(h.ctx, primitive.NewObjectID().Hex(), token, browserRequest(response))
				return err
			},
			wantErr: passkey.ErrFinishReauth,
		},
		{
			name: "add passkey state finished by another account",
			begin: func(h *passkeyHarness, acc *model.Account) (any, string) {
				reauthToken := h.grantReauthToken(acc, auth.TokenTypeReauthPasskeyAdd)
				options, token, err := h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &reauthToken, &model.MfaData{})
				require.NoError(h.t, err)
				return options, token
			},
			finish: func(h *passkeyHarness, acc *model.Account, token, response string) error {
				return h.svc.FinishAddPasskey(h.ctx, primitive.NewObjectID().Hex(), token, browserRequest(response))
			},
			wantErr: passkey.ErrFinishAddPasskey,
			attest:  true,
		},
		{
			name: "add passkey state finished as signup",
			begin: func(h *passkeyHarness, acc *model.Account) (any, string) {
				reauthToken := h.grantReauthToken(acc, auth.TokenTypeReauthPasskeyAdd)
				options, token, err := h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &reauthToken, &model.MfaData{})
				require.NoError(h.t, err)
				return options, token
			},
			finish: func(h *passkeyHarness, acc *model.Account, token, response string) error {
				return h.svc.FinishRegistration(h.ctx, token, browserRequest(response), harnessSession)
			},
			wantErr: passkey.ErrFinishRegistration,
			attest:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newPasskeyHarness(t)
			acc, authenticator, cred := h.registered("binding@example.com")
			tokensBefore := len(acc.Tokens)

			options, token := tt.begin(h, acc)
			response := ""
			if tt.attest {
				response = h.attest(options, authenticator, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			} else {
				response = h.assert(options, authenticator, cred)
			}

			require.ErrorIs(t, tt.finish(h, acc, token, response), tt.wantErr)
			require.Len(t, h.credentials(acc), 1)
			require.Empty(t, h.store.sessions)
			require.LessOrEqual(t, len(acc.Tokens), tokensBefore, "no reauth token is issued")
			require.Empty(t, h.ceremonyKeys(), "the refused state is consumed")
		})
	}
}

// specRef: F9
func TestPasskeyHarnessCeremonyIsSingleUse(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("replay@example.com")

	options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(t, err)
	response := h.assert(options, authenticator, cred)

	_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(response), true)
	require.NoError(t, err)
	_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(response), true)
	require.Error(t, err, "a replayed finish call is refused")
	require.Len(t, h.store.sessions, 1)
}

// A failed finish call consumes the state, so a retry with a valid response fails too.
// specRef: F9
func TestPasskeyHarnessFailedFinishConsumesState(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("retry@example.com")

	options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(t, err)

	_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(`{}`), true)
	require.Error(t, err)
	_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(h.assert(options, authenticator, cred)), true)
	require.Error(t, err)
	require.Empty(t, h.store.sessions)
}

// specRef: F9
func TestPasskeyHarnessCeremonyExpires(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("expiry@example.com")

	options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(t, err)
	require.Equal(t, webauthn.CeremonyTimeout, h.redis.TTL("webauthn_ceremony:"+token))

	h.redis.FastForward(webauthn.CeremonyTimeout + time.Second)
	_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(h.assert(options, authenticator, cred)), true)
	require.Error(t, err)
	require.Empty(t, h.store.sessions)
}

// A response produced for another origin or RP ID is refused.
// specRef: A5
func TestPasskeyHarnessOriginAndRPID(t *testing.T) {
	t.Parallel()

	tests := map[string]virtualwebauthn.RelyingParty{
		"other origin": {ID: harnessRPID, Name: harnessRPName, Origin: "https://evil.test"},
		"other rp id":  {ID: "evil.test", Name: harnessRPName, Origin: harnessOrigin},
	}
	for name, rp := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newPasskeyHarness(t)
			acc, authenticator, cred := h.registered("origin@example.com")

			options, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
			require.NoError(t, err)
			parsed, err := virtualwebauthn.ParseAssertionOptions(toJSON(t, options))
			require.NoError(t, err)
			response := virtualwebauthn.CreateAssertionResponse(rp, authenticator, cred, *parsed)

			_, _, _, err = h.svc.FinishLogin(h.ctx, token, browserRequest(response), true)
			require.Error(t, err)
			require.Empty(t, h.store.sessions)
		})
	}
}

// normalizeWire replaces the random and per-account values of a wire document with
// placeholders so the rest of its shape can be compared with a fixture.
func normalizeWire(key string, v any) any {
	if v == "" {
		return v
	}
	switch key {
	case "challenge", "expires":
		return "<" + key + ">"
	case "user_id", "account_id":
		return "<account>"
	case "allowed_credentials":
		if list, ok := v.([]any); ok {
			for i := range list {
				list[i] = "<credential>"
			}
			return list
		}
	}
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			switch {
			case key == "user" && k == "id":
				val[k] = "<account>"
			case (key == "allowCredentials" || key == "excludeCredentials") && k == "id":
				val[k] = "<credential>"
			default:
				val[k] = normalizeWire(k, child)
			}
		}
	case []any:
		for i, child := range val {
			val[i] = normalizeWire(key, child)
		}
	}
	return v
}

// The JSON the browser receives, the ceremony state kept in Redis and the stored
// login session must keep their shape: in-flight ceremonies and existing sessions
// are read by instances running another build during a rolling deploy.
// Regenerate with: REFRESH_FIXTURES=1 go test ./service -run TestPasskeyHarnessWireFormats -count=1
// specRef: F1, F3, F5, F9, A4, A5
func TestPasskeyHarnessWireFormats(t *testing.T) {
	t.Parallel()
	h := newPasskeyHarness(t)
	acc, authenticator, cred := h.registered("wire@example.com")

	wire := map[string]any{}
	record := func(name string, raw []byte) {
		var doc any
		require.NoError(t, json.Unmarshal(raw, &doc), name)
		wire[name] = normalizeWire("", doc)
	}
	recordCeremony := func(name string, options any, token string) {
		record(name+"_options", []byte(toJSON(t, options)))
		state, err := h.redis.Get("webauthn_ceremony:" + token)
		require.NoError(t, err, name)
		record(name+"_ceremony", []byte(state))
	}

	options, token, err := h.svc.BeginRegistration(h.ctx, h.newAccount("wire-signup@example.com", nil), harnessSubID)
	require.NoError(t, err)
	recordCeremony("registration", options, token)

	loginOptions, token, err := h.svc.BeginLogin(h.ctx, acc.Email)
	require.NoError(t, err)
	recordCeremony("login", loginOptions, token)
	_, sessionToken, _, err := h.svc.FinishLogin(h.ctx, token, browserRequest(h.assert(loginOptions, authenticator, cred)), true)
	require.NoError(t, err)
	record("login_session", h.store.sessions[sessionToken].Data)

	reauthToken := h.grantReauthToken(acc, auth.TokenTypeReauthPasskeyAdd)
	options, token, err = h.svc.BeginAddPasskey(h.ctx, acc.ID.Hex(), nil, &reauthToken, &model.MfaData{})
	require.NoError(t, err)
	recordCeremony("add_passkey", options, token)

	loginOptions, token, err = h.svc.BeginReauth(h.ctx, "email_change", acc.ID.Hex())
	require.NoError(t, err)
	recordCeremony("reauth", loginOptions, token)

	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(wire))
	got := []byte(buf.String())

	path := filepath.Join("testdata", "passkeys", "wire_formats.json")
	if os.Getenv("REFRESH_FIXTURES") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
}
