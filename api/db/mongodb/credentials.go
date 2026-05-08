package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	CredentialsCollection      = "credentials"
	WebAuthnSessionsCollection = "webauthn_sessions"
)

// CredentialRepository handles WebAuthn credential operations
type CredentialRepository struct {
	DbName          string
	CollectionName  string
	credentialsColl *mongo.Collection
}

// NewCredentialRepository creates a new credential repository
func NewCredentialRepository(client *mongo.Client, dbName, collectionName string) CredentialRepository {
	return CredentialRepository{
		DbName:          dbName,
		CollectionName:  collectionName,
		credentialsColl: client.Database(dbName).Collection(collectionName),
	}

}

// SaveCredential saves a WebAuthn credential to the database
func (r *CredentialRepository) SaveCredential(ctx context.Context, credential webauthn.Credential, accountID primitive.ObjectID) error {
	cred := model.NewCredentialFromWebAuthn(credential, accountID)

	_, err := r.credentialsColl.InsertOne(ctx, cred)
	if err != nil {
		return fmt.Errorf("failed to save credential: %w", err)
	}

	return nil
}

// UpdateCredential updates an existing WebAuthn credential
func (r *CredentialRepository) UpdateCredential(ctx context.Context, credential webauthn.Credential, accountID primitive.ObjectID) error {

	filter := bson.M{
		"credential_id": credential.ID,
		"account_id":    accountID,
	}

	update := bson.M{
		"$set": bson.M{
			"authenticator.sign_count": credential.Authenticator.SignCount,
			"updated_at":               time.Now(),
		},
	}

	_, err := r.credentialsColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	return nil
}

// GetCredentials retrieves all credentials for an account
func (r *CredentialRepository) GetCredentials(ctx context.Context, accountID primitive.ObjectID) ([]model.Credential, error) {
	filter := bson.M{"account_id": accountID}
	cursor, err := r.credentialsColl.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find credentials: %w", err)
	}
	defer cursor.Close(ctx)

	var credentials []model.Credential
	if err := cursor.All(ctx, &credentials); err != nil {
		return nil, fmt.Errorf("failed to decode credentials: %w", err)
	}

	return credentials, nil
}

// GetCredentialsCount returns the number of credentials for an account
func (r *CredentialRepository) GetCredentialsCount(ctx context.Context, accountID primitive.ObjectID) (int, error) {
	filter := bson.M{"account_id": accountID}
	count, err := r.credentialsColl.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count credentials: %w", err)
	}

	return int(count), nil
}

// GetCredentialByID retrieves a credential by its ID
func (r *CredentialRepository) GetCredentialByID(ctx context.Context, credentialID primitive.ObjectID) (*model.Credential, error) {

	filter := bson.M{"_id": credentialID}
	var credential model.Credential
	err := r.credentialsColl.FindOne(ctx, filter).Decode(&credential)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("credential not found")
		}
		return nil, fmt.Errorf("failed to find credential: %w", err)
	}

	return &credential, nil
}

// DeleteCredential deletes a credential by credential ID and account ID
func (r *CredentialRepository) DeleteCredential(ctx context.Context, credentialID []byte, accountID primitive.ObjectID) error {
	filter := bson.M{
		"credential_id": credentialID,
		"account_id":    accountID,
	}

	_, err := r.credentialsColl.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
}

// DeleteCredentialByID deletes a credential by its MongoDB ID
func (r *CredentialRepository) DeleteCredentialByID(ctx context.Context, credentialID primitive.ObjectID, accountID primitive.ObjectID) error {
	filter := bson.M{
		"_id":        credentialID,
		"account_id": accountID,
	}

	_, err := r.credentialsColl.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
}

// DeleteCredentialsByAccountID deletes all credentials belonging to an account.
func (r *CredentialRepository) DeleteCredentialsByAccountID(ctx context.Context, accountID primitive.ObjectID) error {
	_, err := r.credentialsColl.DeleteMany(ctx, bson.M{"account_id": accountID})
	if err != nil {
		return fmt.Errorf("failed to delete credentials for account: %w", err)
	}

	return nil
}
