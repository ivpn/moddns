package service

import (
	"context"
	"errors"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/internal/auth"
	"github.com/ivpn/dns/api/internal/reauth"
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

func (s *Service) SaveSession(ctx context.Context, session model.SessionPayload, token string, accID string) error {
	err := s.Store.SaveSession(ctx, session, token, accID)
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

// DeleteOtherSessions signs out every session but the current one once the
// account owner is verified with the current password (plus a 2FA code when
// TOTP is on) or a passkey reauth token of purpose sessions_delete.
func (s *Service) DeleteOtherSessions(ctx context.Context, accID, currentToken string, req requests.SessionsDeleteRequest, mfa *model.MfaData) error {
	if _, err := reauth.Verify(ctx, s.Store, s.AccountServicer, reauth.Params{
		AccountId:        accID,
		TokenType:        auth.TokenTypeReauthSessionsDelete,
		Password:         req.CurrentPassword,
		ReauthToken:      req.ReauthToken,
		Mfa:              mfa,
		PersistOnConsume: true,
	}); err != nil {
		return err
	}
	return s.DeleteSessionsByAccountIDExceptCurrent(ctx, accID, currentToken)
}

func (s *Service) DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error {
	err := s.Store.DeleteSessionsByAccountIDExceptCurrent(ctx, accID, currentToken)
	if err != nil {
		return ErrDeleteSession
	}

	return nil
}
