package mongodb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/model"
)

// UpdateCredential writes the counter and flags of the latest assertion and leaves
// the rest of the record, including the registration data, untouched.
// specRef: F15
func TestCredentialRepository_UpdateCredentialStoresAssertionFlags(t *testing.T) {
	client, container := startTestMongo(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
		_ = container.Terminate(ctx)
	})
	ctx := context.Background()
	repo := NewCredentialRepository(client, firstNonEmpty(os.Getenv("DB_TEST_NAME"), "dns_test")+"_credentials", CredentialsCollection)

	accountID := primitive.NewObjectID()
	created := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	cred := &model.Credential{
		ID:            primitive.NewObjectID(),
		CredentialID:  []byte("credential-1"),
		PublicKey:     []byte("public-key"),
		Flags:         model.CredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: true, BackupState: true},
		Authenticator: model.Authenticator{AAGUID: []byte("aaguid"), SignCount: 1, Attachment: "platform"},
		AccountID:     accountID,
		CreatedAt:     created,
		UpdatedAt:     created,
	}
	require.NoError(t, repo.SaveCredential(ctx, cred))

	flags := model.CredentialFlags{UserPresent: true, UserVerified: false, BackupEligible: true, BackupState: false}
	require.NoError(t, repo.UpdateCredential(ctx, cred.CredentialID, accountID, 5, flags))

	stored, err := repo.GetCredentialByID(ctx, cred.ID)
	require.NoError(t, err)
	require.Equal(t, flags, stored.Flags)
	require.Equal(t, uint32(5), stored.Authenticator.SignCount)
	require.Equal(t, []byte("aaguid"), stored.Authenticator.AAGUID)
	require.Equal(t, []byte("public-key"), stored.PublicKey)
	require.True(t, stored.UpdatedAt.After(created))

	require.NoError(t, repo.UpdateCredential(ctx, cred.CredentialID, primitive.NewObjectID(), 9, model.CredentialFlags{}),
		"another account's credential is not matched")
	stored, err = repo.GetCredentialByID(ctx, cred.ID)
	require.NoError(t, err)
	require.Equal(t, uint32(5), stored.Authenticator.SignCount)
	require.Equal(t, flags, stored.Flags)
}
