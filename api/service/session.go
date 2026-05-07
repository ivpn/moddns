package service

import (
	"context"
	"errors"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/model"
)

var (
	ErrGetSession    = errors.New("could not get session by token")
	ErrSaveSession   = errors.New("could not save session")
	ErrDeleteSession = errors.New("could not delete sessions")
)

func (s *Service) GetSession(ctx context.Context, token string) (model.Session, bool, error) {
	session, exists, err := s.Store.GetSession(ctx, token)
	if err != nil {
		return model.Session{}, false, ErrGetSession
	}

	return session, exists, nil
}

func (s *Service) SaveSession(ctx context.Context, session webauthn.SessionData, token string, accID string, purpose string, subID string) error {
	err := s.Store.SaveSession(ctx, session, token, accID, purpose, subID)
	if err != nil {
		return ErrSaveSession
	}

	return nil
}

func (s *Service) DeleteSession(ctx context.Context, token string) error {
	err := s.Store.DeleteSession(ctx, token)
	if err != nil {
		return ErrDeleteSession
	}

	return nil
}

func (s *Service) CountSessionsByAccountID(ctx context.Context, accID string) (int64, error) {
	count, err := s.Store.CountSessionsByAccountID(ctx, accID)
	if err != nil {
		return 0, err
	}

	return count, nil
}
func (s *Service) DeleteSessionsByAccountID(ctx context.Context, accID string) error {
	err := s.Store.DeleteSessionsByAccountID(ctx, accID)
	if err != nil {
		return ErrDeleteSession
	}

	return nil
}

func (s *Service) DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error {
	err := s.Store.DeleteSessionsByAccountIDExceptCurrent(ctx, accID, currentToken)
	if err != nil {
		return ErrDeleteSession
	}

	return nil
}
