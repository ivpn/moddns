package mongodb

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/model"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	TTLIndexNameAscending = "last_modified_1"
	TTLIndexName          = "last_modified"
)

// StatisticsRepository is a MongoDB repository for statistics timeseries collections
type SessionRepository struct {
	DbName         string
	CollectionName string
	sessionsColl   *mongo.Collection
}

// NewSessionRepository creates a new SessionRepository instance
func NewSessionRepository(ctx context.Context, client *mongo.Client, dbName string, sessionTTL time.Duration, collectionName string) (SessionRepository, error) {
	repo := SessionRepository{
		DbName:         dbName,
		CollectionName: collectionName,
	}
	repo.sessionsColl = client.Database(repo.DbName).Collection(collectionName)

	// Get existing indexes
	cursor, err := repo.sessionsColl.Indexes().List(ctx)
	if err != nil {
		return SessionRepository{}, errors.Wrap(err, "failed to list indexes")
	}

	// Look for TTL index
	var indexFound bool
	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return SessionRepository{}, errors.Wrap(err, "failed to read indexes")
	}

	for _, index := range results {
		if name, ok := index["name"].(string); ok && name == TTLIndexNameAscending {
			expireAfterSeconds, ok := index["expireAfterSeconds"].(int32)
			if !ok {
				log.Warn().Msg("Failed to parse expireAfterSeconds")
				continue
			}

			if expireAfterSeconds != int32(sessionTTL.Seconds()) {
				indexFound = true
				log.Info().Msg("Found existing TTL index, dropping it to update TTL value")
				_, err := repo.sessionsColl.Indexes().DropOne(ctx, TTLIndexNameAscending)
				if err != nil {
					return SessionRepository{}, errors.Wrap(err, "failed to drop existing TTL index")
				}
				break
			} else {
				log.Info().Str("collection", collectionName).Msg("TTL index already exists with up to date expiration time")
				return repo, nil // Skip recreation
			}
		}
	}

	if indexFound {
		log.Info().Dur("ttl", sessionTTL).Msg("Creating new TTL index with updated expiration")
	} else {
		log.Info().Dur("ttl", sessionTTL).Msg("Creating TTL index for sessions")
	}

	// Create or recreate TTL index with the new expiration time
	_, err = repo.sessionsColl.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: TTLIndexName, Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(int32(sessionTTL.Seconds())),
		},
	)
	if err != nil {
		return SessionRepository{}, errors.Wrap(err, "failed to create session TTL index")
	}

	return repo, nil
}

// GetSession retrieves a session by token
func (r *SessionRepository) GetSession(ctx context.Context, token string) (model.Session, bool, error) {
	var session model.Session
	err := r.sessionsColl.FindOne(
		ctx,
		bson.M{"token": token},
	).Decode(&session)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return model.Session{}, false, nil
		}
		log.Error().Err(err).Msg("Failed to get session")
		return model.Session{}, false, errors.Wrap(err, "failed to get session")
	}

	err = session.UnmarshalSessionData()
	if err != nil {
		log.Error().Err(err).Msg("Failed to deserialize session data")
		return model.Session{}, false, errors.Wrap(err, "failed to deserialize session data")
	}

	return session, true, nil
}

// SaveSession saves a webauthn session
func (r *SessionRepository) SaveSession(ctx context.Context, sessionData webauthn.SessionData, token string, accID string, purpose string) error {
	// Serialize the webauthn session data to JSON
	dataBytes, err := json.Marshal(sessionData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize session data")
		return errors.Wrap(err, "failed to serialize session data")
	}

	data := model.Session{
		Token:        token,
		AccountID:    accID,
		Data:         dataBytes,
		Purpose:      purpose,
		LastModified: time.Now(),
	}

	// Upsert the session
	filter := bson.M{"token": token}
	update := bson.M{"$set": data}
	opts := options.Update().SetUpsert(true)

	_, err = r.sessionsColl.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save session")
		return errors.Wrap(err, "failed to save session")
	}

	return nil
}

// DeleteSession removes a session by token
func (r *SessionRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.sessionsColl.DeleteOne(ctx, bson.M{"token": token})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete session")
		return errors.Wrap(err, "failed to delete session")
	}
	return nil
}

// DeleteSessionsByAccountID removes all sessions for an account
func (r *SessionRepository) DeleteSessionsByAccountID(ctx context.Context, accID string) error {
	_, err := r.sessionsColl.DeleteMany(ctx, bson.M{"account_id": accID})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete sessions for account")
		return errors.Wrap(err, "failed to delete sessions for account")
	}
	return nil
}

// DeleteSessionsByAccountIDExceptCurrent removes all sessions for an account except the current one
func (r *SessionRepository) DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error {
	filter := bson.M{
		"account_id": accID,
		"token":      bson.M{"$ne": currentToken},
	}
	_, err := r.sessionsColl.DeleteMany(ctx, filter)
	if err != nil {
		return errors.Wrap(err, "failed to delete sessions for account except current")
	}
	return nil
}

// CountSessionsByAccountID counts the number of active sessions for an account
func (r *SessionRepository) CountSessionsByAccountID(ctx context.Context, accID string) (int64, error) {
	count, err := r.sessionsColl.CountDocuments(ctx, bson.M{"account_id": accID})
	if err != nil {
		log.Error().Err(err).Msg("Failed to count sessions for account")
		return 0, errors.Wrap(err, "failed to count sessions for account")
	}
	return count, nil
}
