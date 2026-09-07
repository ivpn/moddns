package mongodb

import (
	"context"
	"time"

	"github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/internal/validator"
	"github.com/ivpn/dns/api/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AccountRepository is a MongoDB repository for accounts collection
type AccountRepository struct {
	DbName             string
	CollectionName     string
	accountsCollection *mongo.Collection
}

// NewAccountRepository creates a new AccountRepository instance
func NewAccountRepository(client *mongo.Client, dbName, collectionName string) AccountRepository {
	collection := client.Database(dbName).Collection(collectionName)

	return AccountRepository{
		DbName:             dbName,
		CollectionName:     collectionName,
		accountsCollection: collection,
	}
}

// Create adds a new account to the accounts collection
func (r *AccountRepository) CreateAccount(ctx context.Context, email, passwordPlain, accountId, profileId string) (*model.Account, error) {
	acc, err := model.NewAccount(validator.NormalizeEmail(email), passwordPlain, accountId, profileId)
	if err != nil {
		return nil, err
	}

	_, err = r.accountsCollection.InsertOne(ctx, acc)
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Msgf("Created new account")
	return acc, nil
}

// Update updates account
func (r *AccountRepository) UpdateAccount(ctx context.Context, account *model.Account) (*model.Account, error) {
	pByte, err := bson.Marshal(account)
	if err != nil {
		return nil, err
	}
	var update bson.M
	err = bson.Unmarshal(pByte, &update)
	if err != nil {
		return nil, err
	}

	updateQuery := bson.D{{Key: "$set", Value: update}}

	res, err := r.accountsCollection.UpdateByID(ctx, account.ID, updateQuery)
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Int64("modified_count", res.ModifiedCount).Msgf("Account updated")
	return account, nil
}

func (r *AccountRepository) GetAccountById(ctx context.Context, accountId string) (*model.Account, error) {
	accId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return nil, err
	}

	filterBson := bson.D{primitive.E{Key: "_id", Value: accId}}

	var account model.Account
	if err := r.accountsCollection.FindOne(ctx, filterBson).Decode(&account); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil

}

// GetAccountByEmail gets account data based on given email address.
// The lookup key is normalized to the canonical stored form, so any
// casing/whitespace variant of the address matches.
func (r *AccountRepository) GetAccountByEmail(ctx context.Context, email string) (*model.Account, error) {
	// Unique index "email" created by migration 013.
	filterBson := bson.D{primitive.E{Key: "email", Value: validator.NormalizeEmail(email)}}

	var account model.Account
	if err := r.accountsCollection.FindOne(ctx, filterBson).Decode(&account); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil
}

// GetAccountByToken gets account data based on given token and its type
func (r *AccountRepository) GetAccountByToken(ctx context.Context, token, tokenType string) (*model.Account, error) {
	filterBson := bson.D{
		primitive.E{Key: "tokens.value", Value: token},
		primitive.E{Key: "tokens.type", Value: tokenType},
	}

	var account model.Account
	if err := r.accountsCollection.FindOne(ctx, filterBson).Decode(&account); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil
}

// DeleteAccountById deletes account data by ID
func (r *AccountRepository) DeleteAccountById(ctx context.Context, accountId string) error {
	accId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}

	filterBson := bson.D{primitive.E{Key: "_id", Value: accId}}

	res, err := r.accountsCollection.DeleteOne(ctx, filterBson)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Int64("deleted_count", res.DeletedCount).Msgf("Account deleted")
	return nil
}

// UpdateDeletionCode updates the deletion code and expiration time for an account
// TODO: this should be handled via tokens array
func (r *AccountRepository) UpdateDeletionCode(ctx context.Context, accountId string, code string, expiresAt time.Time) error {
	accId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}

	filterBson := bson.D{primitive.E{Key: "_id", Value: accId}}
	updateBson := bson.D{
		primitive.E{Key: "$set", Value: bson.D{
			primitive.E{Key: "deletion_code", Value: code},
			primitive.E{Key: "deletion_code_expires", Value: expiresAt},
		}},
	}

	res, err := r.accountsCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Int64("modified_count", res.ModifiedCount).Msgf("Updated deletion code")
	return nil
}

// GetAccount retrieves an account by ID (alias for GetAccountById for compatibility)
func (r *AccountRepository) GetAccount(ctx context.Context, accountId string) (*model.Account, error) {
	return r.GetAccountById(ctx, accountId)
}

// AddProfileToAccount adds a profile ID to the account's profiles array using MongoDB $addToSet operator
func (r *AccountRepository) AddProfileToAccount(ctx context.Context, accountId string, profileId string) error {
	accId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}

	filterBson := bson.D{primitive.E{Key: "_id", Value: accId}}
	updateBson := bson.D{{Key: "$addToSet", Value: bson.D{
		primitive.E{Key: "profiles", Value: profileId},
	}}}

	res, err := r.accountsCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Int64("modified_count", res.ModifiedCount).Msgf("Added profile to account")
	return nil
}

// RemoveProfileFromAccount removes a profile ID from the account's profiles array using MongoDB $pull operator
func (r *AccountRepository) RemoveProfileFromAccount(ctx context.Context, accountId string, profileId string) error {
	accId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return err
	}

	filterBson := bson.D{primitive.E{Key: "_id", Value: accId}}
	updateBson := bson.D{{Key: "$pull", Value: bson.D{
		primitive.E{Key: "profiles", Value: profileId},
	}}}

	res, err := r.accountsCollection.UpdateOne(ctx, filterBson, updateBson)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Int64("modified_count", res.ModifiedCount).Msgf("Removed profile from account")
	return nil
}
