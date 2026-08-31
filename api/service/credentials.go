package service

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CredentialError is a custom error type for credential errors
type CredentialError struct {
	Code    string
	Message error
}

// Error implements Error interface
func (e *CredentialError) Error() string {
	return e.Message.Error()
}

// NewCredentialError creates a constructor function for each error
func NewCredentialError(msg string) *CredentialError {
	return &CredentialError{
		Code:    "CREDENTIAL_ERROR",
		Message: errors.New(msg),
	}
}

var (
	ErrGetCredentials        = NewCredentialError("Unable to retrieve credentials.")
	ErrSaveCredential        = NewCredentialError("Unable to save credential. Please try again.")
	ErrUpdateCredential      = NewCredentialError("Unable to update credential. Please try again.")
	ErrDeleteCredential      = NewCredentialError("Unable to delete credential. Please try again.")
	ErrMaxExceededCredential = NewCredentialError("You have reached the maximum number of allowed passkeys.")
)

// GetCredentials retrieves all credentials for an account
func (s *Service) GetCredentials(ctx context.Context, accountID primitive.ObjectID) ([]model.Credential, error) {
	credentials, err := s.Store.GetCredentials(ctx, accountID)
	if err != nil {
		return nil, ErrGetCredentials
	}

	return credentials, nil
}

func (s *Service) SaveCredential(ctx context.Context, credential webauthn.Credential, accountID primitive.ObjectID) error {
	count, err := s.Store.GetCredentialsCount(ctx, accountID)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("error saving credential")
		return ErrSaveCredential
	}

	if count >= s.Cfg.Service.MaxCredentials {
		return ErrMaxExceededCredential
	}

	err = s.Store.SaveCredential(ctx, credential, accountID)
	if err != nil {
		return ErrSaveCredential
	}

	return nil
}

func (s *Service) UpdateCredential(ctx context.Context, credential webauthn.Credential, accountID primitive.ObjectID) error {
	err := s.Store.UpdateCredential(ctx, credential, accountID)
	if err != nil {
		return ErrUpdateCredential
	}

	return nil
}

// func (s *Service) DeleteCredential(ctx context.Context, credential webauthn.Credential, userID string) error {
// 	err := s.Store.DeleteCredential(ctx, credential, userID)
// 	if err != nil {
// 		return ErrDeleteCredential
// 	}

// 	return nil
// }

// DeleteCredential deletes a credential by ID
func (s *Service) DeleteCredential(ctx context.Context, credentialID []byte, accountID primitive.ObjectID) error {
	err := s.Store.DeleteCredential(ctx, credentialID, accountID)
	if err != nil {
		return ErrDeleteCredential
	}

	return nil
}

func (s *Service) DeleteCredentialByID(ctx context.Context, credentialID primitive.ObjectID, accountID primitive.ObjectID) error {
	err := s.Store.DeleteCredentialByID(ctx, credentialID, accountID)
	if err != nil {
		return ErrDeleteCredential
	}

	return nil
}
