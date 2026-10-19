package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/passkey"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// specRef: F8, F8a
func TestDeletePasskeyByIDKeepsASignInMethod(t *testing.T) {
	t.Parallel()

	password := "hash"
	tests := []struct {
		name        string
		password    *string
		credentials int
		expectedErr error
	}{
		{name: "another passkey remains", credentials: 2},
		{name: "last passkey, password set", password: &password, credentials: 1},
		{name: "last passkey, no password", credentials: 1, expectedErr: passkey.ErrLastSignInMethod},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			accountID := primitive.NewObjectID()
			credentialID := primitive.NewObjectID()
			acc := &model.Account{ID: accountID, Password: tt.password, Tokens: []model.Token{
				{Type: auth.TokenTypeReauthPasskeyDelete, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)},
			}}

			store := mocks.NewDb(t)
			svc := &Service{Store: store}
			store.On("GetCredentialByID", mock.Anything, credentialID).Return(&model.Credential{AccountID: accountID}, nil).Once()
			store.On("GetCredentialsCount", mock.Anything, accountID).Return(tt.credentials, nil).Once()
			store.On("GetAccountById", mock.Anything, accountID.Hex()).Return(acc, nil)
			if tt.expectedErr == nil {
				store.On("UpdateAccount", mock.Anything, acc).Return(acc, nil).Once()
				store.On("DeleteCredentialByID", mock.Anything, credentialID, accountID).Return(nil).Once()
			}

			reauthToken := "reauth-token"
			err := svc.DeletePasskeyByID(ctx, credentialID, accountID, nil, &reauthToken, &model.MfaData{})

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				store.AssertNotCalled(t, "UpdateAccount", mock.Anything, mock.Anything)
				store.AssertNotCalled(t, "DeleteCredentialByID", mock.Anything, mock.Anything, mock.Anything)
				require.Len(t, acc.Tokens, 1, "a refused deletion must not consume the reauth token")
				return
			}
			require.NoError(t, err)
			require.Empty(t, acc.Tokens)
			store.AssertExpectations(t)
		})
	}
}

func newTestWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "modDNS Test", RPID: "test.local", RPOrigins: []string{"https://app.test.local"}})
	require.NoError(t, err)
	return wa
}

func ceremonyFor(ceremonyType string, accountID primitive.ObjectID, purpose, subID string) any {
	return mock.MatchedBy(func(c *model.Ceremony) bool {
		return c.Type == ceremonyType && c.AccountID == accountID.Hex() && c.Purpose == purpose && c.SubID == subID && c.SessionData.Challenge != ""
	})
}

// specRef: F9
func TestBeginCeremoniesKeepStateOutOfSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	account := &model.Account{ID: primitive.NewObjectID(), Email: "user@example.com"}
	credentials := []model.Credential{{CredentialID: []byte{0x01}, PublicKey: []byte{0xAA}}}

	tests := []struct {
		name  string
		setup func(store *mocks.Db)
		begin func(svc *Service) (string, error)
		match any
	}{
		{
			name:  "registration",
			setup: func(store *mocks.Db) {},
			begin: func(svc *Service) (string, error) {
				_, token, err := svc.BeginRegistration(ctx, account, "sub-1")
				return token, err
			},
			match: ceremonyFor(model.CeremonyRegistration, account.ID, "", "sub-1"),
		},
		{
			name: "login",
			setup: func(store *mocks.Db) {
				store.On("GetAccountByEmail", mock.Anything, account.Email).Return(account, nil).Once()
				store.On("GetCredentials", mock.Anything, account.ID).Return(credentials, nil).Once()
			},
			begin: func(svc *Service) (string, error) {
				_, token, err := svc.BeginLogin(ctx, account.Email)
				return token, err
			},
			match: ceremonyFor(model.CeremonyLogin, account.ID, "", ""),
		},
		{
			name: "add passkey",
			setup: func(store *mocks.Db) {
				verified := *account
				verified.Tokens = []model.Token{{Type: auth.TokenTypeReauthPasskeyAdd, Value: "reauth-token", ExpiresAt: time.Now().Add(time.Minute)}}
				store.On("GetAccountById", mock.Anything, account.ID.Hex()).Return(&verified, nil).Once()
				store.On("UpdateAccount", mock.Anything, mock.Anything).Return(&verified, nil).Once()
				store.On("GetCredentials", mock.Anything, account.ID).Return(credentials, nil).Once()
			},
			begin: func(svc *Service) (string, error) {
				reauthToken := "reauth-token"
				_, token, err := svc.BeginAddPasskey(ctx, account.ID.Hex(), nil, &reauthToken, &model.MfaData{})
				return token, err
			},
			match: ceremonyFor(model.CeremonyAddPasskey, account.ID, "", ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := mocks.NewDb(t)
			c := mocks.NewCachecache(t)
			svc := &Service{Store: store, Cache: c, Webauthn: newTestWebAuthn(t)}
			tt.setup(store)

			var savedToken string
			c.On("SaveCeremony", mock.Anything, mock.AnythingOfType("string"), tt.match, model.CeremonyTimeout).
				Run(func(args mock.Arguments) { savedToken = args.String(1) }).Return(nil).Once()

			token, err := tt.begin(svc)
			require.NoError(t, err)
			require.NotEmpty(t, token)
			require.Equal(t, savedToken, token)
		})
	}
}

// specRef: F9
func TestFinishCeremoniesConsumeStateFromCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name   string
		finish func(svc *Service) error
	}{
		{"registration", func(svc *Service) error { return svc.FinishRegistration(ctx, "token", nil, "") }},
		{"login", func(svc *Service) error { _, _, _, err := svc.FinishLogin(ctx, "token", nil, true); return err }},
		{"add passkey", func(svc *Service) error { return svc.FinishAddPasskey(ctx, "acc-1", "token", nil) }},
		{"reauth", func(svc *Service) error { _, err := svc.FinishReauth(ctx, "acc-1", "token", nil); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := mocks.NewDb(t)
			c := mocks.NewCachecache(t)
			svc := &Service{Store: store, Cache: c, Webauthn: newTestWebAuthn(t)}
			c.On("ConsumeCeremony", mock.Anything, "token").Return(nil, errors.New("redis: nil")).Once()

			require.Error(t, tt.finish(svc))
		})
	}
}

// specRef: F5
func TestReauthPurposesIssueTheirOwnTokenType(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"email_change":     auth.TokenTypeReauthEmailChange,
		"account_deletion": auth.TokenTypeReauthAccountDeletion,
		"profile_export":   auth.TokenTypeReauthProfileExport,
		"profile_import":   auth.TokenTypeReauthProfileImport,
		"password_set":     auth.TokenTypeReauthPasswordSet,
		"passkey_add":      auth.TokenTypeReauthPasskeyAdd,
		"passkey_delete":   auth.TokenTypeReauthPasskeyDelete,
	}
	for purpose, want := range tests {
		got, ok := reauthTokenType(purpose)
		require.True(t, ok, purpose)
		require.Equal(t, want, got, purpose)

		token, err := auth.NewToken(got)
		require.NoError(t, err, purpose)
		require.WithinDuration(t, time.Now().Add(5*time.Minute), token.ExpiresAt, 5*time.Second, purpose)
	}

	_, ok := reauthTokenType("unknown")
	require.False(t, ok)
}
