package service

import (
	"context"
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDeletePasskeyByIDLeavesAuthMethodWhenCredentialsRemain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := primitive.NewObjectID()
	credentialID := primitive.NewObjectID()

	store := mocks.NewDb(t)
	svc := &Service{Store: store}

	store.On("GetCredentialByID", mock.Anything, credentialID).Return(&model.Credential{AccountID: accountID}, nil).Once()
	store.On("DeleteCredentialByID", mock.Anything, credentialID, accountID).Return(nil).Once()

	err := svc.DeletePasskeyByID(ctx, credentialID, accountID)
	require.NoError(t, err)

	store.AssertExpectations(t)
	store.AssertNotCalled(t, "GetCredentials", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "GetAccount", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "UpdateAccount", mock.Anything, mock.Anything)
}

func TestDeletePasskeyByIDSkipsAccountUpdateWhenLastCredential(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := primitive.NewObjectID()
	credentialID := primitive.NewObjectID()

	store := mocks.NewDb(t)
	svc := &Service{Store: store}

	store.On("GetCredentialByID", mock.Anything, credentialID).Return(&model.Credential{AccountID: accountID}, nil).Once()
	store.On("DeleteCredentialByID", mock.Anything, credentialID, accountID).Return(nil).Once()

	err := svc.DeletePasskeyByID(ctx, credentialID, accountID)
	require.NoError(t, err)

	store.AssertExpectations(t)
	store.AssertNotCalled(t, "GetCredentials", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "GetAccount", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "UpdateAccount", mock.Anything, mock.Anything)
}

func newTestWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "modDNS Test", RPID: "test.local", RPOrigins: []string{"https://app.test.local"}})
	require.NoError(t, err)
	return wa
}

func ceremonyFor(accountID primitive.ObjectID, purpose, subID string) any {
	return mock.MatchedBy(func(c *model.Ceremony) bool {
		return c.AccountID == accountID.Hex() && c.Purpose == purpose && c.SubID == subID && c.SessionData.Challenge != ""
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
			match: ceremonyFor(account.ID, "", "sub-1"),
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
			match: ceremonyFor(account.ID, "", ""),
		},
		{
			name: "add passkey",
			setup: func(store *mocks.Db) {
				store.On("GetCredentials", mock.Anything, account.ID).Return(credentials, nil).Once()
			},
			begin: func(svc *Service) (string, error) {
				_, token, err := svc.BeginAddPasskey(ctx, account)
				return token, err
			},
			match: ceremonyFor(account.ID, "", ""),
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
		{"add passkey", func(svc *Service) error { return svc.FinishAddPasskey(ctx, "token", nil) }},
		{"reauth", func(svc *Service) error { _, err := svc.FinishReauth(ctx, "token", nil); return err }},
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
