package repository

import (
	"context"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/ivpn/dns/api/model"
)

// SessionRepository represents a Session repository
type SessionRepository interface {
	GetSession(ctx context.Context, token string) (model.Session, bool, error)
	SaveSession(ctx context.Context, sessionData webauthn.SessionData, token string, userID string, purpose string, subID string) error
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionsByAccountID(ctx context.Context, accID string) error
	DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error
	CountSessionsByAccountID(ctx context.Context, accID string) (int64, error)
}
