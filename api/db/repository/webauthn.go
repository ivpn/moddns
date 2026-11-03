package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebAuthnCredentialRepository defines the interface for credential operations
type WebAuthnCredentialRepository interface {
	SaveCredential(ctx context.Context, credential *model.Credential) error
	UpdateCredential(ctx context.Context, credentialID []byte, accountID primitive.ObjectID, signCount uint32) error
	GetCredentials(ctx context.Context, accountID primitive.ObjectID) ([]model.Credential, error)
	GetCredentialByID(ctx context.Context, credentialID primitive.ObjectID) (*model.Credential, error)
	DeleteCredential(ctx context.Context, credentialID []byte, accountID primitive.ObjectID) error
	DeleteCredentialByID(ctx context.Context, credentialID primitive.ObjectID, accountID primitive.ObjectID) error
	DeleteCredentialsByAccountID(ctx context.Context, accountID primitive.ObjectID) error
	GetCredentialsCount(ctx context.Context, accountID primitive.ObjectID) (int, error)
}
